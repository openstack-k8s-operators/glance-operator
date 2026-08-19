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
	"github.com/openstack-k8s-operators/lib-common/modules/common/volume"
	"github.com/openstack-k8s-operators/lib-common/modules/storage"
	corev1 "k8s.io/api/core/v1"
)

// configMode is the DefaultMode applied to every config Secret volume.
// 0440 (owner-read + group-read) is the tightest mode that works with the
// fsGroup-based access model: files carry live credentials, the Secret is
// immutable so no write bit is needed, and the non-root service user reads
// them via the supplemental fsGroup.
var configMode int32 = 0440

// GetVolumes - service volumes
func GetVolumes(
	name string,
	hasCinder bool,
	secretNames []string,
	extraVol []glancev1.GlanceExtraVolMounts,
	svc []storage.PropagationType,
) []corev1.Volume {

	vm := []corev1.Volume{
		{
			Name: "config-data",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					DefaultMode: &configMode,
					SecretName:  name + "-config-data",
				},
			},
		},
	}
	// ExtraMounts
	for _, exv := range extraVol {
		for _, vol := range exv.Propagate(svc) {
			for _, v := range vol.Volumes {
				volumeSource, _ := v.ToCoreVolumeSource()
				convertedVolume := corev1.Volume{
					Name:         v.Name,
					VolumeSource: *volumeSource,
				}
				vm = append(vm, convertedVolume)
			}
		}
	}
	// ConfigSecrets
	secretConfig, _ := volume.ConfigSecretVolumes(secretNames)
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

// runOnHostVolumeMount returns a VolumeMount that shims a host storage binary
// via the "scripts" secret's run-on-host nsenter wrapper, so glance (when
// Cinder is configured as a backend) can invoke host-installed multipath/iscsi
// tooling from inside the container (the pod already runs with HostPID: true).
func runOnHostVolumeMount(destPath string) corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:      "scripts",
		MountPath: destPath,
		SubPath:   "run-on-host",
	}
}

// GetVolumeMounts - general VolumeMounts
func GetVolumeMounts(
	secretNames []string,
	hasCinder bool,
	external bool,
	extraVol []glancev1.GlanceExtraVolMounts,
	svc []storage.PropagationType,
	apiMode string,
	wsgi bool,
) []corev1.VolumeMount {

	vm := []corev1.VolumeMount{
		// Writable dir MUST be listed before the SubPath mounts so Kubernetes
		// mounts the EmptyDir first, then overlays the read-only config files.
		volume.WritableDirVolumeMount(ConfigDirVolume, "/etc/glance/glance.conf.d"),
		{
			Name:      "config-data",
			MountPath: "/etc/glance/glance.conf.d/" + DefaultsConfigFileName,
			SubPath:   DefaultsConfigFileName,
			ReadOnly:  true,
		},
		{
			Name:      "config-data",
			MountPath: "/etc/glance/glance.conf.d/" + CustomServiceConfigFileName,
			SubPath:   CustomServiceConfigFileName,
			ReadOnly:  true,
		},
		{
			Name:      "config-data",
			MountPath: "/etc/glance/glance.conf.d/" + CustomServiceConfigSecretsFileName,
			SubPath:   CustomServiceConfigSecretsFileName,
			ReadOnly:  true,
		},
		{
			Name:      "config-data",
			MountPath: "/etc/my.cnf",
			SubPath:   "my.cnf",
			ReadOnly:  true,
		},
	}

	// httpd is the only container that runs Apache and needs its config
	// files; the native glance-api container (legacy/proxypass mode) only
	// needs the glance.conf.d files mounted above.
	if apiMode == "httpd" {
		vhostConf := "10-glance-proxypass.conf"
		if wsgi {
			vhostConf = "10-glance-wsgi.conf"
		}
		vm = append(vm,
			corev1.VolumeMount{
				Name:      "config-data",
				MountPath: "/etc/httpd/conf/httpd.conf",
				SubPath:   "httpd.conf",
				ReadOnly:  true,
			},
			corev1.VolumeMount{
				Name:      "config-data",
				MountPath: "/etc/httpd/conf.d/" + vhostConf,
				SubPath:   vhostConf,
				ReadOnly:  true,
			},
			corev1.VolumeMount{
				Name:      "config-data",
				MountPath: "/etc/httpd/conf.d/ssl.conf",
				SubPath:   "ssl.conf",
				ReadOnly:  true,
			},
		)
	}

	localPVC := []corev1.VolumeMount{
		{
			Name:      ServiceName,
			MountPath: "/var/lib/glance",
			ReadOnly:  false,
		},
	}
	// a PVC is mounted only if external is not set
	if !external {
		vm = append(vm, localPVC...)
	}
	for _, exv := range extraVol {
		for _, vol := range exv.Propagate(svc) {
			vm = append(vm, vol.Mounts...)
		}
	}
	_, secretConfig := volume.ConfigSecretVolumes(secretNames)
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
			runOnHostVolumeMount("/usr/sbin/multipath"),
			runOnHostVolumeMount("/usr/sbin/multipathd"),
			runOnHostVolumeMount("/usr/sbin/iscsiadm"),
			runOnHostVolumeMount("/lib/udev/scsi_id"),
			runOnHostVolumeMount("/usr/sbin/nvme"),
		}
		vm = append(vm, storageVolumeMounts...)
	}
	return vm
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

// GetScriptVolume -
func GetScriptVolume() []corev1.Volume {
	var scriptsVolumeDefaultMode int32 = 0755
	return []corev1.Volume{
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
	}
}

// GetScriptVolumeMount -
func GetScriptVolumeMount() []corev1.VolumeMount {
	return []corev1.VolumeMount{
		{
			Name:      "scripts",
			MountPath: "/usr/local/bin/container-scripts",
			ReadOnly:  true,
		},
	}
}

// GetAPIVolumes -
func GetAPIVolumes() []corev1.Volume {
	apiVolumes := []corev1.Volume{}
	// Writable config dir so the entrypoint can write runtime config
	// (e.g. worker_self_reference_url for distributed image import)
	apiVolumes = append(apiVolumes, volume.WritableDirVolume(ConfigDirVolume))
	// Append LogVolume to the apiVolumes: this will be used to stream logging
	apiVolumes = append(apiVolumes, volume.WritableDirVolume(LogVolume))
	// Append scripts volumeMount
	apiVolumes = append(apiVolumes, GetScriptVolume()...)
	// Append run-httpd volume for httpd PID file
	apiVolumes = append(apiVolumes, volume.WritableDirVolume(volume.RunHttpdVolumeName))
	return apiVolumes
}

// GetAPIVolumeMount -
func GetAPIVolumeMount(cacheSize string) []corev1.VolumeMount {
	apiVolumeMounts := []corev1.VolumeMount{}
	// Append LogVolume to apiVolumes: this will be used to stream logging
	apiVolumeMounts = append(apiVolumeMounts, volume.WritableDirVolumeMount(LogVolume, "/var/log/glance"))
	// Append ScriptsVolume to apiVolumes
	apiVolumeMounts = append(apiVolumeMounts, GetScriptVolumeMount()...)
	// Append run-httpd volume mount
	apiVolumeMounts = append(apiVolumeMounts, volume.WritableDirVolumeMount(volume.RunHttpdVolumeName, volume.RunHttpdMountPath))
	// If cache is provided, we expect the main glance_controller to request a
	// PVC that should be used for that purpose (according to ImageCache.Size)
	if len(cacheSize) > 0 {
		apiVolumeMounts = append(apiVolumeMounts, GetCacheVolumeMount()...)
	}
	return apiVolumeMounts
}
