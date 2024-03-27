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
	"github.com/openstack-k8s-operators/lib-common/modules/storage"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// DbSyncHash hash
	DbSyncHash = "dbsync"
	// APIInternal -
	APIInternal = "internal"
	// APIExternal -
	APIExternal = "external"
	// APISingle -
	APISingle = "single"
	// APIEdge -
	APIEdge = "edge"
)

// GlanceSpec defines the desired state of Glance
type GlanceSpecCore struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=glance
	// ServiceUser - optional username used for this service to register in glance
	ServiceUser string `json:"serviceUser"`

	// +kubebuilder:validation:Required
	// MariaDB instance name
	// Right now required by the maridb-operator to get the credentials from the instance to create the DB
	// Might not be required in future
	DatabaseInstance string `json:"databaseInstance"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default=glance
	// DatabaseAccount - name of MariaDBAccount which will be used to connect.
	DatabaseAccount string `json:"databaseAccount"`

	// +kubebuilder:validation:Required
	// +kubebuilder:default=memcached
	// Memcached instance name.
	MemcachedInstance string `json:"memcachedInstance"`

	// +kubebuilder:validation:Required
	// Secret containing OpenStack password information for glance's keystone
	// password; no longer used for database password
	Secret string `json:"secret"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default={service: GlancePassword}
	// PasswordSelectors - Selectors to identify the DB and ServiceUser password from the Secret
	PasswordSelectors PasswordSelector `json:"passwordSelectors"`

	// +kubebuilder:validation:Optional
	// NodeSelector to target subset of worker nodes running this service
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default=false
	// PreserveJobs - do not delete jobs after they finished e.g. to check logs
	PreserveJobs bool `json:"preserveJobs"`

	// +kubebuilder:validation:Optional
	// CustomServiceConfig - customize the service config using this parameter to change service defaults,
	// or overwrite rendered information using raw OpenStack config format. The content gets added to
	// to /etc/<service>/<service>.conf.d directory as custom.conf file.
	CustomServiceConfig string `json:"customServiceConfig,omitempty"`

	// +kubebuilder:validation:Optional
	// CustomServiceConfigSecrets - customize the service config using this parameter to specify Secrets
	// that contain sensitive service config data. The content of each Secret gets added to the
	// /etc/<service>/<service>.conf.d directory as a custom config file.
	CustomServiceConfigSecrets []string `json:"customServiceConfigSecrets,omitempty"`

	// +kubebuilder:validation:Optional
	// StorageClass
	StorageClass string `json:"storageClass,omitempty"`

	// +kubebuilder:validation:Required
	// StorageRequest
	StorageRequest string `json:"storageRequest"`

	// +kubebuilder:validation:Required
	// +kubebuilder:default={}
	// GlanceAPIs - Spec definition for the API service of this Glance deployment
	GlanceAPIs map[string]GlanceAPITemplate `json:"glanceAPIs"`

	// +kubebuilder:validation:Optional
	// ExtraMounts containing conf files and credentials
	ExtraMounts []GlanceExtraVolMounts `json:"extraMounts,omitempty"`

	// +kubebuilder:validation:Optional
	// Quotas is defined, per-tenant quotas are enforced according to the
	// registered keystone limits
	Quotas QuotaLimits `json:"quotas,omitempty"`

	// ImageCache -
	// +kubebuilder:default={}
	ImageCache ImageCache `json:"imageCache"`

	// +kubebuilder:validation:Required
	// +kubebuilder:default=""
	// KeystoneEndpoint - indicates which glanceAPI should be registered in the
	// keystone catalog, and it acts as a selector for the underlying glanceAPI(s)
	// that can be specified by name
	KeystoneEndpoint string `json:"keystoneEndpoint"`

	// +kubebuilder:validation:Optional
	// DBPurge parameters -
	DBPurge DBPurge `json:"dbPurge,omitempty"`
}

// GlanceSpec defines the desired state of Glance
type GlanceSpec struct {

	// +kubebuilder:validation:Required
	// Glance Container Image URL (will be set to environmental default if empty)
	ContainerImage string `json:"containerImage"`

	GlanceSpecCore `json:",inline"`
}

