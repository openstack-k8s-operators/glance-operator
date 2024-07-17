/*
Copyright 2024.

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
package functional

import (
	"errors"

	. "github.com/onsi/ginkgo/v2" //revive:disable:dot-imports
	. "github.com/onsi/gomega"    //revive:disable:dot-imports
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ = Describe("Glance validation", func() {
	It("webhooks reject the request - invalid keystoneEndpoint", func() {
		// GlanceEmptySpec is used to provide a standard Glance CR where no
		// field is customized: we can inject our parameters to test webhooks
		spec := GetGlanceDefaultSpec()
		spec["keystoneEndpoint"] = "foo"
		raw := map[string]interface{}{
			"apiVersion": "glance.openstack.org/v1beta1",
			"kind":       "Glance",
			"metadata": map[string]interface{}{
				"name":      glanceTest.Instance.Name,
				"namespace": glanceTest.Instance.Namespace,
			},
			"spec": spec,
		}
		unstructuredObj := &unstructured.Unstructured{Object: raw}
		_, err := controllerutil.CreateOrPatch(
			ctx, k8sClient, unstructuredObj, func() error { return nil })

		Expect(err).Should(HaveOccurred())
		var statusError *k8s_errors.StatusError
		Expect(errors.As(err, &statusError)).To(BeTrue())
		Expect(statusError.ErrStatus.Message).To(
			ContainSubstring(
				"KeystoneEndpoint is assigned to an invalid glanceAPI instance"),
		)
	})

	It("webhooks reject the request - invalid backend", func() {
		spec := GetGlanceDefaultSpec()

		gapis := map[string]interface{}{
			"glanceAPIs": map[string]interface{}{
				"default": map[string]interface{}{
					"replicas": 1,
					"type":     "split",
				},
				"edge1": map[string]interface{}{
					"replicas": 1,
					"type":     "edge",
				},
				// Webhooks catch that a backend != File is set for an instance
				// that has type: single
				"api1": map[string]interface{}{
					"customServiceConfig": GetDummyBackend(),
					"replicas":            1,
					"type":                "single",
				},
			},
		}

		spec["keystoneEndpoint"] = "edge1"
		spec["glanceAPIs"] = gapis

		raw := map[string]interface{}{
			"apiVersion": "glance.openstack.org/v1beta1",
			"kind":       "Glance",
			"metadata": map[string]interface{}{
				"name":      glanceTest.Instance.Name,
				"namespace": glanceTest.Instance.Namespace,
			},
			"spec": spec,
		}
		unstructuredObj := &unstructured.Unstructured{Object: raw}
		_, err := controllerutil.CreateOrPatch(
			ctx, k8sClient, unstructuredObj, func() error { return nil })

		Expect(err).Should(HaveOccurred())
		var statusError *k8s_errors.StatusError
		Expect(errors.As(err, &statusError)).To(BeTrue())
		// Webhooks catch that no backend is set before even realize that an
		// invalid endpoint has been set
		Expect(statusError.ErrStatus.Message).To(
			ContainSubstring(
				"Invalid backend configuration detected"),
		)
	})

	It("webhooks reject the request - invalid instance", func() {
		spec := GetGlanceDefaultSpec()

		gapis := map[string]interface{}{
			"edge2": map[string]interface{}{
				"replicas": 1,
				"type":     "edge",
				// inject a valid Ceph backend
				"customServiceConfig": GetDummyBackend(),
			},
			"default": map[string]interface{}{
				"replicas": 1,
				"type":     "split",
				// inject a valid Ceph backend
				"customServiceConfig": GetDummyBackend(),
			},
			"edge1": map[string]interface{}{
				"replicas": 1,
				"type":     "edge",
				// inject a valid Ceph backend
				"customServiceConfig": GetDummyBackend(),
			},
		}
		// Set the KeystoneEndpoint to the wrong instance
		spec["keystoneEndpoint"] = "edge1"
		// Deploy multiple GlanceAPIs
		spec["glanceAPIs"] = gapis

		raw := map[string]interface{}{
			"apiVersion": "glance.openstack.org/v1beta1",
			"kind":       "Glance",
			"metadata": map[string]interface{}{
				"name":      glanceTest.Instance.Name,
				"namespace": glanceTest.Instance.Namespace,
			},
			"spec": spec,
		}
		unstructuredObj := &unstructured.Unstructured{Object: raw}
		_, err := controllerutil.CreateOrPatch(
			ctx, k8sClient, unstructuredObj, func() error { return nil })

		Expect(err).Should(HaveOccurred())
		var statusError *k8s_errors.StatusError
		Expect(errors.As(err, &statusError)).To(BeTrue())
		// We shouldn't fail again for the backend, but because the endpoint is
		// not valid
		Expect(statusError.ErrStatus.Message).To(
			ContainSubstring(
				"KeystoneEndpoint is assigned to an invalid glanceAPI instance"),
		)
	})

	It("webhook rejects with wrong service override endpoint type", func() {
		spec := GetGlanceDefaultSpec()
		gapis := map[string]interface{}{
			"default": map[string]interface{}{
				"replicas":            1,
				"type":                "split",
				"customServiceConfig": GetDummyBackend(),
				"override": map[string]interface{}{
					"service": map[string]interface{}{
						"internal": map[string]interface{}{},
						"wrooong":  map[string]interface{}{},
					},
				},
			},
		}
		spec["glanceAPIs"] = gapis

		raw := map[string]interface{}{
			"apiVersion": "glance.openstack.org/v1beta1",
			"kind":       "Glance",
			"metadata": map[string]interface{}{
				"name":      glanceTest.Instance.Name,
				"namespace": glanceTest.Instance.Namespace,
			},
			"spec": spec,
		}
		unstructuredObj := &unstructured.Unstructured{Object: raw}
		_, err := controllerutil.CreateOrPatch(
			ctx, k8sClient, unstructuredObj, func() error { return nil })
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(
			ContainSubstring(
				"invalid: spec.glanceAPIs[default].override.service[wrooong]: " +
					"Invalid value: \"wrooong\": invalid endpoint type: wrooong"),
		)
	})

	It("webhooks reject the request - glanceAPI key too long", func() {
		// GlanceEmptySpec is used to provide a standard Glance CR where no
		// field is customized: we can inject our parameters to test webhooks
		spec := GetGlanceDefaultSpec()
		raw := map[string]interface{}{
			"apiVersion": "glance.openstack.org/v1beta1",
			"kind":       "Glance",
			"metadata": map[string]interface{}{
				"name":      glanceTest.Instance.Name,
				"namespace": glanceTest.Instance.Namespace,
			},
			"spec": spec,
		}

		apiList := map[string]interface{}{
			"foo-1234567890-1234567890-1234567890-1234567890-1234567890": GetDefaultGlanceAPISpec(GlanceAPITypeSingle),
		}
		spec["glanceAPIs"] = apiList

		unstructuredObj := &unstructured.Unstructured{Object: raw}
		_, err := controllerutil.CreateOrPatch(
			ctx, k8sClient, unstructuredObj, func() error { return nil })

		Expect(err).Should(HaveOccurred())
		var statusError *k8s_errors.StatusError
		Expect(errors.As(err, &statusError)).To(BeTrue())
		Expect(statusError.ErrStatus.Message).To(
			ContainSubstring(
				"Invalid value: \"foo-1234567890-1234567890-1234567890-1234567890-1234567890\": must be no more than 32 characters"),
		)
	})

	It("webhooks reject the request - glanceAPI wrong key/name", func() {
		// GlanceEmptySpec is used to provide a standard Glance CR where no
		// field is customized: we can inject our parameters to test webhooks
		spec := GetGlanceDefaultSpec()
		raw := map[string]interface{}{
			"apiVersion": "glance.openstack.org/v1beta1",
			"kind":       "Glance",
			"metadata": map[string]interface{}{
				"name":      glanceTest.Instance.Name,
				"namespace": glanceTest.Instance.Namespace,
			},
			"spec": spec,
		}

		apiList := map[string]interface{}{
			"foo_bar": GetDefaultGlanceAPISpec(GlanceAPITypeSingle),
		}
		spec["glanceAPIs"] = apiList

		unstructuredObj := &unstructured.Unstructured{Object: raw}
		_, err := controllerutil.CreateOrPatch(
			ctx, k8sClient, unstructuredObj, func() error { return nil })

		Expect(err).Should(HaveOccurred())
		var statusError *k8s_errors.StatusError
		Expect(errors.As(err, &statusError)).To(BeTrue())
		Expect(statusError.ErrStatus.Message).To(
			ContainSubstring(
				"Invalid value: \"foo_bar\": a lowercase RFC 1123 label must consist of lower case alphanumeric characters"),
		)
	})
})
