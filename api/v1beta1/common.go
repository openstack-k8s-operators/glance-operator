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
	"context"
	"time"

	"github.com/gophercloud/gophercloud"
	keystonev1 "github.com/openstack-k8s-operators/keystone-operator/api/v1beta1"
	"github.com/openstack-k8s-operators/lib-common/modules/common/endpoint"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	"github.com/openstack-k8s-operators/lib-common/modules/common/secret"
	"github.com/openstack-k8s-operators/lib-common/modules/common/service"
	"github.com/openstack-k8s-operators/lib-common/modules/common/util"
	openstack "github.com/openstack-k8s-operators/lib-common/modules/openstack"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	// Container image fall-back defaults

	// GlanceAPIContainerImage is the fall-back container image for GlanceAPI
	GlanceAPIContainerImage = "quay.io/podified-antelope-centos9/openstack-glance-api:current-podified"
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
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// +kubebuilder:validation:Optional
	// Debug - enable debug for different deploy stages. If an init container is used, it runs and the
	// actual action pod gets started with sleep infinity
	Debug GlanceAPIDebug `json:"debug,omitempty"`

	// +kubebuilder:validation:Optional
	// Pvc - Storage claim for file-backed Glance
	Pvc string `json:"pvc,omitempty"`

	// +kubebuilder:validation:Optional
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

	// +kubebuilder:validation:Optional
	// StorageClass
	StorageClass string `json:"storageClass,omitempty"`

	// StorageRequest
	StorageRequest string `json:"storageRequest"`

	// +kubebuilder:validation:Enum=split;single
	// +kubebuilder:default:=split
	Type string `json:"type,omitempty"`
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

// GetAdminServiceClient - get an admin serviceClient for the Glance instance
func GetAdminServiceClient(
	ctx context.Context,
	h *helper.Helper,
	keystoneAPI *keystonev1.KeystoneAPI,
) (*openstack.OpenStack, ctrl.Result, error) {
	// get public endpoint as authurl from keystone instance
	authURL, err := keystoneAPI.GetEndpoint(endpoint.EndpointPublic)
	if err != nil {
		return nil, ctrl.Result{}, err
	}

	// get the password of the admin user from Spec.Secret
	// using PasswordSelectors.Admin
	authPassword, ctrlResult, err := secret.GetDataFromSecret(
		ctx,
		h,
		keystoneAPI.Spec.Secret,
		time.Duration(10)*time.Second,
		keystoneAPI.Spec.PasswordSelectors.Admin)
	if err != nil {
		return nil, ctrl.Result{}, err
	}
	if (ctrlResult != ctrl.Result{}) {
		return nil, ctrlResult, nil
	}

	os, err := openstack.NewOpenStack(
		h.GetLogger(),
		openstack.AuthOpts{
			AuthURL:    authURL,
			Username:   keystoneAPI.Spec.AdminUser,
			Password:   authPassword,
			TenantName: keystoneAPI.Spec.AdminProject,
			DomainName: "Default",
			Region:     keystoneAPI.Spec.Region,
			Scope: &gophercloud.AuthScope{
				System: true,
			},
		})
	if err != nil {
		return nil, ctrl.Result{}, err
	}

	return os, ctrl.Result{}, nil
}
