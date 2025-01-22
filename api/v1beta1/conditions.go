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

package v1beta1

import (
	condition "github.com/openstack-k8s-operators/lib-common/modules/common/condition"
)

// Common Messages used by API objects.
const (
	// GlanceAPIReadyInitMessage
	GlanceAPIReadyInitMessage = "GlanceAPI not started"
	// GlanceAPIReadyErrorMessage
	GlanceAPIReadyErrorMessage = "GlanceAPI error occured %s"
	// CinderInitMessage
	CinderInitMessage = "Waiting for Cinder resources"
	// CinderReadyMessage
	CinderReadyMessage = "Cinder resources exist"
	// CinderReadyErrorMessage
	CinderReadyErrorMessage = "Cinder resource error %s"
	// GlanceAPIReadyCondition Status=True condition which indicates if the GlanceAPI is configured and operational
	GlanceAPIReadyCondition condition.Type = "GlanceAPIReady"
	// CinderCondition
	CinderCondition = "CinderReady"
	// GlanceLayoutUpdateErrorMessage
	GlanceLayoutUpdateErrorMessage = "The GlanceAPI layout (type) cannot be modified. To proceed, please add a new API with the desired layout and then decommission the previous API"
	// KeystoneEndpointErrorMessage
	KeystoneEndpointErrorMessage = "KeystoneEndpoint is assigned to an invalid GlanceAPI instance"
	// InvalidBackendErrorMessageGeneric
	InvalidBackendErrorMessageGeneric = "Invalid backend configuration detected"
	// InvalidBackendErrorMessageSplit
	InvalidBackendErrorMessageSplit = "The GlanceAPI layout type: split cannot be used in combination with File and NFS backend"
	// InvalidBackendErrorMessageSingle
	InvalidBackendErrorMessageSingle = "The GlanceAPI layout type: single can only be used in combination with File and NFS backend"
)
