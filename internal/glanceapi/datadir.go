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
	"errors"
	"fmt"
	"path"
	"strings"

	glancev1 "github.com/openstack-k8s-operators/glance-operator/api/v1beta1"
	"github.com/openstack-k8s-operators/glance-operator/internal/glance"
	"github.com/openstack-k8s-operators/lib-common/modules/common/pod"
	"github.com/openstack-k8s-operators/lib-common/modules/users"
	corev1 "k8s.io/api/core/v1"
)

// datadirCheckScriptName is the entry in the "glance-scripts" Secret that
// holds the datadir-check script, which is rendered from
// templates/common/bin/datadir-check.sh by the top-level Glance controller and
// mounted into the API pods via the "scripts" volume.
const datadirCheckScriptName = "datadir-check.sh"

// errDatadirNotOnDataVolume is returned when the configured datadir is not
// covered by a writable image store mount, which means Glance would write
// images to the container filesystem and lose them on restart.
var errDatadirNotOnDataVolume = errors.New("filesystem_store_datadir is not on a writable image store volume")

// DatadirCheckInitContainers returns the init container that runs when the NFS
// (file) backend is detected. It does not repair anything: it runs as the
// glance user and fails the pod when the datadir is not usable by that user,
// so the required server-side fix is reported up front instead of surfacing
// later as rejected image uploads.
func DatadirCheckInitContainers(
	instance *glancev1.GlanceAPI,
	dataMounts []corev1.VolumeMount,
) ([]corev1.Container, error) {
	datadir := glance.GetFilesystemStoreDatadir(instance.Spec.CustomServiceConfig)
	mount, err := datadirMount(datadir, dataMounts)
	if err != nil {
		return nil, err
	}
	mounts := append([]corev1.VolumeMount{mount}, glance.GetScriptVolumeMount()...)

	return []corev1.Container{
		{
			Name:    glance.ServiceName + "-datadir-check",
			Image:   instance.Spec.ContainerImage,
			Command: []string{path.Join(glance.ScriptMountPath, datadirCheckScriptName)},
			Args:    []string{datadir},
			// Same identity and restrictions as the glance-api container: the
			// check is only meaningful if it observes the access the service
			// will actually get.
			SecurityContext: pod.RestrictiveSecurityContext(users.GlanceUID, users.GlanceGID),
			VolumeMounts:    mounts,
			Resources:       instance.Spec.Resources,
		},
	}, nil
}

// datadirMount returns the most specific writable data mount the datadir lives
// on, so the init container checks the directory at the same path, and on the
// same volume, that glance-api will later write images to. Nothing else the
// API mounts is given to the init container.
func datadirMount(datadir string, dataMounts []corev1.VolumeMount) (corev1.VolumeMount, error) {
	datadirPath := path.Clean(datadir)
	var selected corev1.VolumeMount
	selectedPath := ""
	for _, mount := range dataMounts {
		mountPath := path.Clean(mount.MountPath)
		if mount.ReadOnly || !pathContains(mountPath, datadirPath) ||
			(selectedPath != "" && len(mountPath) <= len(selectedPath)) {
			continue
		}
		selected, selectedPath = mount, mountPath
	}
	if selectedPath == "" {
		return selected, fmt.Errorf("%w: %q", errDatadirNotOnDataVolume, datadir)
	}
	return selected, nil
}

// pathContains reports whether target is dir itself or lives below it. Both
// paths are expected to be cleaned already.
func pathContains(dir string, target string) bool {
	if dir == target {
		return true
	}
	if !strings.HasSuffix(dir, "/") {
		// path.Clean only keeps a trailing slash for the root directory
		dir += "/"
	}
	return strings.HasPrefix(target, dir)
}
