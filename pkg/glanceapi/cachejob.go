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
	"github.com/openstack-k8s-operators/glance-operator/pkg/glance"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

// ImageCacheJob -
func ImageCacheJob(
	instance *glancev1.GlanceAPI,
	cronSpec glance.CronJobSpec,
) *batchv1.CronJob {
	var config0644AccessMode int32 = 0644

	cronCommand := fmt.Sprintf(
		"%s --config-dir /etc/glance/glance.conf.d",
		cronSpec.Command,
	)

	args := []string{"-c", cronCommand}

	parallelism := int32(1)
	completions := int32(1)

	cronJobVolume := []corev1.Volume{
		{
			Name: "image-cache-config-data",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					DefaultMode: &config0644AccessMode,
					SecretName:  instance.Name + "-config-data",
					Items: []corev1.KeyToPath{
						{
							Key:  glance.DefaultsConfigFileName,
							Path: glance.DefaultsConfigFileName,
						},
					},
				},
			},
		},
	}
	cronJobVolumeMounts := []corev1.VolumeMount{
		{
			Name:      "image-cache-config-data",
			MountPath: "/etc/glance/glance.conf.d",
			ReadOnly:  true,
		},
	}

	// add CA cert if defined from the first api
	if instance.Spec.GlanceAPITemplate.TLS.CaBundleSecretName != "" {
		cronJobVolume = append(cronJobVolume, instance.Spec.GlanceAPITemplate.TLS.CreateVolume())
		cronJobVolumeMounts = append(cronJobVolumeMounts, instance.Spec.GlanceAPITemplate.TLS.CreateVolumeMounts(nil)...)
	}

	// The image-cache PVC should be available to the Cache CronJobs to properly
	// clean the image-cache path
	if cronSpec.CjType == glance.CachePruner || cronSpec.CjType == glance.CacheCleaner {
		cronJobVolume = append(cronJobVolume, glance.GetCacheVolume(*cronSpec.PvcClaim)...)
		cronJobVolumeMounts = append(cronJobVolumeMounts, glance.GetCacheVolumeMount()...)
	}

	cronjob := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cronSpec.Name,
			Namespace: instance.Namespace,
		},
		Spec: batchv1.CronJobSpec{
			Schedule:          cronSpec.Schedule,
			ConcurrencyPolicy: batchv1.ForbidConcurrent,
			JobTemplate: batchv1.JobTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: cronSpec.Annotations,
					Labels:      cronSpec.Labels,
				},
				Spec: batchv1.JobSpec{
					Parallelism: &parallelism,
					Completions: &completions,
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							SecurityContext: &corev1.PodSecurityContext{
								FSGroup: ptr.To(glance.GlanceUID),
							},
							Affinity: GetGlanceAPIPodAffinity(instance),
							Containers: []corev1.Container{
								{
									Name:  cronSpec.Name,
									Image: instance.Spec.ContainerImage,
									Command: []string{
										"/bin/bash",
									},
									Args:            args,
									VolumeMounts:    cronJobVolumeMounts,
									SecurityContext: glance.BaseSecurityContext(),
								},
							},
							Volumes:            cronJobVolume,
							RestartPolicy:      corev1.RestartPolicyNever,
							ServiceAccountName: instance.Spec.ServiceAccount,
						},
					},
				},
			},
		},
	}
	// We need to schedule the cronJob to the same Node where a given glanceAPI
	// is schedule: this allow to mount the same RWO volume
	if instance.Spec.NodeSelector != nil && len(instance.Spec.NodeSelector) > 0 {
		cronjob.Spec.JobTemplate.Spec.Template.Spec.NodeSelector = instance.Spec.NodeSelector
	}
	return cronjob
}
