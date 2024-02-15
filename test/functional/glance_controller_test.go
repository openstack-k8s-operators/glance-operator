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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/openstack-k8s-operators/lib-common/modules/common/test/helpers"

	corev1 "k8s.io/api/core/v1"

	glancev1 "github.com/openstack-k8s-operators/glance-operator/api/v1beta1"
	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	util "github.com/openstack-k8s-operators/lib-common/modules/common/util"
)

var _ = Describe("Glance controller", func() {
	When("Glance is created", func() {
		BeforeEach(func() {
			DeferCleanup(th.DeleteInstance, CreateDefaultGlance(glanceName))
		})
		It("initializes the status fields", func() {
			Eventually(func(g Gomega) {
				glance := GetGlance(glanceName)
				g.Expect(glance.Status.Conditions).To(HaveLen(12))
				g.Expect(glance.Status.DatabaseHostname).To(Equal(""))
				g.Expect(glance.Status.APIEndpoints).To(BeEmpty())
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
		It("initializes Spec fields", func() {
			Glance := GetGlance(glanceTest.Instance)
			Expect(Glance.Spec.DatabaseInstance).Should(Equal("openstack"))
			Expect(Glance.Spec.DatabaseUser).Should(Equal(glanceTest.GlanceDatabaseUser))
			Expect(Glance.Spec.ServiceUser).Should(Equal(glanceTest.GlanceServiceUser))
			// No Keystone Quota is present, check the default is 0
			Expect(Glance.Spec.Quotas.ImageCountUpload).To(Equal(int(0)))
			Expect(Glance.Spec.Quotas.ImageSizeTotal).To(Equal(int(0)))
			Expect(Glance.Spec.Quotas.ImageCountTotal).To(Equal(int(0)))
			Expect(Glance.Spec.Quotas.ImageStageTotal).To(Equal(int(0)))
		})
		It("should have a finalizer", func() {
			// the reconciler loop adds the finalizer so we have to wait for
			// it to run
			Eventually(func() []string {
				return GetGlance(glanceTest.Instance).Finalizers
			}, timeout, interval).Should(ContainElement("Glance"))
		})
		It("should not create a config map", func() {
			Eventually(func() []corev1.ConfigMap {
				return th.ListConfigMaps(glanceTest.GlanceConfigMapData.Name).Items
			}, timeout, interval).Should(BeEmpty())
		})
		It("creates service account, role and rolebindig", func() {
			th.ExpectCondition(
				glanceName,
				ConditionGetterFunc(GlanceConditionGetter),
				condition.ServiceAccountReadyCondition,
				corev1.ConditionTrue,
			)
			sa := th.GetServiceAccount(glanceTest.GlanceSA)

			th.ExpectCondition(
				glanceName,
				ConditionGetterFunc(GlanceConditionGetter),
				condition.RoleReadyCondition,
				corev1.ConditionTrue,
			)
			role := th.GetRole(glanceTest.GlanceRole)
			Expect(role.Rules).To(HaveLen(2))
			Expect(role.Rules[0].Resources).To(Equal([]string{"securitycontextconstraints"}))
			Expect(role.Rules[1].Resources).To(Equal([]string{"pods"}))

			th.ExpectCondition(
				glanceName,
				ConditionGetterFunc(GlanceConditionGetter),
				condition.RoleBindingReadyCondition,
				corev1.ConditionTrue,
			)
			binding := th.GetRoleBinding(glanceTest.GlanceRoleBinding)
			Expect(binding.RoleRef.Name).To(Equal(role.Name))
			Expect(binding.Subjects).To(HaveLen(1))
			Expect(binding.Subjects[0].Name).To(Equal(sa.Name))
		})
		It("defaults the containerImages", func() {
			glance := GetGlance(glanceName)
			// (Updates) We let the Glance webhooks override the top-level CR
			// ContainerImage, but we pass an override for each glanceAPI
			// instance, so we can manage them independently
			Expect(glance.Spec.ContainerImage).To(Equal(glancev1.GlanceAPIContainerImage))
			for _, api := range glance.Spec.GlanceAPIs {
				// We expect the containerImage enforced in the Spec by GlanceAPI()
				// function
				Expect(api.ContainerImage).To(Equal(glanceTest.ContainerImage))
			}
		})
		It("should not have a pvc yet", func() {
			AssertPVCDoesNotExist(glanceTest.Instance)
		})
		It("dbPurge cronJob does not exist yet", func() {
			AssertCronJobDoesNotExist(glanceTest.Instance)
		})
	})
	When("Glance DB is created", func() {
		BeforeEach(func() {
			DeferCleanup(th.DeleteInstance, CreateGlance(glanceTest.Instance, GetGlanceDefaultSpec()))
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
		})
		It("Should set DBReady Condition and set DatabaseHostname Status when DB is Created", func() {
			mariadb.SimulateMariaDBAccountCompleted(glanceTest.Instance)
			mariadb.SimulateMariaDBDatabaseCompleted(glanceTest.Instance)
			th.SimulateJobSuccess(glanceTest.GlanceDBSync)
			Glance := GetGlance(glanceTest.Instance)
			Expect(Glance.Status.DatabaseHostname).To(Equal(fmt.Sprintf("hostname-for-openstack.%s.svc", namespace)))
			th.ExpectCondition(
				glanceName,
				ConditionGetterFunc(GlanceConditionGetter),
				condition.DBReadyCondition,
				corev1.ConditionTrue,
			)
			th.ExpectCondition(
				glanceName,
				ConditionGetterFunc(GlanceConditionGetter),
				condition.DBSyncReadyCondition,
				corev1.ConditionFalse,
			)
		})
		It("Should fail if db-sync job fails when DB is Created", func() {
			mariadb.SimulateMariaDBAccountCompleted(glanceTest.Instance)
			mariadb.SimulateMariaDBDatabaseCompleted(glanceTest.Instance)
			th.SimulateJobFailure(glanceTest.GlanceDBSync)
			th.ExpectCondition(
				glanceTest.Instance,
				ConditionGetterFunc(GlanceConditionGetter),
				condition.DBReadyCondition,
				corev1.ConditionTrue,
			)
			th.ExpectCondition(
				glanceTest.Instance,
				ConditionGetterFunc(GlanceConditionGetter),
				condition.DBSyncReadyCondition,
				corev1.ConditionFalse,
			)
		})
		It("Does not create GlanceAPI", func() {
			GlanceAPINotExists(glanceTest.GlanceInternal)
		})
	})
	When("Glance DB is created and db-sync Job succeeded", func() {
		BeforeEach(func() {
			DeferCleanup(th.DeleteInstance, CreateGlance(glanceTest.Instance, GetGlanceDefaultSpec()))
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
			mariadb.SimulateMariaDBAccountCompleted(glanceTest.Instance)
			mariadb.SimulateMariaDBDatabaseCompleted(glanceTest.Instance)
			th.SimulateJobSuccess(glanceTest.GlanceDBSync)
			keystoneAPI := keystone.CreateKeystoneAPI(glanceTest.Instance.Namespace)
			DeferCleanup(keystone.DeleteKeystoneAPI, keystoneAPI)
			keystone.SimulateKeystoneServiceReady(glanceTest.Instance)
		})
		It("Glance DB is Ready and db-sync reports ReadyCondition", func() {
			th.ExpectCondition(
				glanceTest.Instance,
				ConditionGetterFunc(GlanceConditionGetter),
				condition.DBReadyCondition,
				corev1.ConditionTrue,
			)
			th.ExpectCondition(
				glanceTest.Instance,
				ConditionGetterFunc(GlanceConditionGetter),
				condition.DBSyncReadyCondition,
				corev1.ConditionTrue,
			)
		})
		It("GlanceAPI CR is created", func() {
			GlanceAPIExists(glanceTest.GlanceSingle)
		})
	})
	When("Glance CR is created without container images defined", func() {
		BeforeEach(func() {
			// GlanceEmptySpec is used to provide a standard Glance CR where no
			// field is customized
			DeferCleanup(th.DeleteInstance, CreateGlance(glanceTest.Instance, GetGlanceEmptySpec()))
		})
		It("has the expected container image defaults", func() {
			glanceDefault := GetGlance(glanceTest.Instance)
			for _, api := range glanceDefault.Spec.GlanceAPIs {
				Expect(api.ContainerImage).To(Equal(util.GetEnvVar("RELATED_IMAGE_GLANCE_API_IMAGE_URL_DEFAULT", glancev1.GlanceAPIContainerImage)))
			}
		})
		It("has a dummy backend set when and empty spec is passed", func() {
			glanceDefault := GetGlance(glanceTest.Instance)
			Expect(glanceDefault.Spec.CustomServiceConfig).To((ContainSubstring(GlanceDummyBackend)))
		})

	})
	When("All the Resources are ready", func() {
		BeforeEach(func() {
			DeferCleanup(th.DeleteInstance, CreateGlance(glanceTest.Instance, GetGlanceDefaultSpec()))
			// Get Default GlanceAPI
			DeferCleanup(
				mariadb.DeleteDBService,
				mariadb.CreateDBService(
					glanceTest.Instance.Namespace,
					GetGlance(glanceName).Spec.DatabaseInstance,
					corev1.ServiceSpec{
						Ports: []corev1.ServicePort{{Port: 3306}},
					},
				),
			)
			DeferCleanup(keystone.DeleteKeystoneAPI, keystone.CreateKeystoneAPI(glanceTest.Instance.Namespace))
			mariadb.SimulateMariaDBAccountCompleted(glanceTest.Instance)
			mariadb.SimulateMariaDBDatabaseCompleted(glanceTest.Instance)
			th.SimulateJobSuccess(glanceTest.GlanceDBSync)
			keystone.SimulateKeystoneServiceReady(glanceTest.Instance)
			keystone.SimulateKeystoneEndpointReady(glanceTest.GlanceSingle)
		})
		It("Creates glanceAPI", func() {
			GlanceAPIExists(glanceTest.GlanceSingle)
		})
		It("Assert Services are created", func() {
			// Both glance-public and glance-internal svc are created regardless
			// if we split behind the scenes
			th.AssertServiceExists(glanceTest.GlancePublicSvc)
			th.AssertServiceExists(glanceTest.GlanceInternalSvc)
		})
		It("should not have a cache pvc (no imageCacheSize provided)", func() {
			AssertPVCDoesNotExist(glanceTest.GlanceCache)
		})
	})
	When("Glance CR is deleted", func() {
		BeforeEach(func() {
			DeferCleanup(th.DeleteInstance, CreateGlance(glanceTest.Instance, GetGlanceDefaultSpec()))
			DeferCleanup(
				mariadb.DeleteDBService,
				mariadb.CreateDBService(
					glanceTest.Instance.Namespace,
					GetGlance(glanceName).Spec.DatabaseInstance,
					corev1.ServiceSpec{
						Ports: []corev1.ServicePort{{Port: 3306}},
					},
				),
			)
			DeferCleanup(keystone.DeleteKeystoneAPI, keystone.CreateKeystoneAPI(glanceTest.Instance.Namespace))
			mariadb.SimulateMariaDBAccountCompleted(glanceTest.Instance)
			mariadb.SimulateMariaDBDatabaseCompleted(glanceTest.Instance)
			th.SimulateJobSuccess(glanceTest.GlanceDBSync)
			keystone.SimulateKeystoneServiceReady(glanceTest.Instance)
			keystone.SimulateKeystoneEndpointReady(glanceTest.GlanceSingle)
		})
		It("removes the finalizers from the Glance DB", func() {
			mDB := mariadb.GetMariaDBDatabase(glanceTest.Instance)
			Expect(mDB.Finalizers).To(ContainElement("Glance"))
			th.DeleteInstance(GetGlance(glanceTest.Instance))
		})
	})
	When("Glance CR instance is built with NAD", func() {
		BeforeEach(func() {
			nad := th.CreateNetworkAttachmentDefinition(glanceTest.InternalAPINAD)
			DeferCleanup(th.DeleteInstance, nad)

			serviceOverride := map[string]interface{}{}
			serviceOverride["internal"] = map[string]interface{}{
				"metadata": map[string]map[string]string{
					"annotations": {
						"metallb.universe.tf/address-pool":    "osp-internalapi",
						"metallb.universe.tf/allow-shared-ip": "osp-internalapi",
						"metallb.universe.tf/loadBalancerIPs": "internal-lb-ip-1,internal-lb-ip-2",
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
			rawSpec := map[string]interface{}{
				"storageRequest":      glanceTest.GlancePVCSize,
				"secret":              SecretName,
				"databaseInstance":    "openstack",
				"keystoneEndpoint":    "default",
				"customServiceConfig": GlanceDummyBackend,
				"glanceAPIs": map[string]interface{}{
					"default": map[string]interface{}{
						"containerImage":     glancev1.GlanceAPIContainerImage,
						"networkAttachments": []string{"internalapi"},
						"override": map[string]interface{}{
							"service": serviceOverride,
						},
					},
				},
			}
			DeferCleanup(th.DeleteInstance, CreateGlance(glanceTest.Instance, rawSpec))
			DeferCleanup(
				mariadb.DeleteDBService,
				mariadb.CreateDBService(
					glanceTest.Instance.Namespace,
					GetGlance(glanceName).Spec.DatabaseInstance,
					corev1.ServiceSpec{
						Ports: []corev1.ServicePort{{Port: 3306}},
					},
				),
			)
			mariadb.SimulateMariaDBAccountCompleted(glanceTest.Instance)
			mariadb.SimulateMariaDBDatabaseCompleted(glanceTest.Instance)
			th.SimulateJobSuccess(glanceTest.GlanceDBSync)
			keystoneAPI := keystone.CreateKeystoneAPI(glanceTest.Instance.Namespace)
			DeferCleanup(keystone.DeleteKeystoneAPI, keystoneAPI)
			keystoneAPIName := keystone.GetKeystoneAPI(keystoneAPI)
			keystoneAPIName.Status.APIEndpoints["internal"] = "http://keystone-internal-openstack.testing"
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Status().Update(ctx, keystoneAPIName.DeepCopy())).Should(Succeed())
			}, timeout, interval).Should(Succeed())
			keystone.SimulateKeystoneServiceReady(glanceTest.Instance)
		})
		It("Check the resulting endpoints of the generated sub-CRs", func() {
			th.SimulateStatefulSetReplicaReadyWithPods(
				glanceTest.GlanceInternalStatefulSet,
				map[string][]string{glanceName.Namespace + "/internalapi": {"10.0.0.1"}},
			)
			th.SimulateStatefulSetReplicaReadyWithPods(
				glanceTest.GlanceExternalStatefulSet,
				map[string][]string{glanceName.Namespace + "/internalapi": {"10.0.0.1"}},
			)
			// Retrieve the generated resources and the two internal/external
			// instances that are split behind the scenes
			glance := GetGlance(glanceTest.Instance)
			internalAPI := GetGlanceAPI(glanceTest.GlanceInternal)
			externalAPI := GetGlanceAPI(glanceTest.GlanceExternal)
			// Check GlanceAPI(s): we expect the two instances (internal/external)
			// to have the same NADs as we mirror the deployment

			for _, glanceAPI := range glance.Spec.GlanceAPIs {
				Expect(internalAPI.Spec.NetworkAttachments).To(Equal(glanceAPI.NetworkAttachments))
				Expect(externalAPI.Spec.NetworkAttachments).To(Equal(glanceAPI.NetworkAttachments))
			}
		})
	})
})
