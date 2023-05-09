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
)

var _ = Describe("Glance controller", func() {
	When("Glance is created", func() {
		It("initializes the status fields", func() {
			namespace := uuid.New().String()
			th.CreateNamespace(namespace)
			DeferCleanup(th.DeleteNamespace, namespace)

			glanceName := "glance"

			raw := map[string]interface{}{
				"apiVersion": "glance.openstack.org/v1beta1",
				"kind":       "Glance",
				"metadata": map[string]interface{}{
					"name":      glanceName,
					"namespace": namespace,
				},
				"spec": map[string]interface{}{},
			}
			th.CreateUnstructured(raw)
		})
	})
})
