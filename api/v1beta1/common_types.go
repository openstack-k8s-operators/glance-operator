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
	"strings"

	"github.com/openstack-k8s-operators/lib-common/modules/common/service"
	"github.com/openstack-k8s-operators/lib-common/modules/common/tls"
	"github.com/openstack-k8s-operators/lib-common/modules/common/util"
	corev1 "k8s.io/api/core/v1"
)

const (
	// Container image fall-back defaults

	// GlanceAPIContainerImage is the fall-back container image for GlanceAPI
	GlanceAPIContainerImage = "quay.io/podified-antelope-centos9/openstack-glance-api:current-podified"
	//DBPurgeDefaultAge indicates the number of days of purging DB records
	DBPurgeDefaultAge = 30
	//DBPurgeDefaultSchedule is in crontab format, and the default runs the job once every day
	DBPurgeDefaultSchedule = "1 0 * * *"
	//CleanerDefaultSchedule is in crontab format, and the default runs the job once every 30 minutes
	CleanerDefaultSchedule = "*/30 * * * *"
	//PrunerDefaultSchedule is in crontab format, and the default runs the job once every day
	PrunerDefaultSchedule = "1 0 * * *"
	// APIDefaultTimeout indicates the default APITimeout for HAProxy and Apache, defaults to 60 seconds
	APIDefaultTimeout = 60
)

// GlanceAPITemplate defines the desired state of GlanceAPI
type GlanceAPITemplate struct {

	// +kubebuilder:validation:Optional
	// +kubebuilder:default=1
	// +kubebuilder:validation:Maximum=32
	// +kubebuilder:validation:Minimum=0
	// Replicas of glance API to run
	Replicas *int32 `json:"replicas"`

	// +kubebuilder:validation:Required
	// Glance Container Image URL (will be set to environmental default if empty)
	ContainerImage string `json:"containerImage"`

	// +kubebuilder:validation:Optional
	// NodeSelector to target subset of worker nodes running this service
	NodeSelector *map[string]string `json:"nodeSelector,omitempty"`

	// +kubebuilder:validation:Optional
	// Topology to apply the Policy defined by the associated CR referenced by
	// name
	Topology *TopologyRef `json:"topologyRef,omitempty"`

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
	// Resources - Compute Resources required by this service (Limits/Requests).
	// https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// +kubebuilder:validation:Optional
	// NetworkAttachments is a list of NetworkAttachment resource names to expose the services to the given network
	NetworkAttachments []string `json:"networkAttachments,omitempty"`

	// Override, provides the ability to override the generated manifest of several child resources.
	Override APIOverrideSpec `json:"override,omitempty"`

	// Storage -
	Storage Storage `json:"storage,omitempty"`

	// +kubebuilder:validation:Enum=split;single;edge
	// +kubebuilder:default:=split
	// Type - represents the layout of the glanceAPI deployment.
	Type string `json:"type,omitempty"`

	// +kubebuilder:validation:Optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// TLS - Parameters related to the TLS
	TLS tls.API `json:"tls,omitempty"`

	// ImageCache - It represents the struct to expose the ImageCache related
	// parameters (size of the PVC and cronJob schedule)
	// +kubebuilder:validation:Optional
	ImageCache ImageCache `json:"imageCache,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Minimum=1
	// APITimeout for HAProxy and Apache defaults to GlanceSpecCore APITimeout
	APITimeout int `json:"apiTimeout,omitempty"`
}

// TopologyRef -
type TopologyRef struct {
	// +kubebuilder:validation:Optional
	// Name - The Topology CR name that Glance references
	Name string `json:"name"`

	// +kubebuilder:validation:Optional
	// Namespace - The Namespace to fetch the Topology CR referenced
	// NOTE: Namespace currently points by default to the same namespace where
	// Glance is deployed. Customizing the namespace is not supported and
	// webhooks prevent editing this field to a value different from the
	// current project
	Namespace string `json:"namespace,omitempty"`
}

// Storage -
type Storage struct {
	// +kubebuilder:validation:Optional
	// StorageClass -
	StorageClass string `json:"storageClass,omitempty"`

	// StorageRequest -
	StorageRequest string `json:"storageRequest,omitempty"`

	// +kubebuilder:validation:Optional
	// External -
	External bool `json:"external,omitempty"`
}

// ImageCache - struct where the exposed imageCache params are defined
type ImageCache struct {
	// Size - Local storage request, in bytes. (500Gi = 500GiB = 500 * 1024 * 1024 * 1024)
	// +kubebuilder:default=""
	Size string `json:"size"`
	// +kubebuilder:validation:Optional
	// +kubebuilder:default="*/30 * * * *"
	// Schedule defines the crontab format string to schedule the Cleaner cronJob
	CleanerScheduler string `json:"cleanerScheduler"`
	// +kubebuilder:validation:Optional
	// +kubebuilder:default="1 0 * * *"
	//Schedule defines the crontab format string to schedule the Pruner cronJob
	PrunerScheduler string `json:"prunerScheduler"`
}

// APIOverrideSpec to override the generated manifest of several child resources.
type APIOverrideSpec struct {
	// Override configuration for the Service created to serve traffic to the cluster.
	// The key must be the endpoint type (public, internal)
	Service map[service.Endpoint]service.RoutedOverrideSpec `json:"service,omitempty"`
}

// SetupDefaults - initializes any CRD field defaults based on environment variables (the defaulting mechanism itself is implemented via webhooks)
func SetupDefaults() {
	// Acquire environmental defaults and initialize Glance defaults with them
	glanceDefaults := GlanceDefaults{
		ContainerImageURL: util.GetEnvVar("RELATED_IMAGE_GLANCE_API_IMAGE_URL_DEFAULT", GlanceAPIContainerImage),
		DBPurgeAge:        DBPurgeDefaultAge,
		DBPurgeSchedule:   DBPurgeDefaultSchedule,
		CleanerSchedule:   CleanerDefaultSchedule,
		PrunerSchedule:    PrunerDefaultSchedule,
		APITimeout:        APIDefaultTimeout,
	}

	SetupGlanceDefaults(glanceDefaults)
}

// SetupAPIDefaults - initializes any CRD field defaults based on environment variables (the defaulting mechanism itself is implemented via webhooks)
func SetupAPIDefaults() {
	// Acquire environmental defaults and initialize GlanceAPI defaults with them
	glanceAPIDefaults := GlanceAPIDefaults{
		ContainerImageURL: util.GetEnvVar("RELATED_IMAGE_GLANCE_API_IMAGE_URL_DEFAULT", GlanceAPIContainerImage),
	}

	SetupGlanceAPIDefaults(glanceAPIDefaults)
}

// GetEnabledBackends - Given a instance.Spec.CustomServiceConfig object, return
// a list of available stores in form of 'store_id':'backend'
func GetEnabledBackends(customServiceConfig string) []string {
	var availableBackends []string
	svcConfigLines := strings.Split(customServiceConfig, "\n")
	for _, line := range svcConfigLines {
		tokenLine := strings.SplitN(strings.TrimSpace(line), "=", 2)
		token := strings.ReplaceAll(tokenLine[0], " ", "")

		if token == "" || strings.HasPrefix(token, "#") {
			// Skip blank lines and comments
			continue
		}
		if token == "enabled_backends" {
			backendToken := strings.SplitN(strings.TrimSpace(tokenLine[1]), ",", -1)
			for i := 0; i < len(backendToken); i++ {
				availableBackends = append(availableBackends, strings.TrimSpace(backendToken[i]))
			}
			break
		}
	}
	return availableBackends
}
