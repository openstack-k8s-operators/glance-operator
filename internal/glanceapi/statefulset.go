/*

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package glanceapi

import (
	"fmt"

	glancev1 "github.com/openstack-k8s-operators/glance-operator/api/v1beta1"
	glance "github.com/openstack-k8s-operators/glance-operator/internal/glance"
	memcachedv1 "github.com/openstack-k8s-operators/infra-operator/apis/memcached/v1beta1"
	topologyv1 "github.com/openstack-k8s-operators/infra-operator/apis/topology/v1beta1"
	common "github.com/openstack-k8s-operators/lib-common/modules/common"
	"github.com/openstack-k8s-operators/lib-common/modules/common/affinity"
	"github.com/openstack-k8s-operators/lib-common/modules/common/env"
	"github.com/openstack-k8s-operators/lib-common/modules/common/pod"
	"github.com/openstack-k8s-operators/lib-common/modules/common/probes"
	"github.com/openstack-k8s-operators/lib-common/modules/common/service"
	"github.com/openstack-k8s-operators/lib-common/modules/common/tls"
	"github.com/openstack-k8s-operators/lib-common/modules/common/volume"
	"github.com/openstack-k8s-operators/lib-common/modules/storage"
	"github.com/openstack-k8s-operators/lib-common/modules/users"

	"sort"

	"golang.org/x/exp/maps"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

// workerSelfReferenceScript replicates kolla_extend_start's behavior: when
// GLANCE_DOMAIN is set (distributed image import), each replica must write
// its own runtime-only worker_self_reference_url into glance.conf.d -- the
// pod's own hostname isn't known until the container is actually running, so
// this can't be pre-rendered into a static Secret.
const workerSelfReferenceScript = `if [ -n "$GLANCE_DOMAIN" ]; then cat > /etc/glance/glance.conf.d/01-config.conf <<EOF
[DEFAULT]
worker_self_reference_url=${URISCHEME,,}://$(hostname).${GLANCE_DOMAIN}:${GLANCE_PORT}
EOF
fi
`

// privilegedAwareSecurityContext returns the SecurityContext for the httpd
// and glance-api containers. When Cinder is configured as a backend, host
// device access via nsenter'd multipath/iscsi tooling requires Privileged,
// which can't be combined with RunAsNonRoot/ReadOnlyRootFilesystem; otherwise
// use the fully restrictive context.
func privilegedAwareSecurityContext(privileged bool) *corev1.SecurityContext {
	if privileged {
		return pod.PrivilegedSecurityContext(users.GlanceUID, users.GlanceGID)
	}
	return pod.RestrictiveSecurityContext(users.GlanceUID, users.GlanceGID)
}

// StatefulSet func
func StatefulSet(
	instance *glancev1.GlanceAPI,
	configHash string,
	labels map[string]string,
	annotations map[string]string,
	privileged bool,
	topology *topologyv1.Topology,
	wsgi bool,
	memcached *memcachedv1.Memcached,
) (*appsv1.StatefulSet, error) {
	//
	// https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/
	//
	port := int32(glance.GlancePublicPort)
	glanceURIScheme := corev1.URISchemeHTTP
	tlsEnabled := instance.Spec.TLS.API.Enabled(service.EndpointPublic)

	if instance.Spec.APIType == glancev1.APIInternal ||
		instance.Spec.APIType == glancev1.APIEdge {
		port = int32(glance.GlanceInternalPort)
		tlsEnabled = instance.Spec.TLS.API.Enabled(service.EndpointInternal)
	}

	if tlsEnabled {
		glanceURIScheme = corev1.URISchemeHTTPS
	}

	// Create ProbeSet
	probes, err := probes.CreateProbeSet(
		int32(port),
		&glanceURIScheme,
		instance.Spec.Override.Probes,
		glance.GetDefaultProbesAPI(instance.Spec.APITimeout),
	)
	// Could not process probes config
	if err != nil {
		return nil, err
	}

	// envVars
	envVars := map[string]env.Setter{}
	envVars["CONFIG_HASH"] = env.SetValue(configHash)
	envVars["GLANCE_DOMAIN"] = env.SetValue(instance.Status.Domain)
	envVars["URISCHEME"] = env.SetValue(string(glanceURIScheme))
	envVars["GLANCE_PORT"] = env.SetValue(fmt.Sprintf("%d", port))

	// basic volume/volumeMounts
	apiVolumes := glance.GetAPIVolumes()
	apiVolumeMounts := glance.GetAPIVolumeMount(instance.Spec.ImageCache.Size)
	extraVolPropagation := append(glance.GlanceAPIPropagation,
		storage.PropagationType(instance.APIName()))
	// Add the CA bundle to the apiVolumes and httpdVolumeMount
	if instance.Spec.TLS.CaBundleSecretName != "" {
		apiVolumes = append(apiVolumes, instance.Spec.TLS.CreateVolume())
		apiVolumeMounts = append(apiVolumeMounts, instance.Spec.TLS.CreateVolumeMounts(nil)...)
	}

	// add MTLS cert if defined
	if memcached.GetMemcachedMTLSSecret() != "" {
		apiVolumes = append(apiVolumes, memcached.CreateMTLSVolume())
		certMountPath := memcachedv1.CertPathDst
		keyMountPath := memcachedv1.KeyPathDst
		apiVolumeMounts = append(apiVolumeMounts, memcached.CreateMTLSVolumeMounts(&certMountPath, &keyMountPath)...)
	}

	// TLS-e: we need to predict the order of both Volumes and VolumeMounts to
	// prevent any unwanted Pod restart and StatefulSet rollout due to an
	// update on its revision, so we sort the endpoints to make sure we preserve
	// the append order.
	endpts := maps.Keys(GetGlanceEndpoints(instance.Spec.APIType))
	sort.Slice(endpts, func(i, j int) bool {
		return string(endpts[i]) < string(endpts[j])
	})
	for _, endpt := range endpts {
		if instance.Spec.TLS.API.Enabled(endpt) {
			var tlsEndptCfg tls.GenericService
			switch endpt {
			case service.EndpointPublic:
				tlsEndptCfg = instance.Spec.TLS.API.Public
			case service.EndpointInternal:
				tlsEndptCfg = instance.Spec.TLS.API.Internal
			}

			svc, err := tlsEndptCfg.ToService()
			if err != nil {
				return nil, err
			}
			// httpd container is not using kolla, mount the certs to its dst
			svc.CertMount = ptr.To(fmt.Sprintf("/etc/pki/tls/certs/%s.crt", endpt.String()))
			svc.KeyMount = ptr.To(fmt.Sprintf("/etc/pki/tls/private/%s.key", endpt.String()))

			apiVolumes = append(apiVolumes, svc.CreateVolume(endpt.String()))
			apiVolumeMounts = append(apiVolumeMounts, svc.CreateVolumeMounts(endpt.String())...)
		}
	}

	stsName := instance.Name
	// The StatefulSet name **must** match with the headless service
	// endpoint Name (see GetHeadlessService() function under controllers/
	// glance_common)
	if instance.Spec.APIType != glancev1.APISingle {
		stsName = fmt.Sprintf("%s-api", instance.Name)
	}

	LogFile := string(glance.GlanceLogPath + instance.Name + ".log")
	statefulset := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      stsName,
			Namespace: instance.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.StatefulSetSpec{
			ServiceName: stsName,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			PodManagementPolicy: appsv1.ParallelPodManagement,
			Replicas:            instance.Spec.Replicas,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: annotations,
					Labels:      labels,
				},
				Spec: corev1.PodSpec{
					SecurityContext:              pod.RestrictivePodSecurityContext(users.GlanceUID, users.GlanceGID),
					ServiceAccountName:           instance.Spec.ServiceAccount,
					AutomountServiceAccountToken: ptr.To(false),
					// When using Cinder we run as privileged, but also some
					// commands need to be run on the host using nsenter (eg:
					// iscsi commands) so we need to share the PID namespace
					// with the host.
					HostPID: privileged,
					Containers: []corev1.Container{
						{
							Name: glance.ServiceName + "-log",
							Command: []string{
								"/usr/bin/dumb-init",
							},
							Args: []string{
								"--single-child",
								"--",
								"/bin/sh",
								"-c",
								"/usr/bin/tail -n+1 -F " + LogFile + " 2>/dev/null",
							},
							Image:           instance.Spec.ContainerImage,
							SecurityContext: pod.RestrictiveSecurityContext(users.GlanceUID, users.GlanceGID),
							Env:             env.MergeEnvs([]corev1.EnvVar{}, envVars),
							VolumeMounts:    []corev1.VolumeMount{volume.WritableDirVolumeMount(glance.LogVolume, "/var/log/glance")},
							Resources:       instance.Spec.Resources,
						},
						{
							Name: glance.ServiceName + "-httpd",
							Command: []string{
								"/usr/bin/dumb-init",
							},
							Args: []string{
								"--single-child",
								"--",
								"/bin/bash",
								"-c",
								workerSelfReferenceScript + "exec /usr/sbin/httpd -DFOREGROUND",
							},
							Image:           instance.Spec.ContainerImage,
							SecurityContext: privilegedAwareSecurityContext(privileged),
							Env:             env.MergeEnvs([]corev1.EnvVar{}, envVars),
							VolumeMounts: append(glance.GetVolumeMounts(
								instance.Spec.CustomServiceConfigSecrets,
								privileged,
								instance.Spec.Storage.External,
								instance.Spec.ExtraMounts,
								extraVolPropagation,
								"httpd",
								wsgi,
							),
								apiVolumeMounts...,
							),
							Resources:      instance.Spec.Resources,
							ReadinessProbe: probes.Readiness,
							LivenessProbe:  probes.Liveness,
						},
					},
				},
			},
		},
	}
	// When wsgi is false, Glance must be deployed in legacy mode (httpd + proxyPass)
	// For this reason we need an additional container to run glance-api processes
	if !wsgi {
		apiContainer := []corev1.Container{
			{
				Name: glance.ServiceName + "-api",
				Command: []string{
					"/usr/bin/dumb-init",
				},
				Args: []string{
					"--single-child",
					"--",
					"/bin/bash",
					"-c",
					workerSelfReferenceScript + "exec /usr/bin/glance-api --config-dir /etc/glance/glance.conf.d",
				},
				Image:           instance.Spec.ContainerImage,
				SecurityContext: privilegedAwareSecurityContext(privileged),
				Env:             env.MergeEnvs([]corev1.EnvVar{}, envVars),
				VolumeMounts: append(glance.GetVolumeMounts(
					instance.Spec.CustomServiceConfigSecrets,
					privileged,
					instance.Spec.Storage.External,
					instance.Spec.ExtraMounts,
					extraVolPropagation,
					"api",
					wsgi,
				),
					apiVolumeMounts...,
				),
				Resources:      instance.Spec.Resources,
				ReadinessProbe: probes.Readiness,
				LivenessProbe:  probes.Liveness,
			},
		}
		statefulset.Spec.Template.Spec.Containers = append(statefulset.Spec.Template.Spec.Containers, apiContainer...)
	}

	if !instance.Spec.Storage.External {
		localPvc, err := glance.GetPvc(instance, labels, glance.PvcLocal)
		if err != nil {
			return statefulset, err
		}
		statefulset.Spec.VolumeClaimTemplates = []corev1.PersistentVolumeClaim{localPvc}
	}
	// Staging and Cache are realized through separate interfaces
	// (TODO) Allow to externally manage image-cache
	if len(instance.Spec.ImageCache.Size) > 0 {
		cachePvc, err := glance.GetPvc(instance, labels, glance.PvcCache)
		if err != nil {
			return statefulset, err
		}
		statefulset.Spec.VolumeClaimTemplates = append(statefulset.Spec.VolumeClaimTemplates, cachePvc)
	}

	statefulset.Spec.Template.Spec.Volumes = append(glance.GetVolumes(
		instance.Name,
		privileged,
		instance.Spec.CustomServiceConfigSecrets,
		instance.Spec.ExtraMounts,
		extraVolPropagation),
		apiVolumes...)

	if instance.Spec.NodeSelector != nil {
		statefulset.Spec.Template.Spec.NodeSelector = *instance.Spec.NodeSelector
	}

	if topology != nil {
		topology.ApplyTo(&statefulset.Spec.Template)
	} else {
		// If possible two pods of the same service should not
		// run on the same worker node. If this is not possible
		// the get still created on the same worker node.
		statefulset.Spec.Template.Spec.Affinity = affinity.DistributePods(
			common.AppSelector,
			[]string{
				glance.ServiceName,
			},
			corev1.LabelHostname,
		)
	}

	return statefulset, err
}