// PasswordSelector to identify the DB and AdminUser password from the Secret
type PasswordSelector struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default="GlancePassword"
	// Service - Selector to get the glance service password from the Secret
	Service string `json:"service"`
}

// DBPurge struct is used to model the parameters exposed to the Glance API CronJob
type DBPurge struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=30
	// +kubebuilder:validation:Minimum=1
	// Age is the DBPurgeAge parameter and indicates the number of days of purging DB records
	Age int `json:"age"`
	// +kubebuilder:validation:Optional
	// +kubebuilder:default="1 0 * * *"
	//Schedule defines the crontab format string to schedule the DBPurge cronJob
	Schedule string `json:"schedule"`
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

	// GlanceAPIReadyCounts -
	GlanceAPIReadyCounts map[string]int32 `json:"glanceAPIReadyCounts,omitempty"`
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

// IsReady - returns true if the underlying GlanceAPI is reconciled successfully
// and set the ReadyCondition to True: it is mirrored to the top-level CR, so
// the purpose of this function is to let the openstack-operator to gather the
// Status of the entire Glance Deployment (intended as a single entity) from a
// single place
func (instance Glance) IsReady() bool {
	return instance.Status.Conditions.IsTrue(condition.ReadyCondition)
}

// QuotaLimits - The parameters exposed to the top level glance CR that
// represents the limits we set in keystone
type QuotaLimits struct {
	// +kubebuilder:default=0
	ImageSizeTotal int `json:"imageSizeTotal"`
	// +kubebuilder:default=0
	ImageStageTotal int `json:"imageStageTotal"`
	// +kubebuilder:default=0
	ImageCountTotal int `json:"imageCountTotal"`
	// +kubebuilder:default=0
	ImageCountUpload int `json:"imageCountUpload"`
}

// GlanceExtraVolMounts exposes additional parameters processed by the glance-operator
// and defines the common VolMounts structure provided by the main storage module
type GlanceExtraVolMounts struct {
	// +kubebuilder:validation:Optional
	Name string `json:"name,omitempty"`
	// +kubebuilder:validation:Optional
	Region string `json:"region,omitempty"`
	// +kubebuilder:validation:Required
	VolMounts []storage.VolMounts `json:"extraVol"`
}

// Propagate is a function used to filter VolMounts according to the specified
// PropagationType array
func (g *GlanceExtraVolMounts) Propagate(svc []storage.PropagationType) []storage.VolMounts {
	var vl []storage.VolMounts
	for _, gv := range g.VolMounts {
		vl = append(vl, gv.Propagate(svc)...)
	}
	return vl
}

// RbacConditionsSet - set the conditions for the rbac object
func (instance Glance) RbacConditionsSet(c *condition.Condition) {
	instance.Status.Conditions.Set(c)
}

// RbacNamespace - return the namespace
func (instance Glance) RbacNamespace() string {
	return instance.Namespace
}

// RbacResourceName - return the name to be used for rbac objects (serviceaccount, role, rolebinding)
func (instance Glance) RbacResourceName() string {
	return "glance-" + instance.Name
}

// IsQuotaEnabled - return true if one of the QuotaLimits values is set
func (instance Glance) IsQuotaEnabled() bool {
	return (instance.Spec.Quotas.ImageSizeTotal > 0 ||
		instance.Spec.Quotas.ImageCountTotal > 0 ||
		instance.Spec.Quotas.ImageStageTotal > 0 ||
		instance.Spec.Quotas.ImageCountUpload > 0)
}

// GetQuotaLimits - get the glance instance data structure containing
// what has been set in the CR
func (instance Glance) GetQuotaLimits() map[string]int {
	return map[string]int{
		"image_count_uploading": instance.Spec.Quotas.ImageCountUpload,
		"image_count_total":     instance.Spec.Quotas.ImageCountTotal,
		"image_stage_total":     instance.Spec.Quotas.ImageStageTotal,
		"image_size_total":      instance.Spec.Quotas.ImageSizeTotal,
	}
}
