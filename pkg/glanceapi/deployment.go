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
	glance "github.com/openstack-k8s-operators/glance-operator/pkg/glance"
	common "github.com/openstack-k8s-operators/lib-common/modules/common"
	"github.com/openstack-k8s-operators/lib-common/modules/common/affinity"
	"github.com/openstack-k8s-operators/lib-common/modules/common/env"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	// ServiceCommand -
	ServiceCommand = "/usr/local/bin/kolla_set_configs && /usr/local/bin/kolla_start"
)

// Deployment func
func Deployment(
	instance *glancev1.GlanceAPI,
	configHash string,
	labels map[string]string,
) *appsv1.Deployment {
	runAsUser := int64(0)

	livenessProbe := &corev1.Probe{
		// TODO might need tuning
		TimeoutSeconds:      5,
		PeriodSeconds:       3,
		InitialDelaySeconds: 3,
	}
	readinessProbe := &corev1.Probe{
		// TODO might need tuning
		TimeoutSeconds:      5,
		PeriodSeconds:       5,
		InitialDelaySeconds: 5,
	}

	args := []string{"-c"}
	if instance.Spec.Debug.Service {
		args = append(args, common.DebugCommand)
		livenessProbe.Exec = &corev1.ExecAction{
			Command: []string{
				"/bin/true",
			},
		}

		readinessProbe.Exec = &corev1.ExecAction{
			Command: []string{
				"/bin/true",
			},
		}
	} else {
		args = append(args, ServiceCommand)
		//
		// https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/
		//

		port := int32(glance.GlancePublicPort)

		if instance.Spec.APIType == glancev1.APIInternal {
			port = int32(glance.GlanceInternalPort)
		}

		livenessProbe.HTTPGet = &corev1.HTTPGetAction{
			Path: "/healthcheck",
			Port: intstr.IntOrString{Type: intstr.Int, IntVal: port},
		}
		readinessProbe.HTTPGet = &corev1.HTTPGetAction{
			Path: "/healthcheck",
			Port: intstr.IntOrString{Type: intstr.Int, IntVal: port},
		}
	}

	envVars := map[string]env.Setter{}
	envVars["KOLLA_CONFIG_FILE"] = env.SetValue(glance.KollaConfig)
	envVars["KOLLA_CONFIG_STRATEGY"] = env.SetValue("COPY_ALWAYS")
	envVars["CONFIG_HASH"] = env.SetValue(configHash)

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-api", instance.Name),
			Namespace: instance.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Replicas: &instance.Spec.Replicas,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: glance.ServiceAccount,
					Containers: []corev1.Container{
						{
							Name: glance.ServiceName + "-api",
							Command: []string{
								"/bin/bash",
							},
							Args:  args,
							Image: instance.Spec.ContainerImage,
							SecurityContext: &corev1.SecurityContext{
								RunAsUser: &runAsUser,
							},
							Env:            env.MergeEnvs([]corev1.EnvVar{}, envVars),
							VolumeMounts:   glance.GetVolumeMounts(instance.Spec.ExtraMounts, glance.GlanceAPIPropagation),
							Resources:      instance.Spec.Resources,
							ReadinessProbe: readinessProbe,
							LivenessProbe:  livenessProbe,
						},
					},
				},
			},
		},
	}
	deployment.Spec.Template.Spec.Volumes = glance.GetVolumes(instance.Name, glance.ServiceName, instance.Spec.ExtraMounts, glance.GlanceAPIPropagation)
	// If possible two pods of the same service should not
	// run on the same worker node. If this is not possible
	// the get still created on the same worker node.
	deployment.Spec.Template.Spec.Affinity = affinity.DistributePods(
		common.AppSelector,
		[]string{
			glance.ServiceName,
		},
		corev1.LabelHostname,
	)
	if instance.Spec.NodeSelector != nil && len(instance.Spec.NodeSelector) > 0 {
		deployment.Spec.Template.Spec.NodeSelector = instance.Spec.NodeSelector
	}

	initContainerDetails := glance.APIDetails{
		ContainerImage:       instance.Spec.ContainerImage,
		DatabaseHost:         instance.Spec.DatabaseHostname,
		DatabaseUser:         instance.Spec.DatabaseUser,
		DatabaseName:         glance.DatabaseName,
		OSPSecret:            instance.Spec.Secret,
		DBPasswordSelector:   instance.Spec.PasswordSelectors.Database,
		UserPasswordSelector: instance.Spec.PasswordSelectors.Service,
		VolumeMounts:         getInitVolumeMounts(instance.Spec.ExtraMounts, glance.GlanceAPIPropagation),
	}
	deployment.Spec.Template.Spec.InitContainers = glance.InitContainer(initContainerDetails)

	return deployment
}
