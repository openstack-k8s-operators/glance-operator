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
	"github.com/openstack-k8s-operators/lib-common/modules/common/env"
	corev1 "k8s.io/api/core/v1"
	"strconv"
)

// APIDetails information
type APIDetails struct {
	ContainerImage       string
	DatabaseHost         string
	DatabaseUser         string
	DatabaseName         string
	TransportURL         string
	OSPSecret            string
	DBPasswordSelector   string
	UserPasswordSelector string
	VolumeMounts         []corev1.VolumeMount
	QuotaEnabled         bool
	Privileged           bool
}

const (
	// InitContainerCommand -
	InitContainerCommand = "/usr/local/bin/container-scripts/init.sh"
)

// InitContainer - init container for glance api pods
func InitContainer(init APIDetails) []corev1.Container {
	runAsUser := int64(0)
	trueVar := true

	args := []string{
		"-c",
		InitContainerCommand,
	}

	securityContext := &corev1.SecurityContext{
		RunAsUser: &runAsUser,
	}

	if init.Privileged {
		securityContext.Privileged = &trueVar
	}

	envVars := map[string]env.Setter{}
	envVars["DatabaseHost"] = env.SetValue(init.DatabaseHost)
	envVars["DatabaseUser"] = env.SetValue(init.DatabaseUser)
	envVars["DatabaseName"] = env.SetValue(init.DatabaseName)
	envVars["QuotaEnabled"] = env.SetValue(strconv.FormatBool(init.QuotaEnabled))

	envs := []corev1.EnvVar{
		{
			Name: "DatabasePassword",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: init.OSPSecret,
					},
					Key: init.DBPasswordSelector,
				},
			},
		},
		{
			Name: "GlancePassword",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: init.OSPSecret,
					},
					Key: init.UserPasswordSelector,
				},
			},
		},
		// TODO
		// {
		// 	Name: "TransportUrl",
		// 	ValueFrom: &corev1.EnvVarSource{
		// 		SecretKeyRef: &corev1.SecretKeySelector{
		// 			LocalObjectReference: corev1.LocalObjectReference{
		// 				Name: init.OSPSecret,
		// 			},
		// 			Key: "TransportUrl",
		// 		},
		// 	},
		// },
	}
	envs = env.MergeEnvs(envs, envVars)

	return []corev1.Container{
		{
			Name:            "init",
			Image:           init.ContainerImage,
			SecurityContext: securityContext,
			Command: []string{
				"/bin/bash",
			},
			Args:         args,
			Env:          envs,
			VolumeMounts: init.VolumeMounts,
		},
	}
}
