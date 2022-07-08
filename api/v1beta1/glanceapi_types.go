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
	"fmt"

	common "github.com/openstack-k8s-operators/lib-common/pkg/common"
	condition "github.com/openstack-k8s-operators/lib-common/pkg/condition"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// GlanceFinalizer allows GlanceAPIReconciler to clean up resources associated with GlanceAPI before
	// removing it from the apiserver.
	GlanceFinalizer = "glanceapi.osp-director.openstack.org"

	// DbSyncHash hash
	DbSyncHash = "dbsync"

	// DeploymentHash hash used to detect changes
	DeploymentHash = "deployment"
)

// GlanceAPISpec defines the desired state of GlanceAPI
type GlanceAPISpec struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=placement
	// ServiceUser - optional username used for this service to register in glance
	ServiceUser string `json:"serviceUser"`

	// +kubebuilder:validation:Required
	// MariaDB instance name
	// Right now required by the maridb-operator to get the credentials from the instance to create the DB
	// Might not be required in future
	DatabaseInstance string `json:"databaseInstance,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default=placement
	// DatabaseUser - optional username used for placement DB, defaults to placement
	// TODO: -> implement needs work in mariadb-operator, right now only placement
	DatabaseUser string `json:"databaseUser"`

	// +kubebuilder:validation:Required
	// GlanceAPI Container Image URL
	ContainerImage string `json:"containerImage,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default=1
	// +kubebuilder:validation:Maximum=32
	// +kubebuilder:validation:Minimum=0
	// Replicas of placement API to run
	Replicas int32 `json:"replicas"`

	// +kubebuilder:validation:Required
	// Secret containing OpenStack password information for placement GlanceDatabasePassword, AdminPassword
	Secret string `json:"secret,omitempty"`

	// +kubebuilder:validation:Optional
	// PasswordSelectors - Selectors to identify the DB and AdminUser password from the Secret
	PasswordSelectors PasswordSelector `json:"passwordSelectors,omitempty"`

	// +kubebuilder:validation:Optional
	// NodeSelector to target subset of worker nodes running this service
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// +kubebuilder:validation:Optional
	// Debug - enable debug for different deploy stages. If an init container is used, it runs and the
	// actual action pod gets started with sleep infinity
	Debug GlanceAPIDebug `json:"debug,omitempty"`

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
	// Resources - Compute Resources required by this service (Limits/Requests).
	// https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// +kubebuilder:validation:Optional
	// StorageClass
	StorageClass string `json:"storageClass,omitempty"`

	// +kubebuilder:validation:Optional
	// StorageRequest
	StorageRequest string `json:"storageRequest,omitempty"`
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

// GlanceAPIDebug defines the observed state of GlanceAPIDebug
type GlanceAPIDebug struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=false
	// DBSync enable debug
	DBSync bool `json:"dbSync,omitempty"`
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=false
	// Bootstrap enable debug
	Bootstrap bool `json:"bootstrap,omitempty"`
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=false
	// Service enable debug
	Service bool `json:"service,omitempty"`
}

// GlanceAPIStatus defines the observed state of GlanceAPI
type GlanceAPIStatus struct {
	// ReadyCount of glance API instances
	ReadyCount int32 `json:"readyCount,omitempty"`

	// Map of hashes to track e.g. job status
	Hash map[string]string `json:"hash,omitempty"`

	// API endpoint
	APIEndpoints map[string]string `json:"apiEndpoint,omitempty"`

	// Conditions
	Conditions condition.List `json:"conditions,omitempty" optional:"true"`

	// Glance Database Hostname
	DatabaseHostname string `json:"databaseHostname,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// GlanceAPI is the Schema for the glanceapis API
type GlanceAPI struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GlanceAPISpec   `json:"spec,omitempty"`
	Status GlanceAPIStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// GlanceAPIList contains a list of GlanceAPI
type GlanceAPIList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GlanceAPI `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GlanceAPI{}, &GlanceAPIList{})
}

// GetEndpoint - returns OpenStack endpoint url for type
func (instance GlanceAPI) GetEndpoint(endpointType common.Endpoint) (string, error) {
	if url, found := instance.Status.APIEndpoints[string(endpointType)]; found {
		return url, nil
	}
	return "", fmt.Errorf("%s endpoint not found", string(endpointType))
}

// IsReady - returns true if service is ready to server requests
func (instance GlanceAPI) IsReady() bool {
	return instance.Status.ReadyCount >= 1
}
