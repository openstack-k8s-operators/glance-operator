/*
Copyright 2025.

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

// Package v1beta1 contains webhook implementations for the v1beta1 API version.
package v1beta1

import (
	"context"
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	glancev1beta1 "github.com/openstack-k8s-operators/glance-operator/api/v1beta1"
)

// nolint:unused
// log is for logging in this package.
var glanceapilog = logf.Log.WithName("glanceapi-resource")

// Static errors for err113 compliance
var (
	errUnexpectedGlanceAPIObjectType = errors.New("unexpected GlanceAPI object type")
)

// SetupGlanceAPIWebhookWithManager registers the webhook for GlanceAPI in the manager.
func SetupGlanceAPIWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&glancev1beta1.GlanceAPI{}).
		WithValidator(&GlanceAPICustomValidator{}).
		WithDefaulter(&GlanceAPICustomDefaulter{}).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-glance-openstack-org-v1beta1-glanceapi,mutating=true,failurePolicy=fail,sideEffects=None,groups=glance.openstack.org,resources=glanceapis,verbs=create;update,versions=v1beta1,name=mglanceapi-v1beta1.kb.io,admissionReviewVersions=v1

// GlanceAPICustomDefaulter struct is responsible for setting default values on the custom resource of the
// Kind GlanceAPI when those are created or updated.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as it is used only for temporary operations and does not need to be deeply copied.
type GlanceAPICustomDefaulter struct{}

var _ webhook.CustomDefaulter = &GlanceAPICustomDefaulter{}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the Kind GlanceAPI.
func (d *GlanceAPICustomDefaulter) Default(_ context.Context, obj runtime.Object) error {
	glanceapi, ok := obj.(*glancev1beta1.GlanceAPI)

	if !ok {
		return fmt.Errorf("%w: got %T", errUnexpectedGlanceAPIObjectType, obj)
	}
	glanceapilog.Info("Defaulting for GlanceAPI", "name", glanceapi.GetName())

	glanceapi.Default()
	return nil
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// NOTE: The 'path' attribute must follow a specific pattern and should not be modified directly here.
// Modifying the path for an invalid path can cause API server errors; failing to locate the webhook.
// +kubebuilder:webhook:path=/validate-glance-openstack-org-v1beta1-glanceapi,mutating=false,failurePolicy=fail,sideEffects=None,groups=glance.openstack.org,resources=glanceapis,verbs=create;update,versions=v1beta1,name=vglanceapi-v1beta1.kb.io,admissionReviewVersions=v1

// GlanceAPICustomValidator struct is responsible for validating the GlanceAPI resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
type GlanceAPICustomValidator struct {
	// TODO(user): Add more fields as needed for validation
}

var _ webhook.CustomValidator = &GlanceAPICustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type GlanceAPI.
func (v *GlanceAPICustomValidator) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	glanceapi, ok := obj.(*glancev1beta1.GlanceAPI)
	if !ok {
		return nil, fmt.Errorf("%w: got %T", errUnexpectedGlanceAPIObjectType, obj)
	}
	glanceapilog.Info("Validation for GlanceAPI upon creation", "name", glanceapi.GetName())

	return glanceapi.ValidateCreate()
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type GlanceAPI.
func (v *GlanceAPICustomValidator) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	glanceapi, ok := newObj.(*glancev1beta1.GlanceAPI)
	if !ok {
		return nil, fmt.Errorf("%w: got %T", errUnexpectedGlanceAPIObjectType, newObj)
	}
	glanceapilog.Info("Validation for GlanceAPI upon update", "name", glanceapi.GetName())

	return glanceapi.ValidateUpdate(oldObj)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type GlanceAPI.
func (v *GlanceAPICustomValidator) ValidateDelete(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	glanceapi, ok := obj.(*glancev1beta1.GlanceAPI)
	if !ok {
		return nil, fmt.Errorf("%w: got %T", errUnexpectedGlanceAPIObjectType, obj)
	}
	glanceapilog.Info("Validation for GlanceAPI upon deletion", "name", glanceapi.GetName())

	return glanceapi.ValidateDelete()
}
