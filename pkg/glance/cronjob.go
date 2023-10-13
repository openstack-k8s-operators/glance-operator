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
	"strconv"
	"strings"
)

// CronJob func
func CronJob(
	instance *glancev1.Glance,
	labels map[string]string,
	annotations map[string]string,
	cjType CronJobType,
) *batchv1.CronJob {
	runAsUser := int64(0)
	var config0644AccessMode int32 = 0644
	var cronJobCommand []string
	var cronJobSchedule string

	switch cjType {
	case "purge":
		cronJobCommand = DBPurgeCommandBase[:]
		// Extend the resulting command with the DBPurgeAge int in case purge is
		cronJobCommand = append(cronJobCommand, strconv.Itoa(DBPurgeAge))
		cronJobSchedule = DBPurgeDefaultSchedule
	case "cleaner":
		cronJobCommand = CacheCleanerCommandBase[:]
		cronJobSchedule = CacheCleanerDefaultSchedule
	case "pruner":
		cronJobCommand = CachePrunerCommandBase[:]
		cronJobSchedule = CachePrunerDefaultSchedule
	default:
		cronJobCommand = DBPurgeCommandBase[:]
		cronJobSchedule = DBPurgeDefaultSchedule
	}

	var cronCommand []string = cronJobCommand[:]
	args := []string{"-c"}

	if !instance.Spec.Debug.CronJob {
		// If debug mode is not requested, remove the --debug option
		cronCommand = append(cronJobCommand[:1], cronJobCommand[2:]...)
	}
	// NOTE: (fpantano) - check if it makes sense extending this command to
	// purge_images_table
	// Build the resulting command
	commandString := strings.Join(cronCommand, " ")
	args = append(args, commandString)

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
	}
	cronJobVolumeMounts := []corev1.VolumeMount{
		{
			Name:      "db-purge-config-data",
			MountPath: "/etc/glance/glance.conf.d",
			ReadOnly:  true,
		},
	}
	// If cache is provided, we expect the main glance_controller to request a
	// PVC that should be used for that purpose (according to ImageCacheSize)
	// and it should be available to the Cache CronJobs to properly clean the
	// image-cache path
	if cjType == "pruner" || cjType == "cleaner" {
		cronJobVolume = append(cronJobVolume, GetCacheVolume(ServiceName+"-cache")...)
		cronJobVolumeMounts = append(cronJobVolumeMounts, GetCacheVolumeMount()...)
	}

	cronjob := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s-cron", ServiceName, cjType),
			Namespace: instance.Namespace,
		},
		Spec: batchv1.CronJobSpec{
			Schedule:          cronJobSchedule,
			ConcurrencyPolicy: batchv1.ForbidConcurrent,
			JobTemplate: batchv1.JobTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: annotations,
					Labels:      labels,
				},
				Spec: batchv1.JobSpec{
					Parallelism: &parallelism,
					Completions: &completions,
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  ServiceName + "-cron",
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
	if instance.Spec.NodeSelector != nil && len(instance.Spec.NodeSelector) > 0 {
		cronjob.Spec.JobTemplate.Spec.Template.Spec.NodeSelector = instance.Spec.NodeSelector
	}
	return cronjob
}
