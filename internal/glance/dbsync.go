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

package glance

import (
	"maps"
	"slices"

	glancev1 "github.com/openstack-k8s-operators/glance-operator/api/v1beta1"
	"github.com/openstack-k8s-operators/lib-common/modules/common/env"
	"github.com/openstack-k8s-operators/lib-common/modules/common/pod"
	"github.com/openstack-k8s-operators/lib-common/modules/users"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

// DBSyncCommand - direct glance-manage command without kolla wrapper
const DBSyncCommand = "glance-manage --config-dir /etc/glance/glance.conf.d db sync glance && " +
	"glance-manage --config-dir /etc/glance/glance.conf.d db_load_metadefs"

// DbSyncJob func
func DbSyncJob(
	instance *glancev1.Glance,
	labels map[string]string,
	annotations map[string]string,
) *batchv1.Job {
	// Unlike the individual glanceAPI services, the DbSyncJob doesn't need a
	// secret that contains all of the config snippets required by every
	// service, The two snippet files that it does need (DefaultsConfigFileName
	// and CustomConfigFileName) can be extracted from the top-level glance
	// config-data secret.
	dbSyncVolume := []corev1.Volume{
		{
			Name: "db-sync-config-data",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					DefaultMode: &configMode,
					SecretName:  instance.Name + "-config-data",
					Items: []corev1.KeyToPath{
						{
							Key:  DefaultsConfigFileName,
							Path: DefaultsConfigFileName,
						},
						{
							Key:  CustomConfigFileName,
							Path: CustomConfigFileName,
						},
					},
				},
			},
		},
		{
			Name: "config-data",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					DefaultMode: &configMode,
					SecretName:  instance.Name + "-config-data",
				},
			},
		},
	}

	dbSyncMounts := []corev1.VolumeMount{
		{
			Name:      "db-sync-config-data",
			MountPath: "/etc/glance/glance.conf.d",
			ReadOnly:  true,
		},
		{
			Name:      "config-data",
			MountPath: "/etc/my.cnf",
			SubPath:   "my.cnf",
			ReadOnly:  true,
		},
	}

	// add CA cert if defined from the first api (sorted for deterministic selection)
	for _, name := range slices.Sorted(maps.Keys(instance.Spec.GlanceAPIs)) {
		api := instance.Spec.GlanceAPIs[name]
		if api.TLS.CaBundleSecretName != "" {
			dbSyncVolume = append(dbSyncVolume, api.TLS.CreateVolume())
			dbSyncMounts = append(dbSyncMounts, api.TLS.CreateVolumeMounts(nil)...)

			break
		}
	}

	args := []string{"-c", DBSyncCommand}
	envVars := map[string]env.Setter{}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ServiceName + "-db-sync",
			Namespace: instance.Namespace,
			Labels:    labels,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: annotations,
				},
				Spec: corev1.PodSpec{
					RestartPolicy:                corev1.RestartPolicyOnFailure,
					ServiceAccountName:           instance.RbacResourceName(),
					AutomountServiceAccountToken: ptr.To(false),
					SecurityContext:              pod.RestrictivePodSecurityContext(users.GlanceUID, users.GlanceGID),
					Containers: []corev1.Container{
						{
							Name: ServiceName + "-db-sync",
							Command: []string{
								"/bin/bash",
							},
							Args:            args,
							Image:           instance.Spec.ContainerImage,
							SecurityContext: pod.RestrictiveSecurityContext(users.GlanceUID, users.GlanceGID),
							Env:             env.MergeEnvs([]corev1.EnvVar{}, envVars),
							VolumeMounts:    dbSyncMounts,
						},
					},
				},
			},
		},
	}
	if instance.Spec.NodeSelector != nil {
		job.Spec.Template.Spec.NodeSelector = *instance.Spec.NodeSelector
	}
	job.Spec.Template.Spec.Volumes = dbSyncVolume
	return job
}
