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

	"github.com/google/go-cmp/cmp"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// GlanceAPIDefaults -
type GlanceAPIDefaults struct {
	ContainerImageURL string
}

var glanceAPIDefaults GlanceAPIDefaults

// log is for logging in this package.
var glanceapilog = logf.Log.WithName("glanceapi-resource")

// SetupGlanceAPIDefaults - initialize GlanceAPI spec defaults for use with either internal or external webhooks
func SetupGlanceAPIDefaults(defaults GlanceAPIDefaults) {
	glanceAPIDefaults = defaults
	glanceapilog.Info("Glance defaults initialized", "defaults", defaults)
}

var _ webhook.Defaulter = &GlanceAPI{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *GlanceAPI) Default() {
	glanceapilog.Info("default", "name", r.Name)

	r.Spec.Default()
}

// Default - set defaults for this Glance spec
func (spec *GlanceAPISpec) Default() {
	if spec.ContainerImage == "" {
		spec.ContainerImage = glanceAPIDefaults.ContainerImageURL
	}
}

var _ webhook.Validator = &GlanceAPI{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *GlanceAPI) ValidateCreate() (admission.Warnings, error) {
	glanceapilog.Info("validate create", "name", r.Name)

	// TODO(user): fill in your validation logic upon object creation.
	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *GlanceAPI) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	glanceapilog.Info("validate update", "name", r.Name)

	o, ok := old.(*GlanceAPI)
	if !ok || o == nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("unable to convert existing object"))
	}

	glanceapilog.Info("validate update", "diff", cmp.Diff(o, r))

	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *GlanceAPI) ValidateDelete() (admission.Warnings, error) {
	glanceapilog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil, nil
}
