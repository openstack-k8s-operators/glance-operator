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

package functional

import (
	"errors"
	"fmt"

	. "github.com/onsi/ginkgo/v2" //revive:disable:dot-imports
	. "github.com/onsi/gomega"    //revive:disable:dot-imports

	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var _ = Describe("Glance webhook", func() {
	newGlanceWithServicePortNames := func(name, publicPortName, internalPortName string) *unstructured.Unstructured {
		spec := GetDefaultGlanceSpec()
		glanceAPIs := spec["glanceAPIs"].(map[string]any)
		defaultAPI := glanceAPIs["default"].(map[string]any)
		defaultAPI["replicas"] = 0

		serviceOverrides := map[string]any{}
		for endpoint, portName := range map[string]string{
			"public":   publicPortName,
			"internal": internalPortName,
		} {
			serviceOverrides[endpoint] = map[string]any{
				"endpoint": endpoint,
				"spec": map[string]any{
					"ports": []any{
						map[string]any{"name": portName, "port": 9292},
					},
				},
			}
		}
		defaultAPI["override"] = map[string]any{"service": serviceOverrides}

		return &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "glance.openstack.org/v1beta1",
			"kind":       "Glance",
			"metadata": map[string]any{
				"name":      name,
				"namespace": namespace,
			},
			"spec": spec,
		}}
	}

	setGlanceServicePortName := func(object *unstructured.Unstructured, endpoint, portName string) {
		spec := object.Object["spec"].(map[string]any)
		glanceAPIs := spec["glanceAPIs"].(map[string]any)
		defaultAPI := glanceAPIs["default"].(map[string]any)
		override := defaultAPI["override"].(map[string]any)
		serviceOverrides := override["service"].(map[string]any)
		serviceOverride := serviceOverrides[endpoint].(map[string]any)
		serviceSpec := serviceOverride["spec"].(map[string]any)
		ports := serviceSpec["ports"].([]any)
		ports[0].(map[string]any)["name"] = portName
	}

	newGlanceWithEndpointURL := func(name, endpoint, endpointURL string, port *int32) *unstructured.Unstructured {
		spec := GetDefaultGlanceSpec()
		glanceAPIs := spec["glanceAPIs"].(map[string]any)
		defaultAPI := glanceAPIs["default"].(map[string]any)
		defaultAPI["replicas"] = 0

		serviceOverride := map[string]any{
			"endpoint":    endpoint,
			"endpointURL": endpointURL,
		}
		if port != nil {
			serviceOverride["spec"] = map[string]any{
				"ports": []any{
					map[string]any{"name": endpoint, "port": *port},
				},
			}
		}
		defaultAPI["override"] = map[string]any{
			"service": map[string]any{endpoint: serviceOverride},
		}

		return &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "glance.openstack.org/v1beta1",
			"kind":       "Glance",
			"metadata": map[string]any{
				"name":      name,
				"namespace": namespace,
			},
			"spec": spec,
		}}
	}

	It("rejects update to deprecated notificationBusInstance field", func() {
		spec := GetDefaultGlanceSpec()
		notificationBusInstance := "notifications-rabbitmq"
		spec["notificationBusInstance"] = notificationBusInstance

		// Set replicas to 0 to skip backend validation since this test
		// is focused on testing notificationBusInstance field validation
		glanceAPIs := spec["glanceAPIs"].(map[string]any)
		defaultAPI := glanceAPIs["default"].(map[string]any)
		defaultAPI["replicas"] = 0

		glanceName := types.NamespacedName{
			Namespace: namespace,
			Name:      "glance-webhook-test",
		}

		raw := map[string]any{
			"apiVersion": "glance.openstack.org/v1beta1",
			"kind":       "Glance",
			"metadata": map[string]any{
				"name":      glanceName.Name,
				"namespace": glanceName.Namespace,
			},
			"spec": spec,
		}

		// Create the Glance instance
		unstructuredObj := &unstructured.Unstructured{Object: raw}
		_, err := controllerutil.CreateOrPatch(
			ctx, k8sClient, unstructuredObj, func() error { return nil })
		Expect(err).ShouldNot(HaveOccurred())

		DeferCleanup(func() {
			_ = k8sClient.Delete(ctx, unstructuredObj)
		})

		// Try to update notificationBusInstance
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, glanceName, unstructuredObj)).Should(Succeed())
			specMap := unstructuredObj.Object["spec"].(map[string]any)
			specMap["notificationBusInstance"] = "notifications-rabbitmq2"
			err := k8sClient.Update(ctx, unstructuredObj)
			g.Expect(err).Should(HaveOccurred())

			var statusError *k8s_errors.StatusError
			g.Expect(errors.As(err, &statusError)).To(BeTrue())
			g.Expect(statusError.ErrStatus.Details.Kind).To(Equal("Glance"))
			g.Expect(statusError.ErrStatus.Message).To(
				ContainSubstring("field \"spec.notificationBusInstance\" is deprecated, use \"spec.notificationsBus.cluster\" instead"))
		}, timeout, interval).Should(Succeed())
	})

	It("accepts public and internal endpoint port names on create", func() {
		object := newGlanceWithServicePortNames("glance-port-names-create", "public", "internal")
		_, err := controllerutil.CreateOrPatch(ctx, k8sClient, object, func() error { return nil })
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() {
			_ = k8sClient.Delete(ctx, object)
		})
	})

	It("rejects noncanonical public and internal endpoint port names on create", func() {
		for _, testCase := range []struct {
			name                 string
			publicPortName       string
			internalPortName     string
			expectedErrorMessage string
		}{
			{
				name:                 "glance-port-name-create-public",
				publicPortName:       "wrong-public",
				internalPortName:     "internal",
				expectedErrorMessage: `must be "public"`,
			},
			{
				name:                 "glance-port-name-create-internal",
				publicPortName:       "public",
				internalPortName:     "wrong-internal",
				expectedErrorMessage: `must be "internal"`,
			},
		} {
			object := newGlanceWithServicePortNames(
				testCase.name,
				testCase.publicPortName,
				testCase.internalPortName,
			)
			_, err := controllerutil.CreateOrPatch(ctx, k8sClient, object, func() error { return nil })
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(testCase.expectedErrorMessage))
		}
	})

	It("rejects noncanonical public and internal endpoint port names on update", func() {
		object := newGlanceWithServicePortNames("glance-port-name-update", "public", "internal")
		_, err := controllerutil.CreateOrPatch(ctx, k8sClient, object, func() error { return nil })
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() {
			_ = k8sClient.Delete(ctx, object)
		})

		for _, testCase := range []struct {
			endpoint             string
			wrongName            string
			expectedErrorMessage string
		}{
			{endpoint: "public", wrongName: "wrong-public", expectedErrorMessage: `must be "public"`},
			{endpoint: "internal", wrongName: "wrong-internal", expectedErrorMessage: `must be "internal"`},
		} {
			updateErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				latest := &unstructured.Unstructured{}
				latest.SetAPIVersion(object.GetAPIVersion())
				latest.SetKind(object.GetKind())
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: object.GetName(), Namespace: namespace}, latest); err != nil {
					return err
				}
				setGlanceServicePortName(latest, testCase.endpoint, testCase.wrongName)
				return k8sClient.Update(ctx, latest)
			})
			Expect(updateErr).To(HaveOccurred())
			Expect(updateErr.Error()).To(ContainSubstring(testCase.expectedErrorMessage))
		}
	})

	It("rejects noncanonical service port names on a newly added GlanceAPI during update", func() {
		object := newGlanceWithServicePortNames("glance-port-name-new-api-update", "public", "internal")
		_, err := controllerutil.CreateOrPatch(ctx, k8sClient, object, func() error { return nil })
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() {
			_ = k8sClient.Delete(ctx, object)
		})

		updateErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			latest := &unstructured.Unstructured{}
			latest.SetAPIVersion(object.GetAPIVersion())
			latest.SetKind(object.GetKind())
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: object.GetName(), Namespace: namespace}, latest); err != nil {
				return err
			}
			spec := latest.Object["spec"].(map[string]any)
			glanceAPIs := spec["glanceAPIs"].(map[string]any)
			defaultAPI := glanceAPIs["default"].(map[string]any)
			newAPI := make(map[string]any, len(defaultAPI))
			for key, value := range defaultAPI {
				newAPI[key] = value
			}
			newAPI["override"] = map[string]any{
				"service": map[string]any{
					"public": map[string]any{
						"endpoint": "public",
						"spec": map[string]any{
							"ports": []any{map[string]any{"name": "wrong-public", "port": 9292}},
						},
					},
				},
			}
			glanceAPIs["new-api"] = newAPI
			return k8sClient.Update(ctx, latest)
		})
		Expect(updateErr).To(HaveOccurred())
		Expect(updateErr.Error()).To(ContainSubstring(`must be "public"`))
	})

	It("accepts a fully qualified endpointURL for public and internal endpoints", func() {
		for _, endpoint := range []string{"public", "internal"} {
			object := newGlanceWithEndpointURL("glance-epurl-ok-"+endpoint[:3], endpoint, "https://glance."+endpoint+".example.com", nil)
			_, err := controllerutil.CreateOrPatch(ctx, k8sClient, object, func() error { return nil })
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(func() {
				_ = k8sClient.Delete(ctx, object)
			})
		}
	})

	It("rejects an invalid endpointURL for public and internal endpoints", func() {
		for _, endpoint := range []string{"public", "internal"} {
			for _, testCase := range []struct {
				label                 string
				endpointURL           string
				expectedErrorMessages []string
			}{
				{
					label:                 "bare",
					endpointURL:           "https://glance",
					expectedErrorMessages: nil,
				},
				{
					label:                 "ns",
					endpointURL:           "glance.example.com",
					expectedErrorMessages: []string{"must be a valid URL with a scheme and host"},
				},
				{
					label:                 "ip",
					endpointURL:           "https://192.168.1.10",
					expectedErrorMessages: []string{"host must not be an IP literal"},
				},
			} {
				object := newGlanceWithEndpointURL(
					fmt.Sprintf("glance-epurl-bad-%s-%s", endpoint[:3], testCase.label),
					endpoint,
					testCase.endpointURL,
					nil,
				)
				_, err := controllerutil.CreateOrPatch(ctx, k8sClient, object, func() error { return nil })
				Expect(err).To(HaveOccurred(), testCase.label)
				for _, msg := range testCase.expectedErrorMessages {
					Expect(err.Error()).To(ContainSubstring(msg), testCase.label)
				}
			}
		}
	})

	It("rejects a ports[].port that disagrees with the endpointURL port, for public and internal endpoints", func() {
		for _, endpoint := range []string{"public", "internal"} {
			mismatchedPort := int32(9393)
			object := newGlanceWithEndpointURL(
				"glance-epurl-pm-"+endpoint[:3],
				endpoint,
				"https://glance."+endpoint+".example.com:9292",
				&mismatchedPort,
			)
			_, err := controllerutil.CreateOrPatch(ctx, k8sClient, object, func() error { return nil })
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("must match the port in endpointURL"))
		}
	})

	It("accepts a ports[].port that matches the endpointURL port, for public and internal endpoints", func() {
		for _, endpoint := range []string{"public", "internal"} {
			matchingPort := int32(9292)
			object := newGlanceWithEndpointURL(
				"glance-epurl-pok-"+endpoint[:3],
				endpoint,
				"https://glance."+endpoint+".example.com:9292",
				&matchingPort,
			)
			_, err := controllerutil.CreateOrPatch(ctx, k8sClient, object, func() error { return nil })
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(func() {
				_ = k8sClient.Delete(ctx, object)
			})
		}
	})
})
