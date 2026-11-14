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
	"slices"
	"strings"

	glancev1 "github.com/openstack-k8s-operators/glance-operator/api/v1beta1"
	"github.com/openstack-k8s-operators/glance-operator/internal/glance"
	"github.com/openstack-k8s-operators/lib-common/modules/common/pod"
	"github.com/openstack-k8s-operators/lib-common/modules/users"
	corev1 "k8s.io/api/core/v1"
)

// datadirCheckScriptName is the entry in the "glance-scripts" Secret rendered
// from templates/common/bin/datadir-check.sh and mounted via "scripts"
const datadirCheckScriptName = "datadir-check.sh"

// errDatadirNotOnDataVolume is returned when the datadir is not covered by a
// writable image store mount, so images would be lost on restart
var errDatadirNotOnDataVolume = errors.New("filesystem_store_datadir is not on a writable image store volume")

// errNoFileStoreDatadir is returned when a file store is enabled but no
// datadir could be resolved, e.g. an enabled_backends entry with no store id
var errNoFileStoreDatadir = errors.New("no filesystem_store_datadir resolved for the enabled file stores")

// DatadirCheckInitContainers returns the init container that stops the pod
// when the datadir of an enabled file store is not usable by the glance user
func DatadirCheckInitContainers(
	instance *glancev1.GlanceAPI,
	dataMounts []corev1.VolumeMount,
) ([]corev1.Container, error) {
	datadirs := glance.GetFilesystemStoreDatadirs(instance.Spec.CustomServiceConfig)
	if len(datadirs) == 0 {
		return nil, errNoFileStoreDatadir
	}
	mounts, err := datadirMounts(datadirs, dataMounts)
	if err != nil {
		return nil, err
	}

	return []corev1.Container{
		{
			Name:    glance.ServiceName + "-datadir-check",
			Image:   instance.Spec.ContainerImage,
			Command: []string{path.Join(glance.ScriptMountPath, datadirCheckScriptName)},
			Args:    datadirs,
			// Same identity as glance-api: the check is only meaningful if it
			// observes the access the service will actually get
			SecurityContext: pod.RestrictiveSecurityContext(users.GlanceUID, users.GlanceGID),
			VolumeMounts:    mounts,
			Resources:       instance.Spec.Resources,
		},
	}, nil
}

// datadirMounts returns the deduplicated data mounts the datadirs live on,
// plus the scripts mount: nothing else the API mounts is exposed to the check
func datadirMounts(datadirs []string, dataMounts []corev1.VolumeMount) ([]corev1.VolumeMount, error) {
	mounts := []corev1.VolumeMount{}
	for _, datadir := range datadirs {
		// the most specific writable mount is the volume, and the path, that
		// glance-api will later write the images of this store to
		datadirPath := path.Clean(datadir)
		var selected corev1.VolumeMount
		longest := -1

		for _, mount := range dataMounts {
			mountPath := path.Clean(mount.MountPath)
			if mount.ReadOnly || len(mountPath) <= longest || !pathContains(mountPath, datadirPath) {
				continue
			}
			selected, longest = mount, len(mountPath)
		}
		if longest < 0 {
			return nil, fmt.Errorf("%w: %q", errDatadirNotOnDataVolume, datadir)
		}
		// two file stores can be backed by the same volume
		if !slices.Contains(mounts, selected) {
			mounts = append(mounts, selected)
		}
	}
	return append(mounts, glance.GetScriptVolumeMount()...), nil
}

// pathContains reports whether the cleaned target is dir itself or lives below
// it. TrimSuffix covers "/", the only dir path.Clean leaves a slash on.
func pathContains(dir string, target string) bool {
	return target == dir || strings.HasPrefix(target, strings.TrimSuffix(dir, "/")+"/")
}
