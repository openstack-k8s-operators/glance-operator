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
	"github.com/google/uuid"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/openstack-k8s-operators/lib-common/modules/test/helpers"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	glancev1 "github.com/openstack-k8s-operators/glance-operator/api/v1beta1"
	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
)

var (
	namespace  string
	glanceName types.NamespacedName
)

var _ = Describe("Glance controller", func() {
	When("Glance is created", func() {
		BeforeEach(func() {
			namespace = uuid.New().String()
			th.CreateNamespace(namespace)
			DeferCleanup(th.DeleteNamespace, namespace)

			glanceName = types.NamespacedName{Namespace: namespace, Name: "glance"}
			DeferCleanup(th.DeleteInstance, CreateGlance(glanceName))

		})

		It("initializes the status fields", func() {
			Eventually(func(g Gomega) {
				glance := GetGlance(glanceName)
				g.Expect(glance.Status.Conditions).To(HaveLen(11))

			}, timeout, interval).Should(Succeed())
		})

		It("reports InputReady False as secret is not found", func() {
			th.ExpectConditionWithDetails(
				glanceName,
				ConditionGetterFunc(GlanceConditionGetter),
				condition.InputReadyCondition,
				corev1.ConditionFalse,
				condition.RequestedReason,
				"Input data resources missing",
			)
		})

		It("creates service account, role and rolebindig", func() {
			th.ExpectCondition(
				glanceName,
				ConditionGetterFunc(GlanceConditionGetter),
				condition.ServiceAccountReadyCondition,
				corev1.ConditionTrue,
			)
			sa := th.GetServiceAccount(types.NamespacedName{Namespace: namespace, Name: "glance-" + glanceName.Name})

			th.ExpectCondition(
				glanceName,
				ConditionGetterFunc(GlanceConditionGetter),
				condition.RoleReadyCondition,
				corev1.ConditionTrue,
			)
			role := th.GetRole(types.NamespacedName{Namespace: namespace, Name: "glance-" + glanceName.Name + "-role"})
			Expect(role.Rules).To(HaveLen(2))
			Expect(role.Rules[0].Resources).To(Equal([]string{"securitycontextconstraints"}))
			Expect(role.Rules[1].Resources).To(Equal([]string{"pods"}))

			th.ExpectCondition(
				glanceName,
				ConditionGetterFunc(GlanceConditionGetter),
				condition.RoleBindingReadyCondition,
				corev1.ConditionTrue,
			)
			binding := th.GetRoleBinding(types.NamespacedName{Namespace: namespace, Name: "glance-" + glanceName.Name + "-rolebinding"})
			Expect(binding.RoleRef.Name).To(Equal(role.Name))
			Expect(binding.Subjects).To(HaveLen(1))
			Expect(binding.Subjects[0].Name).To(Equal(sa.Name))
		})

		It("defaults the containerImages", func() {
			glance := GetGlance(glanceName)
			Expect(glance.Spec.ContainerImage).To(Equal(glancev1.GlanceAPIContainerImage))
			Expect(glance.Spec.GlanceAPIInternal.ContainerImage).To(Equal(glancev1.GlanceAPIContainerImage))
			Expect(glance.Spec.GlanceAPIExternal.ContainerImage).To(Equal(glancev1.GlanceAPIContainerImage))
		})

	})
})
