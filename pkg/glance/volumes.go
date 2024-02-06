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
	var scriptsVolumeDefaultMode int32 = 0755
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
			// TODO: Scripts have no reason to be secrets, should move to configmap
			// For now scripts only exist for Cinder backend
			{
				Name: "scripts",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						DefaultMode: &scriptsVolumeDefaultMode,
						// -scripts are inherited from top level CR
						SecretName: ServiceName + "-scripts",
					},
				},
			},
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
			{
				Name: "lib-modules",
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: "/lib/modules",
					},
				},
			},
			{
				Name: "run",
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: "/run",
					},
				},
			},
			// /sys needed for os-brick code that looks for information there
			{
				Name: "sys",
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: "/sys",
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
			{
				Name: "etc-nvme",
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: "/etc/nvme",
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
			Name:      ServiceName,
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
			// For now scripts only exist for Cinder backend
			{
				Name:      "scripts",
				MountPath: "/usr/local/bin/container-scripts",
				ReadOnly:  true,
			},
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
				Name:      "lib-modules",
				MountPath: "/lib/modules",
				ReadOnly:  true,
			},
			{
				Name:      "run",
				MountPath: "/run",
			},
			{
				Name:      "sys",
				MountPath: "/sys",
			},
			{
				Name:      "var-locks-brick",
				MountPath: "/var/locks/openstack/os-brick",
				ReadOnly:  false,
			},
			{
				Name:      "etc-nvme",
				MountPath: "/etc/nvme",
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

// GetHttpdVolumeMount - Returns the VolumeMounts used by the httpd sidecar
func GetHttpdVolumeMount() []corev1.VolumeMount {
	return []corev1.VolumeMount{
		{
			Name:      "config-data",
			MountPath: "/etc/httpd/conf/httpd.conf",
			SubPath:   "httpd.conf",
			ReadOnly:  true,
		},
		{
			Name:      "config-data",
			MountPath: "/etc/httpd/conf.d/10-glance.conf",
			SubPath:   "10-glance-httpd.conf",
			ReadOnly:  true,
		},
		{
			Name:      "config-data",
			MountPath: "/etc/httpd/conf.d/ssl.conf",
			SubPath:   "ssl.conf",
			ReadOnly:  true,
		},
	}
}

// GetCacheVolume - Return the Volume used for image caching purposes
func GetCacheVolume(pvcName string) []corev1.Volume {
	return []corev1.Volume{
		{
			Name: "glance-cache",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: pvcName,
				},
			},
		},
	}
}

// GetCacheVolumeMount - Return the VolumeMount used for image caching purposes
func GetCacheVolumeMount() []corev1.VolumeMount {
	return []corev1.VolumeMount{
		{
			Name:      "glance-cache",
			MountPath: ImageCacheDir,
			ReadOnly:  false,
		},
	}
}
