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
	"strconv"

	glancev1 "github.com/openstack-k8s-operators/glance-operator/api/v1beta1"
	"github.com/openstack-k8s-operators/lib-common/modules/storage"
	corev1 "k8s.io/api/core/v1"
)

// GetVolumes - service volumes
func GetVolumes(name string, pvcName string, hasCinder bool, secretNames []string, extraVol []glancev1.GlanceExtraVolMounts, svc []storage.PropagationType) []corev1.Volume {
	//var scriptsVolumeDefaultMode int32 = 0755
	var config0644AccessMode int32 = 0644

	vm := []corev1.Volume{
		{
			Name: "config-data",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					DefaultMode: &config0644AccessMode,
					SecretName:  name + "-config-data",
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
	}

	for _, exv := range extraVol {
		for _, vol := range exv.Propagate(svc) {
			vm = append(vm, vol.Volumes...)
		}
	}
	secretConfig, _ := GetConfigSecretVolumes(secretNames)
	vm = append(vm, secretConfig...)

	if hasCinder {
		var dirOrCreate = corev1.HostPathDirectoryOrCreate

		// Add the required volumes
		storageVolumes := []corev1.Volume{
			// os-brick reads the initiatorname.iscsi from theere
			{
				Name: "etc-iscsi",
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: "/etc/iscsi",
					},
				},
			},
			// /dev needed for os-brick code that looks for things there and
			// for Volume and Backup operations that access data
			{
				Name: "dev",
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: "/dev",
					},
				},
			},
			// os-brick locks need to be shared between the different volume
			// consumers (available since OSP18)
			{
				Name: "var-locks-brick",
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: "/var/locks/openstack/os-brick",
						Type: &dirOrCreate,
					},
				},
			},
		}
		vm = append(vm, storageVolumes...)
	}
	return vm
}

// GetVolumeMounts - general VolumeMounts
func GetVolumeMounts(secretNames []string, hasCinder bool, extraVol []glancev1.GlanceExtraVolMounts, svc []storage.PropagationType) []corev1.VolumeMount {

	vm := []corev1.VolumeMount{
		{
			Name:      "config-data",
			MountPath: "/var/lib/config-data/default",
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
	_, secretConfig := GetConfigSecretVolumes(secretNames)
	vm = append(vm, secretConfig...)
	if hasCinder {
		storageVolumeMounts := []corev1.VolumeMount{
			{
				Name:      "etc-iscsi",
				MountPath: "/etc/iscsi",
				ReadOnly:  true,
			},
			{
				Name:      "dev",
				MountPath: "/dev",
			},
			{
				Name:      "var-locks-brick",
				MountPath: "/var/locks/openstack/os-brick",
				ReadOnly:  false,
			},
		}
		vm = append(vm, storageVolumeMounts...)
	}
	return vm
}

// GetConfigSecretVolumes - Returns a list of volumes associated with a list of Secret names
func GetConfigSecretVolumes(secretNames []string) ([]corev1.Volume, []corev1.VolumeMount) {
	var config0640AccessMode int32 = 0640
	secretVolumes := []corev1.Volume{}
	secretMounts := []corev1.VolumeMount{}

	for idx, secretName := range secretNames {
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
			MountPath: "/var/lib/config-data/secret-" + strconv.Itoa(idx),
			ReadOnly:  true,
		}
		secretVolumes = append(secretVolumes, secretVol)
		secretMounts = append(secretMounts, secretMount)
	}

	return secretVolumes, secretMounts
}

// GetLogVolumeMount - Returns the VolumeMount used for logging purposes
func GetLogVolumeMount() []corev1.VolumeMount {
	return []corev1.VolumeMount{
		{
			Name:      LogVolume,
			MountPath: "/var/log/glance",
			ReadOnly:  false,
		},
	}
}

// GetLogVolume - Returns the Volume used for logging purposes
func GetLogVolume() []corev1.Volume {
	return []corev1.Volume{
		{
			Name: LogVolume,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{Medium: ""},
			},
		},
	}
}
