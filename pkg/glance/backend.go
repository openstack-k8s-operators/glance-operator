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
	glancev1beta1 "github.com/openstack-k8s-operators/glance-operator/api/v1beta1"
	"net"
	"strings"
)

/* This is a utily function that validates the comma separated Mon list defined
for the external ceph cluster; it also checks the provided IP addresses are not
malformed */
func validateMons(ipList string) bool {
	for _, ip := range strings.Split(ipList, ",") {
		if net.ParseIP(strings.Trim(ip, " ")) == nil {
			return false
		}
	}
	return true
}

// SetGlanceBackend is a function that computes the right backend used by Glance.
// GlanceBackend is set to file by default. However, if Ceph parameters are
// provided means we want to use it as backend. This function checks the Ceph
// related parameters and returns the right backend.
func SetGlanceBackend(instance *glancev1beta1.GlanceAPI) string {
	// Validate parameters to enable CephBackend
	if instance.Spec.CephBackend.CephClusterFSID != "" &&
		validateMons(instance.Spec.CephBackend.CephClusterMonHosts) &&
		instance.Spec.CephBackend.CephClientKey != "" {
		return "rbd"
	}
	/* file is the default backend: if we don't have any ceph
	parameter or they are malformed, set backend to file */
	return "file"
}
