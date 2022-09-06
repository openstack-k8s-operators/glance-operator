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
	"fmt"
	glancev1beta1 "github.com/openstack-k8s-operators/glance-operator/api/v1beta1"
	"net"
	"sort"
	"strings"
)

// CephDefaults is used as type to reference defaults
type CephDefaults string

// Spec is the Glance Ceph Backend struct defining defaults
type Spec struct {
	CephDefaults *CephDefaults
}

// Glance defaults
const (
	CephDefaultGlanceUser CephDefaults = "openstack"
	CephDefaultGlancePool CephDefaults = "images"
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

// GetCephGlancePool is a function that validate the pool passed as input, and return
// images no pool is given
func GetCephGlancePool(instance *glancev1beta1.GlanceAPI) string {

	if pool, found := instance.Spec.CephBackend.CephPools["glance"]; found {
		return pool.CephPoolName
	}
	return string(CephDefaultGlancePool)
}

// GetCephRbdUser is a function that validate the user passed as input, and return
// openstack if no user is given
func GetCephRbdUser(instance *glancev1beta1.GlanceAPI) string {

	if instance.Spec.CephBackend.CephUser == "" {
		return string(CephDefaultGlanceUser)
	}
	return instance.Spec.CephBackend.CephUser
}

// GetCephOsdCaps is a function that returns the Caps for each defined pool
func GetCephOsdCaps(instance *glancev1beta1.GlanceAPI) string {

	var osdCaps string // the resulting string containing caps

	/**
	A map of strings (pool service/name in this case) is, by definition, an
	unordered structure, and let the function return a different pattern
	each time. This causes the ConfigMap hash to change, and the pod being
	redeployed because the operator detects the different hash. Sorting the
	resulting array of pools makes everything predictable
	**/
	var plist []string
	for _, pool := range instance.Spec.CephBackend.CephPools {
		plist = append(plist, pool.CephPoolName)
	}
	// sort the pool names
	sort.Strings(plist)

	// Build the resulting caps from the _ordered_ array applying the template
	for _, pool := range plist {
		if pool != "" {
			osdCaps += fmt.Sprintf("profile rbd pool=%s,", pool)
		}
	}
	// Default case, no pools are specified, adding just "images" (the default)
	if osdCaps == "" {
		osdCaps = "profile rbd pool=" + string(CephDefaultGlancePool) + ","
	}

	return strings.TrimSuffix(osdCaps, ",")
}
