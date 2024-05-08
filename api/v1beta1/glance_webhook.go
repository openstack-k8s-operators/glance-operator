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
	"strings"

	"github.com/google/go-cmp/cmp"
	"github.com/openstack-k8s-operators/lib-common/modules/common/service"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// GlanceDefaults -
type GlanceDefaults struct {
	ContainerImageURL string
	DBPurgeAge        int
	DBPurgeSchedule   string
	CleanerSchedule   string
	PrunerSchedule    string
}

var glanceDefaults GlanceDefaults

// log is for logging in this package.
var glancelog = logf.Log.WithName("glance-resource")

// SetupGlanceDefaults - initialize Glance spec defaults for use with either internal or external webhooks
func SetupGlanceDefaults(defaults GlanceDefaults) {
	glanceDefaults = defaults
	glancelog.Info("Glance defaults initialized", "defaults", defaults)
}

// SetupWebhookWithManager sets up the webhook with the Manager
func (r *Glance) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-glance-openstack-org-v1beta1-glance,mutating=true,failurePolicy=fail,sideEffects=None,groups=glance.openstack.org,resources=glances,verbs=create;update,versions=v1beta1,name=mglance.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &Glance{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *Glance) Default() {
	glancelog.Info("default", "name", r.Name)

	if len(r.Spec.ContainerImage) == 0 {
		r.Spec.ContainerImage = glanceDefaults.ContainerImageURL
	}
	r.Spec.GlanceSpecCore.Default()
}

// Check if the KeystoneEndpoint matches with a deployed glanceAPI
func (r *GlanceSpecCore) isValidKeystoneEP() bool {
	for name, api := range r.GlanceAPIs {
		// A valid keystoneEndpoint can either be applied to
		// a single API or split type, but not to an EdgeAPI
		if api.Type != APIEdge && r.KeystoneEndpoint == name {
			return true
		}
	}
	return false
}

// GetTemplateBackend -
func GetTemplateBackend() string {
	section := "[DEFAULT]"
	dummyBackend := "enabled_backends=backend1:type1 # CHANGE_ME"
	return fmt.Sprintf("%s\n%s", section, dummyBackend)
}

// Default - set defaults for this Glance spec
func (r *GlanceSpecCore) Default() {
	var rep int32 = 0

	if r.DBPurge.Age == 0 {
		r.DBPurge.Age = glanceDefaults.DBPurgeAge
	}

	if r.DBPurge.Schedule == "" {
		r.DBPurge.Schedule = glanceDefaults.DBPurgeSchedule
	}
	// When no glanceAPI(s) are specified in the top-level CR
	// we build one by default, but we set replicas=0 and we
	// build a "CustomServiceConfig" template that should be
	// customized: by doing this we force to provide the
	// required parameters
	if r.GlanceAPIs == nil || len(r.GlanceAPIs) == 0 {
		// keystoneEndpoint will match with the only instance
		// deployed by default
		r.KeystoneEndpoint = "default"
		r.CustomServiceConfig = GetTemplateBackend()
		r.GlanceAPIs = map[string]GlanceAPITemplate{
			"default": {
				Replicas: &rep,
			},
		}
	}
	for key, glanceAPI := range r.GlanceAPIs {
		// Check the sub-cr ContainerImage parameter
		if glanceAPI.ContainerImage == "" {
			glanceAPI.ContainerImage = glanceDefaults.ContainerImageURL
			r.GlanceAPIs[key] = glanceAPI
		}
		if glanceAPI.ImageCache.CleanerScheduler == "" {
			glanceAPI.ImageCache.CleanerScheduler = glanceDefaults.CleanerSchedule
			r.GlanceAPIs[key] = glanceAPI
		}
		if glanceAPI.ImageCache.PrunerScheduler == "" {
			glanceAPI.ImageCache.PrunerScheduler = glanceDefaults.PrunerSchedule
			r.GlanceAPIs[key] = glanceAPI
		}
	}
	// In the special case where the GlanceAPI list is composed by a single
	// element, we can omit the "KeystoneEndpoint" spec parameter and default
	// it to that only instance present in the main CR
	if r.KeystoneEndpoint == "" && len(r.GlanceAPIs) == 1 {
		for k := range r.GlanceAPIs {
			r.KeystoneEndpoint = k
			break
		}
	}
}

//+kubebuilder:webhook:path=/validate-glance-openstack-org-v1beta1-glance,mutating=false,failurePolicy=fail,sideEffects=None,groups=glance.openstack.org,resources=glances,verbs=create;update,versions=v1beta1,name=vglance.kb.io,admissionReviewVersions=v1

// Check if File is used as a backend for Glance
func isFileBackend(customServiceConfig string, topLevel bool) bool {
	availableBackends := GetEnabledBackends(customServiceConfig)
	// if we have "enabled_backends=backend1:type1,backend2:type2 ..
	// we need to iterate over this list and look for type=file
	for i := 0; i < len(availableBackends); i++ {
		backendToken := strings.SplitN(availableBackends[i], ":", 2)
		if backendToken[1] == "file" {
			return true
		}
	}
	// If the iteration over the list has not produced file, we have yet another
	// possible scenario to evaluate:
	// - availableBackends is []
	// - the topLevel CR is [] or has File has backend (topLevel is true)
	if len(availableBackends) == 0 && topLevel {
		return true
	}
	return false
}

// Check if the File is used in combination with a wrong layout
func (r *GlanceSpecCore) isInvalidBackend(glanceAPI GlanceAPITemplate, topLevel bool) bool {
	var rep int32 = 0
	// For each current glanceAPI instance, detect an invalid configuration
	// made by "type: split && backend: file": raise an issue if this config
	// is found. However, do not fail if 'replica: 0' because it means the
	// operator has not made any choice about the backend yet
	if *glanceAPI.Replicas != rep && glanceAPI.Type == "split" && isFileBackend(glanceAPI.CustomServiceConfig, topLevel) {
		return true
	}
	return false
}

