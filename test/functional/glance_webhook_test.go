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

	. "github.com/onsi/ginkgo/v2" //revive:disable:dot-imports
	. "github.com/onsi/gomega"    //revive:disable:dot-imports

	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var _ = Describe("Glance webhook", func() {
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
})
