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

package functional_test

import (
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/gomega"

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

func CreateGlance(name types.NamespacedName) client.Object {
	raw := map[string]interface{}{
		"apiVersion": "glance.openstack.org/v1beta1",
		"kind":       "Glance",
		"metadata": map[string]interface{}{
			"name":      name.Name,
			"namespace": name.Namespace,
		},
		"spec": map[string]interface{}{
			"databaseInstance": "openstack",
			"storageRequest":   "10G",
		},
	}
	return th.CreateUnstructured(raw)
}
