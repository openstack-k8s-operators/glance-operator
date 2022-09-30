/*
Copyright 2022.

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
	"github.com/openstack-k8s-operators/lib-common/modules/storage/ceph"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// DbSyncHash hash
	DbSyncHash = "dbsync"
	// APIInternal -
	APIInternal = "internal"
	// APIExternal -
	APIExternal = "external"
)

// GlanceSpec defines the desired state of Glance
type GlanceSpec struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=glance
	// ServiceUser - optional username used for this service to register in glance
	ServiceUser string `json:"serviceUser"`

	// +kubebuilder:validation:Required
	// Glance Container Image URL
	ContainerImage string `json:"containerImage,omitempty"`

	// +kubebuilder:validation:Required
	// MariaDB instance name
	// Right now required by the maridb-operator to get the credentials from the instance to create the DB
	// Might not be required in future
	DatabaseInstance string `json:"databaseInstance,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default=glance
	// DatabaseUser - optional username used for glance DB, defaults to glance
	// TODO: -> implement needs work in mariadb-operator, right now only glance
	DatabaseUser string `json:"databaseUser"`

	// +kubebuilder:validation:Required
	// Secret containing OpenStack password information for glance GlanceDatabasePassword
	Secret string `json:"secret"`

	// +kubebuilder:validation:Optional
	// PasswordSelectors - Selectors to identify the DB password from the Secret
	PasswordSelectors PasswordSelector `json:"passwordSelectors,omitempty"`

	// +kubebuilder:validation:Optional
	// Debug - enable debug for different deploy stages. If an init container is used, it runs and the
	// actual action pod gets started with sleep infinity
	Debug GlanceDebug `json:"debug,omitempty"`

	// +kubebuilder:validation:Optional
	// CephBackend - The ceph Backend structure with all the parameters
	CephBackend ceph.Backend `json:"cephBackend,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default=false
	// PreserveJobs - do not delete jobs after they finished e.g. to check logs
	PreserveJobs bool `json:"preserveJobs,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default="# add your customization here"
	// CustomServiceConfig - customize the service config using this parameter to change service defaults,
	// or overwrite rendered information using raw OpenStack config format. The content gets added to
	// to /etc/<service>/<service>.conf.d directory as custom.conf file.
	CustomServiceConfig string `json:"customServiceConfig,omitempty"`

	// +kubebuilder:validation:Optional
	// ConfigOverwrite - interface to overwrite default config files like e.g. logging.conf or policy.json.
	// But can also be used to add additional files. Those get added to the service config dir in /etc/<service> .
	// TODO: -> implement
	DefaultConfigOverwrite map[string]string `json:"defaultConfigOverwrite,omitempty"`

	// +kubebuilder:validation:Optional
	// StorageClass
	StorageClass string `json:"storageClass,omitempty"`

	// +kubebuilder:validation:Required
	// StorageRequest
	StorageRequest string `json:"storageRequest"`

	// +kubebuilder:validation:Required
	// GlanceAPIInternal - Spec definition for the internal and admin API service of this Glance deployment
	GlanceAPIInternal GlanceAPISpec `json:"glanceAPIInternal"`

	// +kubebuilder:validation:Required
	// GlanceAPIExternal - Spec definition for the external API service of this Glance deployment
	GlanceAPIExternal GlanceAPISpec `json:"glanceAPIExternal"`
}

// PasswordSelector to identify the DB and AdminUser password from the Secret
type PasswordSelector struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default="GlanceDatabasePassword"
	// Database - Selector to get the glance database user password from the Secret
	// TODO: not used, need change in mariadb-operator
	Database string `json:"database,omitempty"`
	// +kubebuilder:validation:Optional
	// +kubebuilder:default="GlancePassword"
	// Database - Selector to get the glance service password from the Secret
	Service string `json:"admin,omitempty"`
}

// GlanceDebug defines the observed state of GlanceAPIDebug
type GlanceDebug struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=false
	// DBSync enable debug
	DBSync bool `json:"dbSync,omitempty"`
}

// GlanceStatus defines the observed state of Glance
type GlanceStatus struct {
	// Map of hashes to track e.g. job status
	Hash map[string]string `json:"hash,omitempty"`

	// API endpoint
	APIEndpoints map[string]string `json:"apiEndpoint,omitempty"`

	// ServiceID
	ServiceID string `json:"serviceID,omitempty"`

	// Conditions
	Conditions condition.Conditions `json:"conditions,omitempty" optional:"true"`

	// Glance Database Hostname
	DatabaseHostname string `json:"databaseHostname,omitempty"`

	// ReadyCount of internal and admin Glance API instance
	GlanceAPIInternalReadyCount int32 `json:"cinderAPIReadyInternalCount,omitempty"`

	// ReadyCount of external and admin Glance API instance
	GlanceAPIExternalReadyCount int32 `json:"cinderAPIReadyExternalCount,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[0].status",description="Status"
//+kubebuilder:printcolumn:name="Message",type="string",JSONPath=".status.conditions[0].message",description="Message"

// Glance is the Schema for the glances API
type Glance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GlanceSpec   `json:"spec,omitempty"`
	Status GlanceStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// GlanceList contains a list of Glance
type GlanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Glance `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Glance{}, &GlanceList{})
}

// IsReady - returns true if service is ready to serve requests
func (instance Glance) IsReady() bool {
	// Ready when:
	// - Both the internal and external API endpoints are ready (count >= 1)
	return instance.Status.GlanceAPIInternalReadyCount >= 1 && instance.Status.GlanceAPIExternalReadyCount >= 1
}
