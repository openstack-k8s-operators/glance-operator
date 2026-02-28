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
	"fmt"

	. "github.com/onsi/ginkgo/v2" //revive:disable:dot-imports
	. "github.com/onsi/gomega"    //revive:disable:dot-imports
	glancev1 "github.com/openstack-k8s-operators/glance-operator/api/v1beta1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var _ = Describe("Glance validation", func() {
	It("webhooks reject the request - invalid keystoneEndpoint", func() {
		// GlanceEmptySpec is used to provide a standard Glance CR where no
		// field is customized: we can inject our parameters to test webhooks
		spec := GetGlanceDefaultSpec()
		spec["keystoneEndpoint"] = "foo"
		raw := map[string]any{
			"apiVersion": "glance.openstack.org/v1beta1",
			"kind":       "Glance",
			"metadata": map[string]any{
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
			ContainSubstring(glancev1.KeystoneEndpointErrorMessage),
		)
	})

	It("webhooks reject the request - invalid backend", func() {
		spec := GetGlanceDefaultSpec()
		gapis := map[string]any{
			// Webhooks catch that a backend == File is set for an instance
			// that has type: split, which is invalid
			"default": map[string]any{
				"replicas": 1,
				"type":     "split",
			},
		}

		spec["keystoneEndpoint"] = "default"
		spec["glanceAPIs"] = gapis

		raw := map[string]any{
			"apiVersion": "glance.openstack.org/v1beta1",
			"kind":       "Glance",
			"metadata": map[string]any{
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
			ContainSubstring(glancev1.InvalidBackendErrorMessageSplit),
		)
	})

	It("webhooks reject the request - invalid instance", func() {
		spec := GetGlanceDefaultSpec()

		gapis := map[string]any{
			"edge2": map[string]any{
				"replicas": 1,
				"type":     "edge",
				// inject a valid Ceph backend
				"customServiceConfig": GetDummyBackend(),
			},
			"default": map[string]any{
				"replicas": 1,
				"type":     "split",
				// inject a valid Ceph backend
				"customServiceConfig": GetDummyBackend(),
			},
			"edge1": map[string]any{
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

		raw := map[string]any{
			"apiVersion": "glance.openstack.org/v1beta1",
			"kind":       "Glance",
			"metadata": map[string]any{
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
			ContainSubstring(glancev1.KeystoneEndpointErrorMessage),
		)
	})

	It("webhook rejects with wrong service override endpoint type", func() {
		spec := GetGlanceDefaultSpec()
		gapis := map[string]any{
			"default": map[string]any{
				"replicas":            1,
				"type":                "split",
				"customServiceConfig": GetDummyBackend(),
				"override": map[string]any{
					"service": map[string]any{
						"internal": map[string]any{},
						"wrooong":  map[string]any{},
					},
				},
			},
		}
		spec["glanceAPIs"] = gapis

		raw := map[string]any{
			"apiVersion": "glance.openstack.org/v1beta1",
			"kind":       "Glance",
			"metadata": map[string]any{
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
		raw := map[string]any{
			"apiVersion": "glance.openstack.org/v1beta1",
			"kind":       "Glance",
			"metadata": map[string]any{
				"name":      glanceTest.Instance.Name,
				"namespace": glanceTest.Instance.Namespace,
			},
			"spec": spec,
		}

		apiList := map[string]any{
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
		raw := map[string]any{
			"apiVersion": "glance.openstack.org/v1beta1",
			"kind":       "Glance",
			"metadata": map[string]any{
				"name":      glanceTest.Instance.Name,
				"namespace": glanceTest.Instance.Namespace,
			},
			"spec": spec,
		}

		apiList := map[string]any{
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
	DescribeTable("rejects wrong topology for",
		func(serviceNameFunc func() (string, string)) {

			api, errorPath := serviceNameFunc()
			expectedErrorMessage := fmt.Sprintf("spec.%s.namespace: Invalid value: \"namespace\": Customizing namespace field is not supported", errorPath)

			spec := GetDefaultGlanceSpec()
			if api != "top-level" {
				apiList := map[string]any{
					"default": map[string]any{
						"topologyRef": map[string]any{
							"name":      "foo",
							"namespace": "bar",
						},
					},
				}
				spec["glanceAPIs"] = apiList
			} else {
				// Reference a top-level topology
				spec["topologyRef"] = map[string]any{
					"name":      glanceTest.GlanceAPITopologies[0].Name,
					"namespace": "foo",
				}
			}
			// Build the Glance CR
			raw := map[string]any{
				"apiVersion": "glance.openstack.org/v1beta1",
				"kind":       "Glance",
				"metadata": map[string]any{
					"name":      glanceTest.Instance.Name,
					"namespace": glanceTest.Instance.Namespace,
				},
				"spec": spec,
			}

			unstructuredObj := &unstructured.Unstructured{Object: raw}
			_, err := controllerutil.CreateOrPatch(
				th.Ctx, th.K8sClient, unstructuredObj, func() error { return nil })
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(
				ContainSubstring(expectedErrorMessage))
		},
		Entry("top-level topologyRef", func() (string, string) {
			return "top-level", "topologyRef"
		}),
		Entry("default GlanceAPI topologyRef", func() (string, string) {
			api := "default"
			return api, fmt.Sprintf("glanceAPIs[%s].topologyRef", api)
		}),
	)
})