var _ webhook.Validator = &Glance{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *Glance) ValidateCreate() (admission.Warnings, error) {
	glancelog.Info("validate create", "name", r.Name)
	var allErrs field.ErrorList
	basePath := field.NewPath("spec")
	if err := r.Spec.ValidateCreate(basePath); err != nil {
		allErrs = append(allErrs, err...)
	}

	if len(allErrs) != 0 {
		return nil, apierrors.NewInvalid(
			schema.GroupKind{Group: "glance.openstack.org", Kind: "Glance"},
			r.Name, allErrs)
	}

	return nil, nil
}

// ValidateCreate - Exported function wrapping non-exported validate functions,
// this function can be called externally to validate an ironic spec.
func (r *GlanceSpec) ValidateCreate(basePath *field.Path) field.ErrorList {
	return r.GlanceSpecCore.ValidateCreate(basePath)
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *GlanceSpecCore) ValidateCreate(basePath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	// Check if the top-level CR has a "customServiceConfig" with an explicit
	// "backend:file || empty string" and save the result into topLevel var.
	// If it's empty it should be ignored and having a file backend depends
	// only on the sub-cr.
	// if it has an explicit "backend:file", then the top-level "customServiceConfig"
	// should play a role in the backedn evaluation. To save the result of
	// top-level using the same function, "true" as the second parameter, as it
	// represents an invariant for the top-level CR.
	topLevelFileBackend := isFileBackend(r.CustomServiceConfig, true)

	// For each Glance backend
	for key, glanceAPI := range r.GlanceAPIs {
		path := basePath.Child("glanceAPIs").Key(key)

		// fail if an invalid configuration/layout is detected
		if r.isInvalidBackend(glanceAPI, topLevelFileBackend) {
			allErrs = append(allErrs, field.Invalid(
				path, key, "Invalid backend configuration detected"))
		}

		// validate the service override key is valid
		allErrs = append(allErrs, service.ValidateRoutedOverrides(
			path.Child("override").Child("service"),
			glanceAPI.Override.Service)...)
	}

	// At creation time, if the CR has an invalid keystoneEndpoint value (that
	// doesn't match with any defined backend), return an error.
	if !r.isValidKeystoneEP() {
		path := basePath.Child("keystoneEndpoint")
		allErrs = append(allErrs, field.Invalid(
			path, r.KeystoneEndpoint, "KeystoneEndpoint is assigned to an invalid glanceAPI instance"))
	}

	return allErrs
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *Glance) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	o, ok := old.(*Glance)
	if !ok || o == nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("unable to convert existing object"))
	}
	glancelog.Info("validate update", "diff", cmp.Diff(old, r))

	var allErrs field.ErrorList
	basePath := field.NewPath("spec")

	if err := r.Spec.ValidateUpdate(o.Spec, basePath); err != nil {
		allErrs = append(allErrs, err...)
	}

	if len(allErrs) != 0 {
		return nil, apierrors.NewInvalid(
			schema.GroupKind{Group: "glance.openstack.org", Kind: "Glance"},
			r.Name, allErrs)
	}

	return nil, nil
}

// ValidateUpdate - Exported function wrapping non-exported validate functions,
// this function can be called externally to validate an glance spec.
func (r *GlanceSpec) ValidateUpdate(old GlanceSpec, basePath *field.Path) field.ErrorList {
	return r.GlanceSpecCore.ValidateUpdate(old.GlanceSpecCore, basePath)
}

func (r *GlanceSpecCore) ValidateUpdate(old GlanceSpecCore, basePath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	// Type can either be "split" or "single": we do not support changing layout
	// because there's no logic in the operator to scale down the existing statefulset
	// and scale up the new one, hence updating the Spec.GlanceAPI.Type is not supported
	topLevelFileBackend := isFileBackend(r.CustomServiceConfig, true)
	for key, glanceAPI := range r.GlanceAPIs {
		path := basePath.Child("glanceAPIs").Key(key)

		// When a new entry (new glanceAPI instance) is added in the main CR, it's
		// possible that the old CR used to compare the new map had no entry with
		// the same name. This represent a valid use case and we shouldn't prevent
		// to grow the deployment
		if _, found := old.GlanceAPIs[key]; !found {
			continue
		}
		// The current glanceAPI exists and the layout is different
		if glanceAPI.Type != old.GlanceAPIs[key].Type {
			allErrs = append(allErrs, field.Invalid(path, key, "GlanceAPI deployment layout can't be updated"))
		}
		// Fail if an invalid configuration/layout is detected for the current
		// glanceAPI instance
		if r.isInvalidBackend(glanceAPI, topLevelFileBackend) {
			allErrs = append(allErrs, field.Invalid(path, key, "Invalid backend configuration detected"))
		}
		// validate the service override key is valid
		allErrs = append(allErrs, service.ValidateRoutedOverrides(
			path.Child("override").Child("service"),
			glanceAPI.Override.Service)...)
	}

	// At update time, if the CR has an invalid keystoneEndpoint set
	// (e.g. an Edge GlanceAPI instance that can't be registered in keystone)
	// return an error message
	if !r.isValidKeystoneEP() {
		path := basePath.Child("keystoneEndpoint")
		allErrs = append(allErrs, field.Invalid(
			path, r.KeystoneEndpoint, "KeystoneEndpoint is assigned to an invalid glanceAPI instance"))
	}

	return allErrs
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *Glance) ValidateDelete() (admission.Warnings, error) {
	glancelog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil, nil
}
