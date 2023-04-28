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
	"github.com/openstack-k8s-operators/lib-common/modules/storage"
	corev1 "k8s.io/api/core/v1"
)

// GetVolumes - service volumes
func GetVolumes(
	name string,
	pvcName string,
	extraVol []glancev1.GlanceExtraVolMounts,
	svc []storage.PropagationType,
	confSecret []string,
) []corev1.Volume {
	var config0640AccessMode int32 = 0640

	vm := []corev1.Volume{
		{
			Name: "config-data",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					DefaultMode: &config0640AccessMode,
					LocalObjectReference: corev1.LocalObjectReference{
						Name: name + "-config-data",
					},
				},
			},
		},
		{
			Name: "lib-data",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: pvcName,
				},
			},
		},
		{
			Name: "config-data-merged",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{Medium: ""},
			},
		},
	}

	for _, exv := range extraVol {
		for _, vol := range exv.Propagate(svc) {
			vm = append(vm, vol.Volumes...)
		}
	}

	if len(confSecret) > 0 {
		//Append the resulting secret
		for _, secret := range confSecret {
			sec, _ := GetConfigSecretVolumes(secret)
			vm = append(vm, sec)
		}
	}
	return vm
}

// GetVolumeMounts - general VolumeMounts
func GetVolumeMounts(
	extraVol []glancev1.GlanceExtraVolMounts,
	svc []storage.PropagationType,
	confSecret []string,
) []corev1.VolumeMount {

	vm := []corev1.VolumeMount{
		{
			Name:      "config-data",
			MountPath: "/var/lib/config-data/merged",
			ReadOnly:  true,
		},
		{
			Name:      "config-data-merged",
			MountPath: "/var/lib/config-data/merged/glance.conf.d",
			ReadOnly:  true,
		},
		{
			Name:      "lib-data",
			MountPath: "/var/lib/glance",
			ReadOnly:  false,
		},
	}

	for _, exv := range extraVol {
		for _, vol := range exv.Propagate(svc) {
			vm = append(vm, vol.Mounts...)
		}
	}

	if len(confSecret) > 0 {
		//Append the resulting secret
		for _, secret := range confSecret {
			_, sec := GetConfigSecretVolumes(secret)
			vm = append(vm, sec)
		}
	}
	return vm
}

// GetConfigSecretVolumes - Returns a list of volumes associated with a list of Secret names
func GetConfigSecretVolumes(secretName string) (corev1.Volume, corev1.VolumeMount) {
	var config0640AccessMode int32 = 0640

	//Mount the resulting secret
	secretVol := corev1.Volume{
		Name: secretName,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName:  secretName,
				DefaultMode: &config0640AccessMode,
			},
		},
	}
	secretMount := corev1.VolumeMount{
		Name: secretName,
		// Each secret needs its own MountPath
		MountPath: DeploymentConfigDir + secretName + ".conf",
		SubPath:   secretName + ".conf",
		ReadOnly:  true,
	}
	return secretVol, secretMount
}
