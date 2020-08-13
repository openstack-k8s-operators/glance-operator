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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// GlanceAPISpec defines the desired state of GlanceAPI
type GlanceAPISpec struct {
	// Glance Database Hostname String
	DatabaseHostname string `json:"databaseHostname,omitempty"`
	// Glance Container Image URL
	ContainerImage string `json:"containerImage,omitempty"`
	// Replicas
	Replicas int32 `json:"replicas"`
	// StorageClass
	StorageClass string `json:"storageClass,omitempty"`
	// StorageRequest
	StorageRequest string `json:"storageRequest,omitempty"`
	// Secret containing: GlancePassword, TransportURL
	Secret string `json:"secret,omitempty"`
}

// GlanceAPIStatus defines the observed state of GlanceAPI
type GlanceAPIStatus struct {
	// DbSyncHash db sync hash
	DbSyncHash string `json:"dbSyncHash"`
	// DeploymentHash deployment hash
	DeploymentHash string `json:"deploymentHash"`
	// API endpoint
	APIEndpoint string `json:"apiEndpoint"`
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
