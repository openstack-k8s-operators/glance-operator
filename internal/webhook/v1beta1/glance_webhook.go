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
var glancelog = logf.Log.WithName("glance-resource")

// Static errors for err113 compliance
var (
	errUnexpectedGlanceObjectType = errors.New("unexpected Glance object type")
)

// SetupGlanceWebhookWithManager registers the webhook for Glance in the manager.
func SetupGlanceWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&glancev1beta1.Glance{}).
		WithValidator(&GlanceCustomValidator{}).
		WithDefaulter(&GlanceCustomDefaulter{}).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-glance-openstack-org-v1beta1-glance,mutating=true,failurePolicy=fail,sideEffects=None,groups=glance.openstack.org,resources=glances,verbs=create;update,versions=v1beta1,name=mglance-v1beta1.kb.io,admissionReviewVersions=v1

// GlanceCustomDefaulter struct is responsible for setting default values on the custom resource of the
// Kind Glance when those are created or updated.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as it is used only for temporary operations and does not need to be deeply copied.
type GlanceCustomDefaulter struct{}

var _ webhook.CustomDefaulter = &GlanceCustomDefaulter{}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the Kind Glance.
func (d *GlanceCustomDefaulter) Default(_ context.Context, obj runtime.Object) error {
	glance, ok := obj.(*glancev1beta1.Glance)

	if !ok {
		return fmt.Errorf("%w: got %T", errUnexpectedGlanceObjectType, obj)
	}
	glancelog.Info("Defaulting for Glance", "name", glance.GetName())

	glance.Default()
	return nil
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// NOTE: The 'path' attribute must follow a specific pattern and should not be modified directly here.
// Modifying the path for an invalid path can cause API server errors; failing to locate the webhook.
// +kubebuilder:webhook:path=/validate-glance-openstack-org-v1beta1-glance,mutating=false,failurePolicy=fail,sideEffects=None,groups=glance.openstack.org,resources=glances,verbs=create;update,versions=v1beta1,name=vglance-v1beta1.kb.io,admissionReviewVersions=v1

// GlanceCustomValidator struct is responsible for validating the Glance resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
type GlanceCustomValidator struct {
	// TODO(user): Add more fields as needed for validation
}

var _ webhook.CustomValidator = &GlanceCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type Glance.
func (v *GlanceCustomValidator) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	glance, ok := obj.(*glancev1beta1.Glance)
	if !ok {
		return nil, fmt.Errorf("%w: got %T", errUnexpectedGlanceObjectType, obj)
	}
	glancelog.Info("Validation for Glance upon creation", "name", glance.GetName())

	return glance.ValidateCreate()

}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type Glance.
func (v *GlanceCustomValidator) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	glance, ok := newObj.(*glancev1beta1.Glance)
	if !ok {
		return nil, fmt.Errorf("%w: got %T", errUnexpectedGlanceObjectType, newObj)
	}
	glancelog.Info("Validation for Glance upon update", "name", glance.GetName())

	return glance.ValidateUpdate(oldObj)

}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type Glance.
func (v *GlanceCustomValidator) ValidateDelete(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	glance, ok := obj.(*glancev1beta1.Glance)
	if !ok {
		return nil, fmt.Errorf("%w: got %T", errUnexpectedGlanceObjectType, obj)
	}
	glancelog.Info("Validation for Glance upon deletion", "name", glance.GetName())

	return glance.ValidateDelete()
}
