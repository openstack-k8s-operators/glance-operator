/*
Copyright 2023.

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

package controllers

import (
	"github.com/google/uuid"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/openstack-k8s-operators/lib-common/modules/test/helpers"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	glancev1 "github.com/openstack-k8s-operators/glance-operator/api/v1beta1"
	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
)

func GetGlance(name types.NamespacedName) *glancev1.Glance {
	instance := &glancev1.Glance{}
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, name, instance)).Should(Succeed())
	}, timeout, interval).Should(Succeed())
	return instance
}

func GlanceConditionGetter(name types.NamespacedName) condition.Conditions {
	instance := GetGlance(name)
	return instance.Status.Conditions
}

var _ = Describe("Glance controller", func() {
	When("Glance is created", func() {
		It("initializes the status fields", func() {
			namespace := uuid.New().String()
			th.CreateNamespace(namespace)
			DeferCleanup(th.DeleteNamespace, namespace)

			raw := map[string]interface{}{
				"apiVersion": "glance.openstack.org/v1beta1",
				"kind":       "Glance",
				"metadata": map[string]interface{}{
					"name":      "glance",
					"namespace": namespace,
				},
				"spec": map[string]interface{}{
					// We shouldn't need to specify this as it is expected
					// to be defaulted by the webhook. However we did not
					// enable the webhook to run in the test *yet*.
					"containerImage":   "test-image-glance",
					"databaseInstance": "openstack",
					"storageRequest":   "10G",
					"glanceAPIExternal": map[string]interface{}{
						// I'm wondering what is the reason we have containerImage
						// on the top level and per GlanceAPI as well. Does
						// the top level overwrites the per API image?
						"containerImage": "test-image-glance-api-external",
					},
					"glanceAPIInternal": map[string]interface{}{
						"containerImage": "test-image-glance-api-internal",
					},
				},
			}
			th.CreateUnstructured(raw)

			glanceName := types.NamespacedName{Namespace: namespace, Name: "glance"}

			th.ExpectCondition(
				glanceName,
				ConditionGetterFunc(GlanceConditionGetter),
				condition.InputReadyCondition,
				corev1.ConditionFalse,
			)

			glance := GetGlance(glanceName)
			Expect(glance.Status.Conditions).To(HaveLen(11))
		})
	})
})
