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
	glancev1 "github.com/openstack-k8s-operators/glance-operator/api/v1beta1"

	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CronJobSpec -
type CronJobSpec struct {
	Name        string
	PvcClaim    *string
	Schedule    string
	Command     string
	CjType      CronJobType
	Labels      map[string]string
	Annotations map[string]string
}

// DBPurgeJob -
func DBPurgeJob(
	instance *glancev1.Glance,
	cronSpec CronJobSpec,
) *batchv1.CronJob {
	runAsUser := int64(0)
	var config0644AccessMode int32 = 0644

	cronCommand := fmt.Sprintf(
		"%s --config-dir /etc/glance/glance.conf.d db purge %d",
		cronSpec.Command,
		instance.Spec.DBPurge.Age,
	)

	args := []string{"-c", cronCommand}

	parallelism := int32(1)
	completions := int32(1)

	cronJobVolume := []corev1.Volume{
		{
			Name: "db-purge-config-data",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					DefaultMode: &config0644AccessMode,
					SecretName:  instance.Name + "-config-data",
					Items: []corev1.KeyToPath{
						{
							Key:  DefaultsConfigFileName,
							Path: DefaultsConfigFileName,
						},
					},
				},
			},
		},
		{
			Name: "config-data",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					DefaultMode: &config0644AccessMode,
					SecretName:  ServiceName + "-config-data",
				},
			},
		},
	}
	cronJobVolumeMounts := []corev1.VolumeMount{
		{
			Name:      "db-purge-config-data",
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

	// add CA cert if defined from the first api
	for _, api := range instance.Spec.GlanceAPIs {
		if api.TLS.CaBundleSecretName != "" {
			cronJobVolume = append(cronJobVolume, api.TLS.CreateVolume())
			cronJobVolumeMounts = append(cronJobVolumeMounts, api.TLS.CreateVolumeMounts(nil)...)

			break
		}
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
							Containers: []corev1.Container{
								{
									Name:  ServiceName + "-dbpurge",
									Image: instance.Spec.ContainerImage,
									Command: []string{
										"/bin/bash",
									},
									Args:         args,
									VolumeMounts: cronJobVolumeMounts,
									SecurityContext: &corev1.SecurityContext{
										RunAsUser: &runAsUser,
									},
								},
							},
							Volumes:            cronJobVolume,
							RestartPolicy:      corev1.RestartPolicyNever,
							ServiceAccountName: instance.RbacResourceName(),
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
