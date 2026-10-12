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
	"path"
	"strconv"

	glancev1 "github.com/openstack-k8s-operators/glance-operator/api/v1beta1"
	"github.com/openstack-k8s-operators/glance-operator/internal/glance"
	"github.com/openstack-k8s-operators/lib-common/modules/users"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

// datadirSetupScriptName is the entry in the "glance-scripts" Secret that
// calls the datadir-setup script, which is rendered from
// templates/common/bin/datadir-setup.sh by the top-level Glance controller and
// mounted into the API pods via the "scripts" volume.
const datadirSetupScriptName = "datadir-setup.sh"

// DatadirInitContainers returns the init container that runs when the NFS (file)
// backend is detected.
func DatadirInitContainers(
	instance *glancev1.GlanceAPI,
	volumeMounts []corev1.VolumeMount,
) []corev1.Container {
	datadir := glance.GetFilesystemStoreDatadir(instance.Spec.CustomServiceConfig)

	return []corev1.Container{
		{
			Name:    glance.ServiceName + "-datadir-setup",
			Image:   instance.Spec.ContainerImage,
			Command: []string{path.Join(glance.ScriptMountPath, datadirSetupScriptName)},
			Args: []string{
				datadir,
				strconv.FormatInt(users.GlanceUID, 10),
				strconv.FormatInt(users.GlanceGID, 10),
			},
			SecurityContext: &corev1.SecurityContext{
				// Changing ownership on an NFS export is privileged on the NFS
				// server, and uid 0 is the only identity a client can present
				// that the server may accept for it. Scoped to this init
				// container: the API containers stay unprivileged as 42415.
				RunAsUser:    ptr.To(int64(0)),
				RunAsGroup:   ptr.To(int64(0)),
				RunAsNonRoot: ptr.To(false),
				// Capabilities are deliberately left at the runtime default
				// rather than Drop:[ALL] plus Add:[CHOWN]. chown(2) needs
				// CAP_CHOWN, and the anyuid-family SCCs set allowedCapabilities:
				// null, so a dropped capability could not be re-added without
				// the pod being refused at admission.
				AllowPrivilegeEscalation: ptr.To(false),
				SeccompProfile: &corev1.SeccompProfile{
					Type: corev1.SeccompProfileTypeRuntimeDefault,
				},
			},
			VolumeMounts: append(volumeMounts, glance.GetScriptVolumeMount()...),
			Resources:    instance.Spec.Resources,
		},
	}
}
