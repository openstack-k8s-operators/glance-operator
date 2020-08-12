package glance

import (
	corev1 "k8s.io/api/core/v1"
)

// common Glance API Volumes
func getVolumes(name string) []corev1.Volume {

	return []corev1.Volume{

		{
			Name: "kolla-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: name,
					},
					Items: []corev1.KeyToPath{
						{
							Key:  "config.json",
							Path: "config.json",
						},
					},
				},
			},
		},
		{
			Name: "config-data",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: name,
					},
					Items: []corev1.KeyToPath{
						{
							Key:  "glance-api.conf",
							Path: "glance-api.conf",
						},
						{
							Key:  "logging.conf",
							Path: "logging.conf",
						},
					},
				},
			},
		},
		{
			Name: "lib-data",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: name,
				},
			},
		},
	}

}

// common Glance API VolumeMounts
func getVolumeMounts() []corev1.VolumeMount {
	return []corev1.VolumeMount{
		{
			MountPath: "/var/lib/config-data",
			ReadOnly:  true,
			Name:      "config-data",
		},
		{
			MountPath: "/var/lib/kolla/config_files",
			ReadOnly:  true,
			Name:      "kolla-config",
		},
		{
			MountPath: "/var/lib/glance",
			ReadOnly:  false,
			Name:      "lib-data",
		},
	}

}
