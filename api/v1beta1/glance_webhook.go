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
	"errors"
	"fmt"
	"strings"
	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// GlanceDefaults -
type GlanceDefaults struct {
	ContainerImageURL string
	DBPurgeAge        int
	DBPurgeSchedule   string
	CleanerSchedule   string
	PrunerSchedule   string
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
func (spec *GlanceSpec) isValidKeystoneEP() bool {
	for name, api := range spec.GlanceAPIs {
		// A valid keystoneEndpoint can either be applied to
		// a single API or split type, but not to an EdgeAPI
		if api.Type != APIEdge && spec.KeystoneEndpoint == name {
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
func (spec *GlanceSpecCore) Default() {
	var rep int32 = 0

	if spec.DBPurge.Age == 0 {
		spec.DBPurge.Age = glanceDefaults.DBPurgeAge
	}

	if spec.DBPurge.Schedule == "" {
		spec.DBPurge.Schedule = glanceDefaults.DBPurgeSchedule
	}
	// When no glanceAPI(s) are specified in the top-level CR
	// we build one by default, but we set replicas=0 and we
	// build a "CustomServiceConfig" template that should be
	// customized: by doing this we force to provide the
	// required parameters
	if spec.GlanceAPIs == nil || len(spec.GlanceAPIs) == 0 {
		// keystoneEndpoint will match with the only instance
		// deployed by default
		spec.KeystoneEndpoint = "default"
		spec.CustomServiceConfig = GetTemplateBackend()
		spec.GlanceAPIs = map[string]GlanceAPITemplate{
			"default": {
				Replicas: &rep,
			},
		}
	}
	for key, glanceAPI := range spec.GlanceAPIs {
		// Check the sub-cr ContainerImage parameter
		if glanceAPI.ContainerImage == "" {
			glanceAPI.ContainerImage = glanceDefaults.ContainerImageURL
			spec.GlanceAPIs[key] = glanceAPI
		}
		if glanceAPI.ImageCache.CleanerScheduler == "" {
			glanceAPI.ImageCache.CleanerScheduler = glanceDefaults.CleanerSchedule
			spec.GlanceAPIs[key] = glanceAPI
		}
		if glanceAPI.ImageCache.PrunerScheduler == "" {
			glanceAPI.ImageCache.PrunerScheduler = glanceDefaults.PrunerSchedule
			spec.GlanceAPIs[key] = glanceAPI
		}
	}
	// In the special case where the GlanceAPI list is composed by a single
	// element, we can omit the "KeystoneEndpoint" spec parameter and default
	// it to that only instance present in the main CR
	if spec.KeystoneEndpoint == "" && len(spec.GlanceAPIs) == 1 {
		for k := range spec.GlanceAPIs {
			spec.KeystoneEndpoint = k
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
func (r *Glance) isInvalidBackend(glanceAPI GlanceAPITemplate, topLevel bool) bool {
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
	// Check if the top-level CR has a "customServiceConfig" with an explicit
	// "backend:file || empty string" and save the result into topLevel var.
	// If it's empty it should be ignored and having a file backend depends
	// only on the sub-cr.
	// if it has an explicit "backend:file", then the top-level "customServiceConfig"
	// should play a role in the backedn evaluation. To save the result of
	// top-level using the same function, "true" as the second parameter, as it
	// represents an invariant for the top-level CR.
	topLevelFileBackend := isFileBackend(r.Spec.CustomServiceConfig, true)
	// For each Glance backend, fail if an invalid configuration/layout is
	// detected
	for _, glanceAPI := range r.Spec.GlanceAPIs {
		if r.isInvalidBackend(glanceAPI, topLevelFileBackend) {
			return nil, errors.New("Invalid backend configuration detected")
		}
	}
	// At creation time, if the CR has an invalid keystoneEndpoint value (that
	// doesn't match with any defined backend), return an error.
	if !r.Spec.isValidKeystoneEP() {
		return nil, errors.New("KeystoneEndpoint is assigned to an invalid glanceAPI instance")
	}
	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *Glance) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {

	// Type can either be "split" or "single": we do not support changing layout
	// because there's no logic in the operator to scale down the existing statefulset
	// and scale up the new one, hence updating the Spec.GlanceAPI.Type is not supported
	o, ok := old.(*Glance)
	if !ok || o == nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("unable to convert existing object"))
	}
	glancelog.Info("validate update", "diff", cmp.Diff(old, r))

	topLevelFileBackend := isFileBackend(r.Spec.CustomServiceConfig, true)
	for key, glanceAPI := range r.Spec.GlanceAPIs {
		// When a new entry (new glanceAPI instance) is added in the main CR, it's
		// possible that the old CR used to compare the new map had no entry with
		// the same name. This represent a valid use case and we shouldn't prevent
		// to grow the deployment
		if _, found := o.Spec.GlanceAPIs[key]; !found {
			continue
		}
		// The current glanceAPI exists and the layout is different
		if glanceAPI.Type != o.Spec.GlanceAPIs[key].Type {
			return nil, errors.New("GlanceAPI deployment layout can't be updated")
		}
		// Fail if an invalid configuration/layout is detected for the current
		// glanceAPI instance
		if r.isInvalidBackend(glanceAPI, topLevelFileBackend) {
			return nil, errors.New("Invalid backend configuration detected")
		}
	}
	// At update time, if the CR has an invalid keystoneEndpoint set
	// (e.g. an Edge GlanceAPI instance that can't be registered in keystone)
	// return an error message
	if !r.Spec.isValidKeystoneEP() {
		return nil, errors.New(
			"KeystoneEndpoint is assigned to an invalid glanceAPI instance")
	}
	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *Glance) ValidateDelete() (admission.Warnings, error) {
	glancelog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil, nil
}
