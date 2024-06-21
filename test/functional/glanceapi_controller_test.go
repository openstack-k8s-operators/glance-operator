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

package functional

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2" //revive:disable:dot-imports
	. "github.com/onsi/gomega"    //revive:disable:dot-imports
	memcachedv1 "github.com/openstack-k8s-operators/infra-operator/apis/memcached/v1beta1"
	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"

	//revive:disable-next-line:dot-imports
	. "github.com/openstack-k8s-operators/lib-common/modules/common/test/helpers"
	mariadbv1 "github.com/openstack-k8s-operators/mariadb-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ptr "k8s.io/utils/ptr"
)

var _ = Describe("Glanceapi controller", func() {
	var memcachedSpec memcachedv1.MemcachedSpec

	BeforeEach(func() {
		memcachedSpec = memcachedv1.MemcachedSpec{
			MemcachedSpecCore: memcachedv1.MemcachedSpecCore{
				Replicas: ptr.To[int32](3),
			},
		}
		acc, accSecret := mariadb.CreateMariaDBAccountAndSecret(glanceTest.GlanceDatabaseAccount, mariadbv1.MariaDBAccountSpec{})
		DeferCleanup(k8sClient.Delete, ctx, accSecret)
		DeferCleanup(k8sClient.Delete, ctx, acc)
	})

	When("GlanceAPI CR is created", func() {
		BeforeEach(func() {
			DeferCleanup(th.DeleteInstance, CreateGlanceAPI(glanceTest.GlanceSingle, GetDefaultGlanceAPISpec(GlanceAPITypeSingle)))
			DeferCleanup(infra.DeleteMemcached, infra.CreateMemcached(namespace, glanceTest.MemcachedInstance, memcachedSpec))
			infra.SimulateMemcachedReady(glanceTest.GlanceMemcached)
		})
		It("is not Ready", func() {
			th.ExpectCondition(
				glanceTest.GlanceSingle,
				ConditionGetterFunc(GlanceAPIConditionGetter),
				condition.ReadyCondition,
				corev1.ConditionFalse,
			)
		})
		It("has empty Status fields", func() {
			instance := GetGlanceAPI(glanceTest.GlanceSingle)
			Expect(instance.Status.Hash).To(BeEmpty())
			Expect(instance.Status.ReadyCount).To(Equal(int32(0)))
		})
	})
	When("an unrelated Secret is created the CR state does not change", func() {
		BeforeEach(func() {
			DeferCleanup(th.DeleteInstance, CreateGlanceAPI(glanceTest.GlanceSingle, GetDefaultGlanceAPISpec(GlanceAPITypeSingle)))
			DeferCleanup(infra.DeleteMemcached, infra.CreateMemcached(namespace, glanceTest.MemcachedInstance, memcachedSpec))
			DeferCleanup(th.DeleteInstance, CreateGlanceAPI(glanceTest.GlanceSingle, GetDefaultGlanceAPISpec(GlanceAPITypeSingle)))
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "not-relevant-secret",
					Namespace: glanceTest.Instance.Namespace,
				},
			}
			Expect(k8sClient.Create(ctx, secret)).Should(Succeed())
			DeferCleanup(k8sClient.Delete, ctx, secret)
		})
		It("is not Ready", func() {
			th.ExpectCondition(
				glanceTest.GlanceSingle,
				ConditionGetterFunc(GlanceAPIConditionGetter),
				condition.ReadyCondition,
				corev1.ConditionFalse,
			)
		})
	})
	When("the Secret is created with all the expected fields", func() {
		BeforeEach(func() {
			DeferCleanup(infra.DeleteMemcached, infra.CreateMemcached(namespace, glanceTest.MemcachedInstance, memcachedSpec))
			infra.SimulateMemcachedReady(glanceTest.GlanceMemcached)
			DeferCleanup(th.DeleteInstance, CreateDefaultGlance(glanceTest.Instance))
			DeferCleanup(
				mariadb.DeleteDBService,
				mariadb.CreateDBService(
					glanceName.Namespace,
					GetGlance(glanceTest.Instance).Spec.DatabaseInstance,
					corev1.ServiceSpec{
						Ports: []corev1.ServicePort{{Port: 3306}},
					},
				),
			)
			mariadb.CreateMariaDBDatabase(glanceTest.GlanceDatabaseName.Namespace, glanceTest.GlanceDatabaseName.Name, mariadbv1.MariaDBDatabaseSpec{})
			DeferCleanup(k8sClient.Delete, ctx, mariadb.GetMariaDBDatabase(glanceTest.GlanceDatabaseName))

			spec := GetDefaultGlanceAPISpec(GlanceAPITypeSingle)
			spec["customServiceConfig"] = "foo=bar"
			DeferCleanup(th.DeleteInstance, CreateGlanceAPI(glanceTest.GlanceSingle, spec))
			DeferCleanup(keystone.DeleteKeystoneAPI, keystone.CreateKeystoneAPI(glanceTest.Instance.Namespace))
		})
		It("reports that input is ready", func() {
			th.ExpectCondition(
				glanceTest.GlanceSingle,
				ConditionGetterFunc(GlanceAPIConditionGetter),
				condition.InputReadyCondition,
				corev1.ConditionTrue,
			)
		})
		It("generated configs successfully", func() {
			th.ExpectCondition(
				glanceTest.GlanceSingle,
				ConditionGetterFunc(GlanceAPIConditionGetter),
				condition.ServiceConfigReadyCondition,
				corev1.ConditionTrue,
			)
			secretDataMap := th.GetSecret(glanceTest.GlanceSingleConfigMapData)
			Expect(secretDataMap).ShouldNot(BeNil())
			// We apply customServiceConfig to the GlanceAPI Pod
			Expect(secretDataMap.Data).Should(HaveKey("02-config.conf"))
			//Double check customServiceConfig has been applied
			configData := string(secretDataMap.Data["02-config.conf"])
			Expect(configData).Should(ContainSubstring("foo=bar"))

			Expect(secretDataMap).ShouldNot(BeNil())
			myCnf := secretDataMap.Data["my.cnf"]
			Expect(myCnf).To(
				ContainSubstring("[client]\nssl=0"))
		})
		It("stored the input hash in the Status", func() {
			Eventually(func(g Gomega) {
				glanceAPI := GetGlanceAPI(glanceTest.GlanceSingle)
				g.Expect(glanceAPI.Status.Hash).Should(HaveKeyWithValue("input", Not(BeEmpty())))
			}, timeout, interval).Should(Succeed())
		})
	})
	When("GlanceAPI is generated by the top-level CR", func() {
		BeforeEach(func() {
			DeferCleanup(infra.DeleteMemcached, infra.CreateMemcached(namespace, glanceTest.MemcachedInstance, memcachedSpec))
			infra.SimulateMemcachedReady(glanceTest.GlanceMemcached)
			DeferCleanup(th.DeleteInstance, CreateDefaultGlance(glanceTest.Instance))
			DeferCleanup(
				mariadb.DeleteDBService,
				mariadb.CreateDBService(
					glanceName.Namespace,
					GetGlance(glanceTest.Instance).Spec.DatabaseInstance,
					corev1.ServiceSpec{
						Ports: []corev1.ServicePort{{Port: 3306}},
					},
				),
			)

			mariadb.CreateMariaDBDatabase(glanceTest.GlanceDatabaseName.Namespace, glanceTest.GlanceDatabaseName.Name, mariadbv1.MariaDBDatabaseSpec{})
			DeferCleanup(k8sClient.Delete, ctx, mariadb.GetMariaDBDatabase(glanceTest.GlanceDatabaseName))

			DeferCleanup(th.DeleteInstance, CreateGlanceAPI(glanceTest.GlanceInternal, CreateGlanceAPISpec(GlanceAPITypeInternal)))
			DeferCleanup(th.DeleteInstance, CreateGlanceAPI(glanceTest.GlanceExternal, CreateGlanceAPISpec(GlanceAPITypeExternal)))
			DeferCleanup(keystone.DeleteKeystoneAPI, keystone.CreateKeystoneAPI(glanceTest.Instance.Namespace))
			th.ExpectCondition(
				glanceTest.GlanceInternal,
				ConditionGetterFunc(GlanceAPIConditionGetter),
				condition.ServiceConfigReadyCondition,
				corev1.ConditionTrue,
			)
			th.ExpectCondition(
				glanceTest.GlanceExternal,
				ConditionGetterFunc(GlanceAPIConditionGetter),
				condition.ServiceConfigReadyCondition,
				corev1.ConditionTrue,
			)
		})
		It("creates a StatefulSet for glance-api service - Internal", func() {
			th.SimulateStatefulSetReplicaReady(glanceTest.GlanceInternalStatefulSet)

			ss := th.GetStatefulSet(glanceTest.GlanceInternalStatefulSet)
			// Check the resulting deployment fields
			Expect(int(*ss.Spec.Replicas)).To(Equal(1))
			Expect(ss.Spec.Template.Spec.Volumes).To(HaveLen(4))
			Expect(ss.Spec.Template.Spec.Containers).To(HaveLen(3))

			container := ss.Spec.Template.Spec.Containers[2]
			Expect(container.VolumeMounts).To(HaveLen(6))
			Expect(container.Image).To(Equal(glanceTest.ContainerImage))
			Expect(container.LivenessProbe.HTTPGet.Port.IntVal).To(Equal(int32(9292)))
			Expect(container.ReadinessProbe.HTTPGet.Port.IntVal).To(Equal(int32(9292)))
			// Check headlessSvc Name matches with the statefulSet name
			headlessSvc := th.GetService(glanceTest.GlanceInternalStatefulSet)
			Expect(headlessSvc.Name).To(Equal(glanceTest.GlanceInternalStatefulSet.Name))
		})
		It("creates a StatefulSet for glance-api service - External", func() {
			th.SimulateStatefulSetReplicaReady(glanceTest.GlanceExternalStatefulSet)
			ss := th.GetStatefulSet(glanceTest.GlanceExternalStatefulSet)
			// Check the resulting deployment fields
			Expect(int(*ss.Spec.Replicas)).To(Equal(1))
			Expect(ss.Spec.Template.Spec.Volumes).To(HaveLen(4))
			Expect(ss.Spec.Template.Spec.Containers).To(HaveLen(3))

			// Check the glance-api container
			container := ss.Spec.Template.Spec.Containers[2]
			Expect(container.VolumeMounts).To(HaveLen(6))
			Expect(container.Image).To(Equal(glanceTest.ContainerImage))
			Expect(container.LivenessProbe.HTTPGet.Port.IntVal).To(Equal(int32(9292)))
			Expect(container.ReadinessProbe.HTTPGet.Port.IntVal).To(Equal(int32(9292)))

			// Check the glance-httpd container
			container = ss.Spec.Template.Spec.Containers[1]
			Expect(container.VolumeMounts).To(HaveLen(3))
			Expect(container.Image).To(Equal(glanceTest.ContainerImage))

			// Check the glance-log container
			container = ss.Spec.Template.Spec.Containers[0]
			Expect(container.VolumeMounts).To(HaveLen(1))
			Expect(container.Image).To(Equal(glanceTest.ContainerImage))

			// Check headlessSvc Name matches with the statefulSet name
			headlessSvc := th.GetService(glanceTest.GlanceExternalStatefulSet)
			Expect(headlessSvc.Name).To(Equal(glanceTest.GlanceExternalStatefulSet.Name))
		})
	})
	When("GlanceAPI is generated by the top-level CR (edge-api)", func() {
		BeforeEach(func() {
			DeferCleanup(infra.DeleteMemcached, infra.CreateMemcached(namespace, glanceTest.MemcachedInstance, memcachedSpec))
			infra.SimulateMemcachedReady(glanceTest.GlanceMemcached)
			DeferCleanup(th.DeleteInstance, CreateDefaultGlance(glanceTest.Instance))
			DeferCleanup(
				mariadb.DeleteDBService,
				mariadb.CreateDBService(
					glanceName.Namespace,
					GetGlance(glanceTest.Instance).Spec.DatabaseInstance,
					corev1.ServiceSpec{
						Ports: []corev1.ServicePort{{Port: 3306}},
					},
				),
			)

			mariadb.CreateMariaDBDatabase(glanceTest.GlanceDatabaseName.Namespace, glanceTest.GlanceDatabaseName.Name, mariadbv1.MariaDBDatabaseSpec{})
			DeferCleanup(k8sClient.Delete, ctx, mariadb.GetMariaDBDatabase(glanceTest.GlanceDatabaseName))

			DeferCleanup(th.DeleteInstance, CreateGlanceAPI(glanceTest.GlanceEdge, CreateGlanceAPISpec(GlanceAPITypeEdge)))
			DeferCleanup(keystone.DeleteKeystoneAPI, keystone.CreateKeystoneAPI(glanceTest.Instance.Namespace))
			th.ExpectCondition(
				glanceTest.GlanceEdge,
				ConditionGetterFunc(GlanceAPIConditionGetter),
				condition.ServiceConfigReadyCondition,
				corev1.ConditionTrue,
			)
		})
		It("creates a StatefulSet for glance-edge-api service", func() {
			th.SimulateStatefulSetReplicaReady(glanceTest.GlanceEdgeStatefulSet)
			ss := th.GetStatefulSet(glanceTest.GlanceEdgeStatefulSet)
			// Check the resulting deployment fields
			Expect(int(*ss.Spec.Replicas)).To(Equal(1))
			Expect(ss.Spec.Template.Spec.Volumes).To(HaveLen(4))
			Expect(ss.Spec.Template.Spec.Containers).To(HaveLen(3))

			container := ss.Spec.Template.Spec.Containers[2]
			Expect(container.VolumeMounts).To(HaveLen(6))
			Expect(container.Image).To(Equal(glanceTest.ContainerImage))
			Expect(container.LivenessProbe.HTTPGet.Port.IntVal).To(Equal(int32(9292)))
			Expect(container.ReadinessProbe.HTTPGet.Port.IntVal).To(Equal(int32(9292)))

			// Check headlessSvc Name matches with the statefulSet name
			headlessSvc := th.GetService(glanceTest.GlanceEdgeStatefulSet)
			Expect(headlessSvc.Name).To(Equal(glanceTest.GlanceEdgeStatefulSet.Name))

			// Check the Internal service exists and follow the usual convention
			internalSvc := th.GetService(glanceTest.GlanceInternalSvc)
			Expect(internalSvc.Name).To(Equal(glanceTest.GlanceInternalSvc.Name))

			// Check the Public service doesn't exist
			th.AssertServiceDoesNotExist(glanceTest.GlanceExternal)
		})
	})
	When("GlanceAPI is generated by the top-level CR (single-api)", func() {
		BeforeEach(func() {
			DeferCleanup(infra.DeleteMemcached, infra.CreateMemcached(namespace, glanceTest.MemcachedInstance, memcachedSpec))
			infra.SimulateMemcachedReady(glanceTest.GlanceMemcached)
			DeferCleanup(th.DeleteInstance, CreateDefaultGlance(glanceTest.Instance))
			DeferCleanup(
				mariadb.DeleteDBService,
				mariadb.CreateDBService(
					glanceName.Namespace,
					GetGlance(glanceTest.Instance).Spec.DatabaseInstance,
					corev1.ServiceSpec{
						Ports: []corev1.ServicePort{{Port: 3306}},
					},
				),
			)

			mariadb.CreateMariaDBDatabase(glanceTest.GlanceDatabaseName.Namespace, glanceTest.GlanceDatabaseName.Name, mariadbv1.MariaDBDatabaseSpec{})
			DeferCleanup(k8sClient.Delete, ctx, mariadb.GetMariaDBDatabase(glanceTest.GlanceDatabaseName))

			DeferCleanup(th.DeleteInstance, CreateGlanceAPI(glanceTest.GlanceSingle, CreateGlanceAPISpec(GlanceAPITypeSingle)))
			DeferCleanup(keystone.DeleteKeystoneAPI, keystone.CreateKeystoneAPI(glanceTest.Instance.Namespace))
			th.ExpectCondition(
				glanceTest.GlanceSingle,
				ConditionGetterFunc(GlanceAPIConditionGetter),
				condition.ServiceConfigReadyCondition,
				corev1.ConditionTrue,
			)
		})
		It("creates a StatefulSet for glance-single-api service", func() {
			th.SimulateStatefulSetReplicaReady(glanceTest.GlanceSingle)
			ss := th.GetStatefulSet(glanceTest.GlanceSingle)
			// Check the resulting deployment fields
			Expect(int(*ss.Spec.Replicas)).To(Equal(1))
			Expect(ss.Spec.Template.Spec.Volumes).To(HaveLen(4))
			Expect(ss.Spec.Template.Spec.Containers).To(HaveLen(3))

			container := ss.Spec.Template.Spec.Containers[2]
			Expect(container.VolumeMounts).To(HaveLen(6))
			Expect(container.Image).To(Equal(glanceTest.ContainerImage))
			Expect(container.LivenessProbe.HTTPGet.Port.IntVal).To(Equal(int32(9292)))
			Expect(container.ReadinessProbe.HTTPGet.Port.IntVal).To(Equal(int32(9292)))

			// Check headlessSvc Name matches with the statefulSet name
			headlessSvc := th.GetService(glanceTest.GlanceSingle)
			Expect(headlessSvc.Name).To(Equal(glanceTest.GlanceSingle.Name))
		})
	})
	When("the StatefulSet has at least one Replica ready - External", func() {
		BeforeEach(func() {
			DeferCleanup(infra.DeleteMemcached, infra.CreateMemcached(namespace, glanceTest.MemcachedInstance, memcachedSpec))
			infra.SimulateMemcachedReady(glanceTest.GlanceMemcached)
			DeferCleanup(th.DeleteInstance, CreateDefaultGlance(glanceTest.Instance))
			DeferCleanup(
				mariadb.DeleteDBService,
				mariadb.CreateDBService(
					glanceName.Namespace,
					GetGlance(glanceTest.Instance).Spec.DatabaseInstance,
					corev1.ServiceSpec{
						Ports: []corev1.ServicePort{{Port: 3306}},
					},
				),
			)
			mariadb.CreateMariaDBDatabase(glanceTest.GlanceDatabaseName.Namespace, glanceTest.GlanceDatabaseName.Name, mariadbv1.MariaDBDatabaseSpec{})
			DeferCleanup(k8sClient.Delete, ctx, mariadb.GetMariaDBDatabase(glanceTest.GlanceDatabaseName))

			DeferCleanup(th.DeleteInstance, CreateGlanceAPI(glanceTest.GlanceExternal, CreateGlanceAPISpec(GlanceAPITypeExternal)))
			DeferCleanup(keystone.DeleteKeystoneAPI, keystone.CreateKeystoneAPI(glanceTest.GlanceExternal.Namespace))
			th.SimulateStatefulSetReplicaReady(glanceTest.GlanceExternalStatefulSet)
			keystone.SimulateKeystoneEndpointReady(glanceTest.GlanceExternal)
		})
		It("reports that StatefulSet is ready", func() {
			th.ExpectCondition(
				glanceTest.GlanceExternal,
				ConditionGetterFunc(GlanceAPIConditionGetter),
				condition.DeploymentReadyCondition,
				corev1.ConditionTrue,
			)
			// StatefulSet is Ready, check the actual ReadyCount is > 0
			glanceAPI := GetGlanceAPI(glanceTest.GlanceExternal)
			Expect(glanceAPI.Status.ReadyCount).To(BeNumerically(">", 0))
		})
		It("exposes the service", func() {
			apiInstance := th.GetService(glanceTest.GlancePublicSvc)
			Expect(apiInstance.Labels["service"]).To(Equal("glance"))
		})
		It("creates KeystoneEndpoint", func() {
			keystoneEndpoint := keystone.GetKeystoneEndpoint(glanceTest.GlanceExternal)
			endpoints := keystoneEndpoint.Spec.Endpoints
			Expect(endpoints).To(HaveKeyWithValue("public", "http://glance-default-public."+glanceTest.Instance.Namespace+".svc:9292"))
			th.ExpectCondition(
				glanceTest.GlanceExternal,
				ConditionGetterFunc(GlanceAPIConditionGetter),
				condition.KeystoneEndpointReadyCondition,
				corev1.ConditionTrue,
			)
		})
	})
	When("the StatefulSet has at least one Replica ready - Internal", func() {
		BeforeEach(func() {
			DeferCleanup(infra.DeleteMemcached, infra.CreateMemcached(namespace, glanceTest.MemcachedInstance, memcachedSpec))
			infra.SimulateMemcachedReady(glanceTest.GlanceMemcached)
			DeferCleanup(th.DeleteInstance, CreateDefaultGlance(glanceTest.Instance))
			DeferCleanup(
				mariadb.DeleteDBService,
				mariadb.CreateDBService(
					glanceName.Namespace,
					GetGlance(glanceTest.Instance).Spec.DatabaseInstance,
					corev1.ServiceSpec{
						Ports: []corev1.ServicePort{{Port: 3306}},
					},
				),
			)

			mariadb.CreateMariaDBDatabase(glanceTest.GlanceDatabaseName.Namespace, glanceTest.GlanceDatabaseName.Name, mariadbv1.MariaDBDatabaseSpec{})
			DeferCleanup(k8sClient.Delete, ctx, mariadb.GetMariaDBDatabase(glanceTest.GlanceDatabaseName))

			DeferCleanup(th.DeleteInstance, CreateGlanceAPI(glanceTest.GlanceInternal, CreateGlanceAPISpec(GlanceAPITypeInternal)))
			DeferCleanup(keystone.DeleteKeystoneAPI, keystone.CreateKeystoneAPI(glanceTest.GlanceInternal.Namespace))
			th.SimulateStatefulSetReplicaReady(glanceTest.GlanceInternalStatefulSet)
			keystone.SimulateKeystoneEndpointReady(glanceTest.GlanceInternal)
		})
		It("reports that StatefulSet is ready", func() {
			th.ExpectCondition(
				glanceTest.GlanceInternal,
				ConditionGetterFunc(GlanceAPIConditionGetter),
				condition.DeploymentReadyCondition,
				corev1.ConditionTrue,
			)
			// StatefulSet is Ready, check the actual ReadyCount is > 0
			glanceAPI := GetGlanceAPI(glanceTest.GlanceInternal)
			Expect(glanceAPI.Status.ReadyCount).To(BeNumerically(">", 0))
		})
		It("exposes the service - Internal", func() {
			apiInstance := th.GetService(glanceTest.GlanceInternalSvc)
			Expect(apiInstance.Labels["service"]).To(Equal("glance"))
		})
		It("creates KeystoneEndpoint - Internal", func() {
			keystoneEndpoint := keystone.GetKeystoneEndpoint(glanceTest.GlanceInternal)
			endpoints := keystoneEndpoint.Spec.Endpoints
			Expect(endpoints).To(HaveKeyWithValue("internal", "http://glance-default-internal."+glanceTest.Instance.Namespace+".svc:9292"))
			th.ExpectCondition(
				glanceTest.GlanceInternal,
				ConditionGetterFunc(GlanceAPIConditionGetter),
				condition.KeystoneEndpointReadyCondition,
				corev1.ConditionTrue,
			)
		})
	})
	When("the StatefulSet has at least one Replica ready - Single", func() {
		BeforeEach(func() {
			DeferCleanup(infra.DeleteMemcached, infra.CreateMemcached(namespace, glanceTest.MemcachedInstance, memcachedSpec))
			infra.SimulateMemcachedReady(glanceTest.GlanceMemcached)
			DeferCleanup(th.DeleteInstance, CreateDefaultGlance(glanceTest.Instance))
			DeferCleanup(
				mariadb.DeleteDBService,
				mariadb.CreateDBService(
					glanceName.Namespace,
					GetGlance(glanceTest.Instance).Spec.DatabaseInstance,
					corev1.ServiceSpec{
						Ports: []corev1.ServicePort{{Port: 3306}},
					},
				),
			)

			mariadb.CreateMariaDBDatabase(glanceTest.GlanceDatabaseName.Namespace, glanceTest.GlanceDatabaseName.Name, mariadbv1.MariaDBDatabaseSpec{})
			DeferCleanup(k8sClient.Delete, ctx, mariadb.GetMariaDBDatabase(glanceTest.GlanceDatabaseName))

			DeferCleanup(th.DeleteInstance, CreateGlanceAPI(glanceTest.GlanceSingle, CreateGlanceAPISpec(GlanceAPITypeSingle)))
			DeferCleanup(keystone.DeleteKeystoneAPI, keystone.CreateKeystoneAPI(glanceTest.GlanceSingle.Namespace))
			th.SimulateStatefulSetReplicaReady(glanceTest.GlanceSingle)
			keystone.SimulateKeystoneEndpointReady(glanceTest.GlanceSingle)
		})
		It("reports that StatefulSet is ready", func() {
			th.ExpectCondition(
				glanceTest.GlanceSingle,
				ConditionGetterFunc(GlanceAPIConditionGetter),
				condition.DeploymentReadyCondition,
				corev1.ConditionTrue,
			)
			// StatefulSet is Ready, check the actual ReadyCount is > 0
			glanceAPI := GetGlanceAPI(glanceTest.GlanceSingle)
			Expect(glanceAPI.Status.ReadyCount).To(BeNumerically(">", 0))
		})
		It("exposes the service", func() {
			apiInstance := th.GetService(glanceTest.GlanceInternalSvc)
			Expect(apiInstance.Labels["service"]).To(Equal("glance"))
		})
		It("creates KeystoneEndpoint", func() {
			keystoneEndpoint := keystone.GetKeystoneEndpoint(glanceTest.GlanceSingle)
			endpoints := keystoneEndpoint.Spec.Endpoints
			Expect(endpoints).To(HaveKeyWithValue("internal", "http://glance-default-internal."+glanceTest.Instance.Namespace+".svc:9292"))
			Expect(endpoints).To(HaveKeyWithValue("public", "http://glance-default-public."+glanceTest.Instance.Namespace+".svc:9292"))
			th.ExpectCondition(
				glanceTest.GlanceSingle,
				ConditionGetterFunc(GlanceAPIConditionGetter),
				condition.KeystoneEndpointReadyCondition,
				corev1.ConditionTrue,
			)
		})
	})
	When("A GlanceAPI is created with service override", func() {
		BeforeEach(func() {
			DeferCleanup(infra.DeleteMemcached, infra.CreateMemcached(namespace, glanceTest.MemcachedInstance, memcachedSpec))
			infra.SimulateMemcachedReady(glanceTest.GlanceMemcached)
			DeferCleanup(th.DeleteInstance, CreateDefaultGlance(glanceTest.Instance))
			DeferCleanup(
				mariadb.DeleteDBService,
				mariadb.CreateDBService(
					glanceName.Namespace,
					GetGlance(glanceTest.Instance).Spec.DatabaseInstance,
					corev1.ServiceSpec{
						Ports: []corev1.ServicePort{{Port: 3306}},
					},
				),
			)
			mariadb.CreateMariaDBDatabase(glanceTest.GlanceDatabaseName.Namespace, glanceTest.GlanceDatabaseName.Name, mariadbv1.MariaDBDatabaseSpec{})
			DeferCleanup(k8sClient.Delete, ctx, mariadb.GetMariaDBDatabase(glanceTest.GlanceDatabaseName))

			spec := CreateGlanceAPISpec(GlanceAPITypeInternal)
			serviceOverride := map[string]interface{}{}
			serviceOverride["internal"] = map[string]interface{}{
				"endpoint": "internal",
				"metadata": map[string]map[string]string{
					"annotations": {
						"dnsmasq.network.openstack.org/hostname": "glance-internal.openstack.svc",
						"metallb.universe.tf/address-pool":       "osp-internalapi",
						"metallb.universe.tf/allow-shared-ip":    "osp-internalapi",
						"metallb.universe.tf/loadBalancerIPs":    "internal-lb-ip-1,internal-lb-ip-2",
					},
					"labels": {
						"internal": "true",
						"service":  "glance",
					},
				},
				"spec": map[string]interface{}{
					"type": "LoadBalancer",
				},
			}

			spec["override"] = map[string]interface{}{
				"service": serviceOverride,
			}
			glance := CreateGlanceAPI(glanceTest.GlanceInternal, spec)
			th.SimulateLoadBalancerServiceIP(glanceTest.GlanceInternalSvc)
			DeferCleanup(keystone.DeleteKeystoneAPI, keystone.CreateKeystoneAPI(glanceTest.GlanceInternal.Namespace))
			th.SimulateStatefulSetReplicaReady(glanceTest.GlanceInternalStatefulSet)
			keystone.SimulateKeystoneEndpointReady(glanceTest.GlanceInternal)
			DeferCleanup(th.DeleteInstance, glance)
		})
		It("creates KeystoneEndpoint", func() {
			keystoneEndpoint := keystone.GetKeystoneEndpoint(glanceTest.GlanceInternal)
			endpoints := keystoneEndpoint.Spec.Endpoints
			Expect(endpoints).To(HaveKeyWithValue("internal", "http://glance-default-internal."+glanceTest.GlanceInternal.Namespace+".svc:9292"))
			th.ExpectCondition(
				glanceTest.GlanceInternal,
				ConditionGetterFunc(GlanceAPIConditionGetter),
				condition.KeystoneEndpointReadyCondition,
				corev1.ConditionTrue,
			)
		})
		It("creates LoadBalancer service", func() {
			// As the internal endpoint is configured in service overrides it
			// gets a LoadBalancer Service with annotations
			service := th.GetService(glanceTest.GlanceInternalSvc)
			Expect(service.Annotations).To(
				HaveKeyWithValue("dnsmasq.network.openstack.org/hostname", "glance-internal.openstack.svc"))
			Expect(service.Annotations).To(
				HaveKeyWithValue("metallb.universe.tf/address-pool", "osp-internalapi"))
			Expect(service.Annotations).To(
				HaveKeyWithValue("metallb.universe.tf/allow-shared-ip", "osp-internalapi"))
			Expect(service.Annotations).To(
				HaveKeyWithValue("metallb.universe.tf/loadBalancerIPs", "internal-lb-ip-1,internal-lb-ip-2"))

			th.ExpectCondition(
				glanceTest.GlanceInternal,
				ConditionGetterFunc(GlanceAPIConditionGetter),
				condition.ReadyCondition,
				corev1.ConditionTrue,
			)
		})
	})
	When("A GlanceAPI is created with service override endpointURL set", func() {
		BeforeEach(func() {
			DeferCleanup(infra.DeleteMemcached, infra.CreateMemcached(namespace, glanceTest.MemcachedInstance, memcachedSpec))
			infra.SimulateMemcachedReady(glanceTest.GlanceMemcached)
			DeferCleanup(th.DeleteInstance, CreateDefaultGlance(glanceTest.Instance))
			DeferCleanup(
				mariadb.DeleteDBService,
				mariadb.CreateDBService(
					glanceName.Namespace,
					GetGlance(glanceTest.Instance).Spec.DatabaseInstance,
					corev1.ServiceSpec{
						Ports: []corev1.ServicePort{{Port: 3306}},
					},
				),
			)

			mariadb.CreateMariaDBDatabase(glanceTest.GlanceDatabaseName.Namespace, glanceTest.GlanceDatabaseName.Name, mariadbv1.MariaDBDatabaseSpec{})
			DeferCleanup(k8sClient.Delete, ctx, mariadb.GetMariaDBDatabase(glanceTest.GlanceDatabaseName))

			spec := CreateGlanceAPISpec(GlanceAPITypeExternal)
			serviceOverride := map[string]interface{}{}
			serviceOverride["public"] = map[string]interface{}{
				"endpoint":    "public",
				"endpointURL": "http://glance-openstack.apps-crc.testing",
			}
			spec["override"] = map[string]interface{}{
				"service": serviceOverride,
			}
			glance := CreateGlanceAPI(glanceTest.GlanceExternal, spec)
			DeferCleanup(keystone.DeleteKeystoneAPI, keystone.CreateKeystoneAPI(glanceTest.GlanceExternal.Namespace))
			th.SimulateStatefulSetReplicaReady(glanceTest.GlanceExternalStatefulSet)
			keystone.SimulateKeystoneEndpointReady(glanceTest.GlanceExternal)
			DeferCleanup(th.DeleteInstance, glance)
		})
		It("creates KeystoneEndpoint", func() {
			keystoneEndpoint := keystone.GetKeystoneEndpoint(glanceTest.GlanceExternal)
			endpoints := keystoneEndpoint.Spec.Endpoints
			Expect(endpoints).To(HaveKeyWithValue("public", "http://glance-openstack.apps-crc.testing"))

			th.ExpectCondition(
				glanceTest.GlanceExternal,
				ConditionGetterFunc(GlanceAPIConditionGetter),
				condition.KeystoneEndpointReadyCondition,
				corev1.ConditionTrue,
			)
		})
	})

	When("A split GlanceAPI with TLS is generated by the top-level CR", func() {
		BeforeEach(func() {
			DeferCleanup(infra.DeleteMemcached, infra.CreateMemcached(namespace, glanceTest.MemcachedInstance, memcachedSpec))
			infra.SimulateMemcachedReady(glanceTest.GlanceMemcached)
			DeferCleanup(th.DeleteInstance, CreateDefaultGlance(glanceTest.Instance))
			DeferCleanup(
				mariadb.DeleteDBService,
				mariadb.CreateDBService(
					glanceName.Namespace,
					GetGlance(glanceTest.Instance).Spec.DatabaseInstance,
					corev1.ServiceSpec{
						Ports: []corev1.ServicePort{{Port: 3306}},
					},
				),
			)

			mariadb.CreateMariaDBDatabase(glanceTest.GlanceDatabaseName.Namespace, glanceTest.GlanceDatabaseName.Name, mariadbv1.MariaDBDatabaseSpec{})
			DeferCleanup(k8sClient.Delete, ctx, mariadb.GetMariaDBDatabase(glanceTest.GlanceDatabaseName))
			mariadb.SimulateMariaDBTLSDatabaseCompleted(glanceTest.GlanceDatabaseName)

			DeferCleanup(k8sClient.Delete, ctx, th.CreateCABundleSecret(glanceTest.CABundleSecret))
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(glanceTest.InternalCertSecret))
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(glanceTest.PublicCertSecret))
			DeferCleanup(th.DeleteInstance, CreateDefaultGlance(glanceTest.Instance))
			DeferCleanup(th.DeleteInstance, CreateGlanceAPI(glanceTest.GlanceInternal, GetTLSGlanceAPISpec(GlanceAPITypeInternal)))
			DeferCleanup(th.DeleteInstance, CreateGlanceAPI(glanceTest.GlanceExternal, GetTLSGlanceAPISpec(GlanceAPITypeExternal)))
			DeferCleanup(keystone.DeleteKeystoneAPI, keystone.CreateKeystoneAPI(glanceTest.Instance.Namespace))
			keystone.SimulateKeystoneEndpointReady(glanceTest.GlanceExternal)
			keystone.SimulateKeystoneEndpointReady(glanceTest.GlanceInternal)

			th.ExpectCondition(
				glanceTest.GlanceInternal,
				ConditionGetterFunc(GlanceAPIConditionGetter),
				condition.ServiceConfigReadyCondition,
				corev1.ConditionTrue,
			)
			th.ExpectCondition(
				glanceTest.GlanceExternal,
				ConditionGetterFunc(GlanceAPIConditionGetter),
				condition.ServiceConfigReadyCondition,
				corev1.ConditionTrue,
			)
		})
		It("creates a StatefulSet for glance-api service with TLS certs attached - Internal", func() {
			th.SimulateStatefulSetReplicaReady(glanceTest.GlanceInternalStatefulSet)

			ss := th.GetStatefulSet(glanceTest.GlanceInternalStatefulSet)
			// Check the resulting deployment fields
			Expect(int(*ss.Spec.Replicas)).To(Equal(1))
			Expect(ss.Spec.Template.Spec.Volumes).To(HaveLen(6))
			Expect(ss.Spec.Template.Spec.Containers).To(HaveLen(3))

			// cert deployment volumes
			th.AssertVolumeExists(glanceTest.CABundleSecret.Name, ss.Spec.Template.Spec.Volumes)
			th.AssertVolumeExists(glanceTest.InternalCertSecret.Name, ss.Spec.Template.Spec.Volumes)
			Expect(ss.Spec.Template.Spec.Volumes).ToNot(ContainElement(HaveField("Name", glanceTest.PublicCertSecret.Name)))

			// svc container ca cert
			svcContainer := ss.Spec.Template.Spec.Containers[2]
			th.AssertVolumeMountExists(glanceTest.CABundleSecret.Name, "tls-ca-bundle.pem", svcContainer.VolumeMounts)

			// httpd container certs
			httpdProxyContainer := ss.Spec.Template.Spec.Containers[1]
			th.AssertVolumeMountExists(glanceTest.InternalCertSecret.Name, "tls.key", httpdProxyContainer.VolumeMounts)
			th.AssertVolumeMountExists(glanceTest.InternalCertSecret.Name, "tls.crt", httpdProxyContainer.VolumeMounts)
			th.AssertVolumeMountExists(glanceTest.CABundleSecret.Name, "tls-ca-bundle.pem", httpdProxyContainer.VolumeMounts)

			Expect(httpdProxyContainer.ReadinessProbe.HTTPGet.Scheme).To(Equal(corev1.URISchemeHTTPS))
			Expect(httpdProxyContainer.LivenessProbe.HTTPGet.Scheme).To(Equal(corev1.URISchemeHTTPS))
		})
		It("creates a StatefulSet for glance-api service with TLS certs attached - External", func() {
			th.SimulateStatefulSetReplicaReady(glanceTest.GlanceExternalStatefulSet)
			ss := th.GetStatefulSet(glanceTest.GlanceExternalStatefulSet)
			// Check the resulting deployment fields
			Expect(int(*ss.Spec.Replicas)).To(Equal(1))
			Expect(ss.Spec.Template.Spec.Volumes).To(HaveLen(6))
			Expect(ss.Spec.Template.Spec.Containers).To(HaveLen(3))

			// cert deployment volumes
			th.AssertVolumeExists(glanceTest.CABundleSecret.Name, ss.Spec.Template.Spec.Volumes)
			th.AssertVolumeExists(glanceTest.PublicCertSecret.Name, ss.Spec.Template.Spec.Volumes)
			Expect(ss.Spec.Template.Spec.Volumes).ToNot(ContainElement(HaveField("Name", glanceTest.InternalCertSecret.Name)))

			// svc container ca cert
			svcContainer := ss.Spec.Template.Spec.Containers[2]
			th.AssertVolumeMountExists(glanceTest.CABundleSecret.Name, "tls-ca-bundle.pem", svcContainer.VolumeMounts)

			// httpd container certs
			httpdProxyContainer := ss.Spec.Template.Spec.Containers[1]
			th.AssertVolumeMountExists(glanceTest.PublicCertSecret.Name, "tls.key", httpdProxyContainer.VolumeMounts)
			th.AssertVolumeMountExists(glanceTest.PublicCertSecret.Name, "tls.crt", httpdProxyContainer.VolumeMounts)
			th.AssertVolumeMountExists(glanceTest.CABundleSecret.Name, "tls-ca-bundle.pem", httpdProxyContainer.VolumeMounts)

			Expect(httpdProxyContainer.ReadinessProbe.HTTPGet.Scheme).To(Equal(corev1.URISchemeHTTPS))
			Expect(httpdProxyContainer.LivenessProbe.HTTPGet.Scheme).To(Equal(corev1.URISchemeHTTPS))
		})

		It("TLS Endpoints are created", func() {
			keystoneEndpoint := keystone.GetKeystoneEndpoint(glanceTest.GlanceExternal)
			endpoints := keystoneEndpoint.Spec.Endpoints
			Expect(endpoints).To(HaveKeyWithValue("public", "https://glance-default-public."+glanceTest.Instance.Namespace+".svc:9292"))
			th.ExpectCondition(
				glanceTest.GlanceExternal,
				ConditionGetterFunc(GlanceAPIConditionGetter),
				condition.KeystoneEndpointReadyCondition,
				corev1.ConditionTrue,
			)

			keystoneEndpoint = keystone.GetKeystoneEndpoint(glanceTest.GlanceInternal)
			endpoints = keystoneEndpoint.Spec.Endpoints
			Expect(endpoints).To(HaveKeyWithValue("internal", "https://glance-default-internal."+glanceTest.Instance.Namespace+".svc:9292"))
			th.ExpectCondition(
				glanceTest.GlanceInternal,
				ConditionGetterFunc(GlanceAPIConditionGetter),
				condition.KeystoneEndpointReadyCondition,
				corev1.ConditionTrue,
			)
		})
	})

	When("A single GlanceAPI with TLS is generated by the top-level CR (single-api)", func() {
		BeforeEach(func() {
			DeferCleanup(infra.DeleteMemcached, infra.CreateMemcached(namespace, glanceTest.MemcachedInstance, memcachedSpec))
			infra.SimulateMemcachedReady(glanceTest.GlanceMemcached)
			DeferCleanup(th.DeleteInstance, CreateDefaultGlance(glanceTest.Instance))
			DeferCleanup(
				mariadb.DeleteDBService,
				mariadb.CreateDBService(
					glanceName.Namespace,
					GetGlance(glanceTest.Instance).Spec.DatabaseInstance,
					corev1.ServiceSpec{
						Ports: []corev1.ServicePort{{Port: 3306}},
					},
				),
			)

			mariadb.CreateMariaDBDatabase(glanceTest.GlanceDatabaseName.Namespace, glanceTest.GlanceDatabaseName.Name, mariadbv1.MariaDBDatabaseSpec{})
			DeferCleanup(k8sClient.Delete, ctx, mariadb.GetMariaDBDatabase(glanceTest.GlanceDatabaseName))
			mariadb.SimulateMariaDBTLSDatabaseCompleted(glanceTest.GlanceDatabaseName)

			DeferCleanup(th.DeleteInstance, CreateDefaultGlance(glanceTest.Instance))
			DeferCleanup(th.DeleteInstance, CreateGlanceAPI(glanceTest.GlanceSingle, GetTLSGlanceAPISpec(GlanceAPITypeSingle)))
			DeferCleanup(keystone.DeleteKeystoneAPI, keystone.CreateKeystoneAPI(glanceTest.Instance.Namespace))
		})

		It("reports that the CA secret is missing", func() {
			th.ExpectConditionWithDetails(
				glanceTest.GlanceSingle,
				ConditionGetterFunc(GlanceAPIConditionGetter),
				condition.TLSInputReadyCondition,
				corev1.ConditionFalse,
				condition.ErrorReason,
				fmt.Sprintf("TLSInput error occured in TLS sources Secret %s/combined-ca-bundle not found", namespace),
			)
			th.ExpectCondition(
				glanceTest.GlanceSingle,
				ConditionGetterFunc(GlanceAPIConditionGetter),
				condition.ReadyCondition,
				corev1.ConditionFalse,
			)
		})

		It("reports that the internal cert secret is missing", func() {
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCABundleSecret(glanceTest.CABundleSecret))
			th.ExpectConditionWithDetails(
				glanceTest.GlanceSingle,
				ConditionGetterFunc(GlanceAPIConditionGetter),
				condition.TLSInputReadyCondition,
				corev1.ConditionFalse,
				condition.ErrorReason,
				fmt.Sprintf("TLSInput error occured in TLS sources Secret %s/internal-tls-certs not found", namespace),
			)
			th.ExpectCondition(
				glanceTest.GlanceSingle,
				ConditionGetterFunc(GlanceAPIConditionGetter),
				condition.ReadyCondition,
				corev1.ConditionFalse,
			)
		})

		It("reports that the public cert secret is missing", func() {
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCABundleSecret(glanceTest.CABundleSecret))
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(glanceTest.InternalCertSecret))
			th.ExpectConditionWithDetails(
				glanceTest.GlanceSingle,
				ConditionGetterFunc(GlanceAPIConditionGetter),
				condition.TLSInputReadyCondition,
				corev1.ConditionFalse,
				condition.ErrorReason,
				fmt.Sprintf("TLSInput error occured in TLS sources Secret %s/public-tls-certs not found", namespace),
			)
			th.ExpectCondition(
				glanceTest.GlanceSingle,
				ConditionGetterFunc(GlanceAPIConditionGetter),
				condition.ReadyCondition,
				corev1.ConditionFalse,
			)
		})

		It("creates a StatefulSet for glance-api service with TLS certs attached", func() {
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCABundleSecret(glanceTest.CABundleSecret))
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(glanceTest.InternalCertSecret))
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(glanceTest.PublicCertSecret))
			keystone.SimulateKeystoneEndpointReady(glanceTest.GlanceSingle)
			th.SimulateStatefulSetReplicaReady(glanceTest.GlanceSingle)

			th.ExpectCondition(
				glanceTest.GlanceSingle,
				ConditionGetterFunc(GlanceAPIConditionGetter),
				condition.ServiceConfigReadyCondition,
				corev1.ConditionTrue,
			)

			th.ExpectCondition(
				glanceTest.GlanceSingle,
				ConditionGetterFunc(GlanceAPIConditionGetter),
				condition.TLSInputReadyCondition,
				corev1.ConditionTrue,
			)

			ss := th.GetStatefulSet(glanceTest.GlanceSingle)
			// Check the resulting deployment fields
			Expect(int(*ss.Spec.Replicas)).To(Equal(1))
			Expect(ss.Spec.Template.Spec.Volumes).To(HaveLen(7))
			Expect(ss.Spec.Template.Spec.Containers).To(HaveLen(3))

			// cert deployment volumes
			th.AssertVolumeExists(glanceTest.CABundleSecret.Name, ss.Spec.Template.Spec.Volumes)
			th.AssertVolumeExists(glanceTest.InternalCertSecret.Name, ss.Spec.Template.Spec.Volumes)
			th.AssertVolumeExists(glanceTest.PublicCertSecret.Name, ss.Spec.Template.Spec.Volumes)

			// svc container ca cert
			svcContainer := ss.Spec.Template.Spec.Containers[2]
			th.AssertVolumeMountExists(glanceTest.CABundleSecret.Name, "tls-ca-bundle.pem", svcContainer.VolumeMounts)

			// httpd container certs
			httpdProxyContainer := ss.Spec.Template.Spec.Containers[1]
			th.AssertVolumeMountExists(glanceTest.InternalCertSecret.Name, "tls.key", httpdProxyContainer.VolumeMounts)
			th.AssertVolumeMountExists(glanceTest.InternalCertSecret.Name, "tls.crt", httpdProxyContainer.VolumeMounts)
			th.AssertVolumeMountExists(glanceTest.PublicCertSecret.Name, "tls.key", httpdProxyContainer.VolumeMounts)
			th.AssertVolumeMountExists(glanceTest.PublicCertSecret.Name, "tls.crt", httpdProxyContainer.VolumeMounts)
			th.AssertVolumeMountExists(glanceTest.CABundleSecret.Name, "tls-ca-bundle.pem", httpdProxyContainer.VolumeMounts)

			Expect(httpdProxyContainer.ReadinessProbe.HTTPGet.Scheme).To(Equal(corev1.URISchemeHTTPS))
			Expect(httpdProxyContainer.LivenessProbe.HTTPGet.Scheme).To(Equal(corev1.URISchemeHTTPS))

			secretDataMap := th.GetSecret(glanceTest.GlanceSingleConfigMapData)
			Expect(secretDataMap).ShouldNot(BeNil())
			myCnf := secretDataMap.Data["my.cnf"]
			Expect(myCnf).To(
				ContainSubstring("[client]\nssl-ca=/etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem\nssl=1"))
		})

		It("TLS Endpoints are created", func() {
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCABundleSecret(glanceTest.CABundleSecret))
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(glanceTest.InternalCertSecret))
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(glanceTest.PublicCertSecret))
			keystone.SimulateKeystoneEndpointReady(glanceTest.GlanceSingle)

			th.ExpectCondition(
				glanceTest.GlanceSingle,
				ConditionGetterFunc(GlanceAPIConditionGetter),
				condition.ServiceConfigReadyCondition,
				corev1.ConditionTrue,
			)

			th.ExpectCondition(
				glanceTest.GlanceSingle,
				ConditionGetterFunc(GlanceAPIConditionGetter),
				condition.TLSInputReadyCondition,
				corev1.ConditionTrue,
			)

			keystoneEndpoint := keystone.GetKeystoneEndpoint(glanceTest.GlanceSingle)
			endpoints := keystoneEndpoint.Spec.Endpoints
			Expect(endpoints).To(HaveKeyWithValue("public", "https://glance-default-public."+glanceTest.Instance.Namespace+".svc:9292"))
			Expect(endpoints).To(HaveKeyWithValue("internal", "https://glance-default-internal."+glanceTest.Instance.Namespace+".svc:9292"))
			th.ExpectCondition(
				glanceTest.GlanceSingle,
				ConditionGetterFunc(GlanceAPIConditionGetter),
				condition.KeystoneEndpointReadyCondition,
				corev1.ConditionTrue,
			)
		})

		It("reconfigures the glance pods when CA changes", func() {
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCABundleSecret(glanceTest.CABundleSecret))
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(glanceTest.InternalCertSecret))
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(glanceTest.PublicCertSecret))
			keystone.SimulateKeystoneEndpointReady(glanceTest.GlanceSingle)
			th.SimulateStatefulSetReplicaReady(glanceTest.GlanceSingle)

			th.ExpectCondition(
				glanceTest.GlanceSingle,
				ConditionGetterFunc(GlanceAPIConditionGetter),
				condition.TLSInputReadyCondition,
				corev1.ConditionTrue,
			)

			// Grab the current config hash
			apiOriginalHash := GetEnvVarValue(
				th.GetStatefulSet(glanceTest.GlanceSingle).Spec.Template.Spec.Containers[0].Env, "CONFIG_HASH", "")
			Expect(apiOriginalHash).NotTo(BeEmpty())

			// Change the content of the CA secret
			th.UpdateSecret(glanceTest.CABundleSecret, "tls-ca-bundle.pem", []byte("DifferentCAData"))

			// Assert that the deployment is updated
			Eventually(func(g Gomega) {
				newHash := GetEnvVarValue(
					th.GetStatefulSet(glanceTest.GlanceSingle).Spec.Template.Spec.Containers[0].Env, "CONFIG_HASH", "")
				g.Expect(newHash).NotTo(BeEmpty())
				g.Expect(newHash).NotTo(Equal(apiOriginalHash))
			}, timeout, interval).Should(Succeed())
		})
	})

	When("A GlanceAPI with TLS is created with service override endpointURL", func() {
		BeforeEach(func() {
			DeferCleanup(infra.DeleteMemcached, infra.CreateMemcached(namespace, glanceTest.MemcachedInstance, memcachedSpec))
			infra.SimulateMemcachedReady(glanceTest.GlanceMemcached)
			DeferCleanup(th.DeleteInstance, CreateDefaultGlance(glanceTest.Instance))
			DeferCleanup(
				mariadb.DeleteDBService,
				mariadb.CreateDBService(
					glanceName.Namespace,
					GetGlance(glanceTest.Instance).Spec.DatabaseInstance,
					corev1.ServiceSpec{
						Ports: []corev1.ServicePort{{Port: 3306}},
					},
				),
			)

			mariadb.CreateMariaDBDatabase(glanceTest.GlanceDatabaseName.Namespace, glanceTest.GlanceDatabaseName.Name, mariadbv1.MariaDBDatabaseSpec{})
			DeferCleanup(k8sClient.Delete, ctx, mariadb.GetMariaDBDatabase(glanceTest.GlanceDatabaseName))

			DeferCleanup(k8sClient.Delete, ctx, th.CreateCABundleSecret(glanceTest.CABundleSecret))
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(glanceTest.InternalCertSecret))
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(glanceTest.PublicCertSecret))
			spec := GetTLSGlanceAPISpec(GlanceAPITypeSingle)
			serviceOverride := map[string]interface{}{}
			serviceOverride["public"] = map[string]interface{}{
				"endpoint":    "public",
				"endpointURL": "https://glance-openstack.apps-crc.testing",
			}
			spec["override"] = map[string]interface{}{
				"service": serviceOverride,
			}
			glance := CreateGlanceAPI(glanceTest.GlanceSingle, spec)
			DeferCleanup(keystone.DeleteKeystoneAPI, keystone.CreateKeystoneAPI(glanceTest.GlanceSingle.Namespace))
			th.SimulateStatefulSetReplicaReady(glanceTest.GlanceSingle)
			keystone.SimulateKeystoneEndpointReady(glanceTest.GlanceSingle)
			DeferCleanup(th.DeleteInstance, glance)
		})
		It("creates KeystoneEndpoint", func() {
			keystoneEndpoint := keystone.GetKeystoneEndpoint(glanceTest.GlanceSingle)
			endpoints := keystoneEndpoint.Spec.Endpoints
			Expect(endpoints).To(HaveKeyWithValue("public", "https://glance-openstack.apps-crc.testing"))
			Expect(endpoints).To(HaveKeyWithValue("internal", "https://glance-default-internal."+glanceTest.Instance.Namespace+".svc:9292"))

			th.ExpectCondition(
				glanceTest.GlanceSingle,
				ConditionGetterFunc(GlanceAPIConditionGetter),
				condition.TLSInputReadyCondition,
				corev1.ConditionTrue,
			)
			th.ExpectCondition(
				glanceTest.GlanceSingle,
				ConditionGetterFunc(GlanceAPIConditionGetter),
				condition.KeystoneEndpointReadyCondition,
				corev1.ConditionTrue,
			)
		})
	})
})
