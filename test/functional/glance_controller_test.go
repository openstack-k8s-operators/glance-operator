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

	//revive:disable-next-line:dot-imports
	. "github.com/openstack-k8s-operators/lib-common/modules/common/test/helpers"

	glancev1 "github.com/openstack-k8s-operators/glance-operator/api/v1beta1"
	"github.com/openstack-k8s-operators/glance-operator/pkg/glance"
	memcachedv1 "github.com/openstack-k8s-operators/infra-operator/apis/memcached/v1beta1"
	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	util "github.com/openstack-k8s-operators/lib-common/modules/common/util"
	mariadb_test "github.com/openstack-k8s-operators/mariadb-operator/api/test/helpers"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ptr "k8s.io/utils/ptr"
)

var _ = Describe("Glance controller", func() {
	var memcachedSpec memcachedv1.MemcachedSpec

	BeforeEach(func() {
		memcachedSpec = memcachedv1.MemcachedSpec{
			MemcachedSpecCore: memcachedv1.MemcachedSpecCore{
				Replicas: ptr.To[int32](3),
			},
		}
	})

	When("Glance is created", func() {
		BeforeEach(func() {
			DeferCleanup(th.DeleteInstance, CreateDefaultGlance(glanceName))
			DeferCleanup(infra.DeleteMemcached, infra.CreateMemcached(namespace, glanceTest.MemcachedInstance, memcachedSpec))
			infra.SimulateMemcachedReady(glanceTest.GlanceMemcached)
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
			th.ExpectCondition(
				glanceName,
				ConditionGetterFunc(GlanceConditionGetter),
				condition.InputReadyCondition,
				corev1.ConditionFalse,
			)
		})
		It("initializes Spec fields", func() {
			Glance := GetGlance(glanceTest.Instance)
			Expect(Glance.Spec.DatabaseInstance).Should(Equal("openstack"))
			Expect(Glance.Spec.DatabaseAccount).Should(Equal(glanceTest.GlanceDatabaseAccount.Name))
			Expect(Glance.Spec.ServiceUser).Should(Equal(glanceTest.GlanceServiceUser))
			Expect(Glance.Spec.MemcachedInstance).Should(Equal(glanceTest.MemcachedInstance))
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
			}, timeout, interval).Should(ContainElement("openstack.org/glance"))
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
			DeferCleanup(infra.DeleteMemcached, infra.CreateMemcached(namespace, glanceTest.MemcachedInstance, memcachedSpec))
			infra.SimulateMemcachedReady(glanceTest.GlanceMemcached)
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
			mariadb.SimulateMariaDBDatabaseCompleted(glanceTest.GlanceDatabaseName)
			mariadb.SimulateMariaDBAccountCompleted(glanceTest.GlanceDatabaseAccount)
			th.SimulateJobSuccess(glanceTest.GlanceDBSync)
			Glance := GetGlance(glanceTest.Instance)
			Expect(Glance.Status.DatabaseHostname).To(Equal(fmt.Sprintf("hostname-for-openstack.%s.svc", namespace)))

			secretDataMap := th.GetSecret(glanceTest.GlanceConfigMapData)
			Expect(secretDataMap).ShouldNot(BeNil())
			myCnf := secretDataMap.Data["my.cnf"]
			Expect(myCnf).To(
				ContainSubstring("[client]\nssl=0"))

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
			mariadb.SimulateMariaDBDatabaseCompleted(glanceTest.GlanceDatabaseName)
			mariadb.SimulateMariaDBAccountCompleted(glanceTest.GlanceDatabaseAccount)
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
			DeferCleanup(infra.DeleteMemcached, infra.CreateMemcached(namespace, glanceTest.MemcachedInstance, memcachedSpec))
			infra.SimulateMemcachedReady(glanceTest.GlanceMemcached)
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
			mariadb.SimulateMariaDBDatabaseCompleted(glanceTest.GlanceDatabaseName)
			mariadb.SimulateMariaDBAccountCompleted(glanceTest.GlanceDatabaseAccount)
			th.SimulateJobSuccess(glanceTest.GlanceDBSync)
			keystoneAPI := keystone.CreateKeystoneAPI(glanceTest.Instance.Namespace)
			DeferCleanup(keystone.DeleteKeystoneAPI, keystoneAPI)
			keystone.SimulateKeystoneServiceReady(glanceTest.KeystoneService)
		})
		It("Glance DB is Ready and db-sync reports ReadyCondition", func() {
			th.ExpectCondition(
				glanceTest.Instance,
				ConditionGetterFunc(GlanceConditionGetter),
				condition.DBReadyCondition,
				corev1.ConditionTrue,
			)
			th.ExpectCondition(glanceTest.Instance,
				ConditionGetterFunc(GlanceConditionGetter),
				condition.DBSyncReadyCondition,
				corev1.ConditionTrue,
			)
		})
	})
	When("Glance CR is created without container images defined", func() {
		BeforeEach(func() {
			// GlanceEmptySpec is used to provide a standard Glance CR where no
			// field is customized
			DeferCleanup(th.DeleteInstance, CreateGlance(glanceTest.Instance, GetGlanceEmptySpec()))
			DeferCleanup(infra.DeleteMemcached, infra.CreateMemcached(namespace, glanceTest.MemcachedInstance, memcachedSpec))
			infra.SimulateMemcachedReady(glanceTest.GlanceMemcached)
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
			DeferCleanup(infra.DeleteMemcached, infra.CreateMemcached(namespace, glanceTest.MemcachedInstance, memcachedSpec))
			infra.SimulateMemcachedReady(glanceTest.GlanceMemcached)
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
			mariadb.SimulateMariaDBDatabaseCompleted(glanceTest.GlanceDatabaseName)
			mariadb.SimulateMariaDBAccountCompleted(glanceTest.GlanceDatabaseAccount)
			th.SimulateJobSuccess(glanceTest.GlanceDBSync)
			keystone.SimulateKeystoneServiceReady(glanceTest.KeystoneService)
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
		It("configures DB Purge job", func() {
			Eventually(func(g Gomega) {
				glance := GetGlance(glanceTest.Instance)
				cron := GetCronJob(glanceTest.DBPurgeCronJob)
				g.Expect(cron.Spec.Schedule).To(Equal(glance.Spec.DBPurge.Schedule))
			}, timeout, interval).Should(Succeed())
		})
		It("update DB Purge job", func() {
			Eventually(func(g Gomega) {
				glance := GetGlance(glanceTest.Instance)
				glance.Spec.DBPurge.Schedule = "*/30 * * * *"
				g.Expect(k8sClient.Update(ctx, glance)).To(Succeed())
			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				glance := GetGlance(glanceTest.Instance)
				cron := GetCronJob(glanceTest.DBPurgeCronJob)
				g.Expect(cron.Spec.Schedule).To(Equal(glance.Spec.DBPurge.Schedule))
			}, timeout, interval).Should(Succeed())
		})
	})
	When("GlanceCR is created with nodeSelector", func() {
		BeforeEach(func() {
			DeferCleanup(infra.DeleteMemcached, infra.CreateMemcached(namespace, glanceTest.MemcachedInstance, memcachedSpec))
			infra.SimulateMemcachedReady(glanceTest.GlanceMemcached)

			spec := GetGlanceDefaultSpec()
			spec["nodeSelector"] = map[string]interface{}{
				"foo": "bar",
			}
			DeferCleanup(th.DeleteInstance, CreateGlance(glanceTest.Instance, spec))
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
			mariadb.SimulateMariaDBDatabaseCompleted(glanceTest.GlanceDatabaseName)
			mariadb.SimulateMariaDBAccountCompleted(glanceTest.GlanceDatabaseAccount)
			th.SimulateJobSuccess(glanceTest.GlanceDBSync)
			keystone.SimulateKeystoneServiceReady(glanceTest.KeystoneService)
			keystone.SimulateKeystoneEndpointReady(glanceTest.GlanceSingle)
		})
		It("sets nodeSelector in resource specs", func() {
			Eventually(func(g Gomega) {
				g.Expect(th.GetJob(glanceTest.GlanceDBSync).Spec.Template.Spec.NodeSelector).To(Equal(map[string]string{"foo": "bar"}))
				g.Expect(th.GetStatefulSet(glanceTest.GlanceSingle).Spec.Template.Spec.NodeSelector).To(Equal(map[string]string{"foo": "bar"}))
				g.Expect(GetCronJob(glanceTest.DBPurgeCronJob).Spec.JobTemplate.Spec.Template.Spec.NodeSelector).To(Equal(map[string]string{"foo": "bar"}))

			}, timeout, interval).Should(Succeed())
		})
		It("updates nodeSelector in resource specs when changed", func() {
			Eventually(func(g Gomega) {
				g.Expect(th.GetJob(glanceTest.GlanceDBSync).Spec.Template.Spec.NodeSelector).To(Equal(map[string]string{"foo": "bar"}))
				g.Expect(th.GetStatefulSet(glanceTest.GlanceSingle).Spec.Template.Spec.NodeSelector).To(Equal(map[string]string{"foo": "bar"}))
				g.Expect(GetCronJob(glanceTest.DBPurgeCronJob).Spec.JobTemplate.Spec.Template.Spec.NodeSelector).To(Equal(map[string]string{"foo": "bar"}))

			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				glance := GetGlance(glanceTest.Instance)
				newNodeSelector := map[string]string{
					"foo2": "bar2",
				}
				glance.Spec.NodeSelector = &newNodeSelector
				g.Expect(k8sClient.Update(ctx, glance)).Should(Succeed())
			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				th.SimulateJobSuccess(glanceTest.GlanceDBSync)
				g.Expect(th.GetJob(glanceTest.GlanceDBSync).Spec.Template.Spec.NodeSelector).To(Equal(map[string]string{"foo2": "bar2"}))
				g.Expect(th.GetStatefulSet(glanceTest.GlanceSingle).Spec.Template.Spec.NodeSelector).To(Equal(map[string]string{"foo2": "bar2"}))
				g.Expect(GetCronJob(glanceTest.DBPurgeCronJob).Spec.JobTemplate.Spec.Template.Spec.NodeSelector).To(Equal(map[string]string{"foo2": "bar2"}))

			}, timeout, interval).Should(Succeed())
		})
		It("removes nodeSelector from resource specs when cleared", func() {
			Eventually(func(g Gomega) {
				g.Expect(th.GetJob(glanceTest.GlanceDBSync).Spec.Template.Spec.NodeSelector).To(Equal(map[string]string{"foo": "bar"}))
				g.Expect(th.GetStatefulSet(glanceTest.GlanceSingle).Spec.Template.Spec.NodeSelector).To(Equal(map[string]string{"foo": "bar"}))
				g.Expect(GetCronJob(glanceTest.DBPurgeCronJob).Spec.JobTemplate.Spec.Template.Spec.NodeSelector).To(Equal(map[string]string{"foo": "bar"}))

			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				glance := GetGlance(glanceTest.Instance)
				emptyNodeSelector := map[string]string{}
				glance.Spec.NodeSelector = &emptyNodeSelector
				g.Expect(k8sClient.Update(ctx, glance)).Should(Succeed())
			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				th.SimulateJobSuccess(glanceTest.GlanceDBSync)
				g.Expect(th.GetJob(glanceTest.GlanceDBSync).Spec.Template.Spec.NodeSelector).To(BeNil())
				g.Expect(th.GetStatefulSet(glanceTest.GlanceSingle).Spec.Template.Spec.NodeSelector).To(BeNil())
				g.Expect(GetCronJob(glanceTest.DBPurgeCronJob).Spec.JobTemplate.Spec.Template.Spec.NodeSelector).To(BeNil())

			}, timeout, interval).Should(Succeed())
		})
		It("removes nodeSelector from resource specs when nilled", func() {
			Eventually(func(g Gomega) {
				g.Expect(th.GetJob(glanceTest.GlanceDBSync).Spec.Template.Spec.NodeSelector).To(Equal(map[string]string{"foo": "bar"}))
				g.Expect(th.GetStatefulSet(glanceTest.GlanceSingle).Spec.Template.Spec.NodeSelector).To(Equal(map[string]string{"foo": "bar"}))
				g.Expect(GetCronJob(glanceTest.DBPurgeCronJob).Spec.JobTemplate.Spec.Template.Spec.NodeSelector).To(Equal(map[string]string{"foo": "bar"}))

			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				glance := GetGlance(glanceTest.Instance)
				glance.Spec.NodeSelector = nil
				g.Expect(k8sClient.Update(ctx, glance)).Should(Succeed())
			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				th.SimulateJobSuccess(glanceTest.GlanceDBSync)
				g.Expect(th.GetJob(glanceTest.GlanceDBSync).Spec.Template.Spec.NodeSelector).To(BeNil())
				g.Expect(th.GetStatefulSet(glanceTest.GlanceSingle).Spec.Template.Spec.NodeSelector).To(BeNil())
				g.Expect(GetCronJob(glanceTest.DBPurgeCronJob).Spec.JobTemplate.Spec.Template.Spec.NodeSelector).To(BeNil())

			}, timeout, interval).Should(Succeed())
		})
		It("allows nodeSelector GlanceAPI override", func() {
			Eventually(func(g Gomega) {
				g.Expect(th.GetJob(glanceTest.GlanceDBSync).Spec.Template.Spec.NodeSelector).To(Equal(map[string]string{"foo": "bar"}))
				g.Expect(th.GetStatefulSet(glanceTest.GlanceSingle).Spec.Template.Spec.NodeSelector).To(Equal(map[string]string{"foo": "bar"}))
				g.Expect(GetCronJob(glanceTest.DBPurgeCronJob).Spec.JobTemplate.Spec.Template.Spec.NodeSelector).To(Equal(map[string]string{"foo": "bar"}))
			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				glance := GetGlance(glanceTest.Instance)
				apiNodeSelector := map[string]string{
					"foo": "api",
				}
				glanceAPI := glance.Spec.GlanceAPIs["default"]
				glanceAPI.NodeSelector = &apiNodeSelector
				glance.Spec.GlanceAPIs["default"] = glanceAPI
				g.Expect(k8sClient.Update(ctx, glance)).Should(Succeed())
			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				g.Expect(th.GetJob(glanceTest.GlanceDBSync).Spec.Template.Spec.NodeSelector).To(Equal(map[string]string{"foo": "bar"}))
				g.Expect(th.GetStatefulSet(glanceTest.GlanceSingle).Spec.Template.Spec.NodeSelector).To(Equal(map[string]string{"foo": "api"}))
				g.Expect(GetCronJob(glanceTest.DBPurgeCronJob).Spec.JobTemplate.Spec.Template.Spec.NodeSelector).To(Equal(map[string]string{"foo": "bar"}))
			}, timeout, interval).Should(Succeed())
		})
		It("allows nodeSelector GlanceAPI override to empty", func() {
			Eventually(func(g Gomega) {
				g.Expect(th.GetJob(glanceTest.GlanceDBSync).Spec.Template.Spec.NodeSelector).To(Equal(map[string]string{"foo": "bar"}))
				g.Expect(th.GetStatefulSet(glanceTest.GlanceSingle).Spec.Template.Spec.NodeSelector).To(Equal(map[string]string{"foo": "bar"}))
				g.Expect(GetCronJob(glanceTest.DBPurgeCronJob).Spec.JobTemplate.Spec.Template.Spec.NodeSelector).To(Equal(map[string]string{"foo": "bar"}))
			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				glance := GetGlance(glanceTest.Instance)
				apiNodeSelector := map[string]string{}
				glanceAPI := glance.Spec.GlanceAPIs["default"]
				glanceAPI.NodeSelector = &apiNodeSelector
				glance.Spec.GlanceAPIs["default"] = glanceAPI
				g.Expect(k8sClient.Update(ctx, glance)).Should(Succeed())
			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				g.Expect(th.GetJob(glanceTest.GlanceDBSync).Spec.Template.Spec.NodeSelector).To(Equal(map[string]string{"foo": "bar"}))
				g.Expect(th.GetStatefulSet(glanceTest.GlanceSingle).Spec.Template.Spec.NodeSelector).To(BeNil())
				g.Expect(GetCronJob(glanceTest.DBPurgeCronJob).Spec.JobTemplate.Spec.Template.Spec.NodeSelector).To(Equal(map[string]string{"foo": "bar"}))
			}, timeout, interval).Should(Succeed())
		})
	})
	When("Glance CR is deleted", func() {
		BeforeEach(func() {
			DeferCleanup(infra.DeleteMemcached, infra.CreateMemcached(namespace, glanceTest.MemcachedInstance, memcachedSpec))
			infra.SimulateMemcachedReady(glanceTest.GlanceMemcached)
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
			mariadb.SimulateMariaDBDatabaseCompleted(glanceTest.GlanceDatabaseName)
			mariadb.SimulateMariaDBAccountCompleted(glanceTest.GlanceDatabaseAccount)
			th.SimulateJobSuccess(glanceTest.GlanceDBSync)
			keystone.SimulateKeystoneServiceReady(glanceTest.KeystoneService)
			keystone.SimulateKeystoneEndpointReady(glanceTest.GlanceSingle)
		})
		It("removes the finalizers from the Glance DB", func() {
			mDB := mariadb.GetMariaDBDatabase(glanceTest.GlanceDatabaseName)
			Expect(mDB.Finalizers).To(ContainElement("openstack.org/glance"))
			th.DeleteInstance(GetGlance(glanceTest.Instance))
		})
	})
	When("Glance CR instance is built with NAD", func() {
		BeforeEach(func() {
			DeferCleanup(infra.DeleteMemcached, infra.CreateMemcached(namespace, glanceTest.MemcachedInstance, memcachedSpec))
			infra.SimulateMemcachedReady(glanceTest.GlanceMemcached)
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
				"storage": map[string]interface{}{
					"storageRequest": glanceTest.GlancePVCSize,
				},
				"storageRequest":      glanceTest.GlancePVCSize,
				"secret":              SecretName,
				"databaseInstance":    "openstack",
				"databaseAccount":     glanceTest.GlanceDatabaseAccount.Name,
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
			mariadb.SimulateMariaDBDatabaseCompleted(glanceTest.GlanceDatabaseName)
			mariadb.SimulateMariaDBAccountCompleted(glanceTest.GlanceDatabaseAccount)
			th.SimulateJobSuccess(glanceTest.GlanceDBSync)
			keystoneAPI := keystone.CreateKeystoneAPI(glanceTest.Instance.Namespace)
			DeferCleanup(keystone.DeleteKeystoneAPI, keystoneAPI)
			keystoneAPIName := keystone.GetKeystoneAPI(keystoneAPI)
			keystoneAPIName.Status.APIEndpoints["internal"] = "http://keystone-internal-openstack.testing"
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Status().Update(ctx, keystoneAPIName.DeepCopy())).Should(Succeed())
			}, timeout, interval).Should(Succeed())
			keystone.SimulateKeystoneServiceReady(glanceTest.KeystoneService)
			th.SimulateLoadBalancerServiceIP(glanceTest.GlanceInternalSvc)
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

	When("Glance CR instance is built with ExtraMounts", func() {
		BeforeEach(func() {
			DeferCleanup(infra.DeleteMemcached, infra.CreateMemcached(namespace, glanceTest.MemcachedInstance, memcachedSpec))
			infra.SimulateMemcachedReady(glanceTest.GlanceMemcached)

			rawSpec := map[string]interface{}{
				"storage": map[string]interface{}{
					"storageRequest": glanceTest.GlancePVCSize,
				},
				"storageRequest":      glanceTest.GlancePVCSize,
				"secret":              SecretName,
				"databaseInstance":    glanceTest.GlanceDatabaseName.Name,
				"databaseAccount":     glanceTest.GlanceDatabaseAccount.Name,
				"customServiceConfig": GlanceDummyBackend,
				"extraMounts":         GetExtraMounts(),
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
			mariadb.SimulateMariaDBDatabaseCompleted(glanceTest.GlanceDatabaseName)
			mariadb.SimulateMariaDBAccountCompleted(glanceTest.GlanceDatabaseAccount)
			th.SimulateJobSuccess(glanceTest.GlanceDBSync)
			keystoneAPI := keystone.CreateKeystoneAPI(glanceTest.Instance.Namespace)
			DeferCleanup(keystone.DeleteKeystoneAPI, keystoneAPI)
			keystone.SimulateKeystoneServiceReady(glanceTest.KeystoneService)
		})
		It("Check the extraMounts of the resulting StatefulSets", func() {
			th.SimulateStatefulSetReplicaReady(glanceTest.GlanceInternalStatefulSet)
			th.SimulateStatefulSetReplicaReady(glanceTest.GlanceExternalStatefulSet)
			// Retrieve the generated resources and the two internal/external
			// instances that are split behind the scenes
			ssInternal := th.GetStatefulSet(glanceTest.GlanceInternalStatefulSet)
			ssExternal := th.GetStatefulSet(glanceTest.GlanceExternalStatefulSet)

			for _, ss := range []*appsv1.StatefulSet{ssInternal, ssExternal} {
				// Check the resulting deployment fields
				Expect(ss.Spec.Template.Spec.Volumes).To(HaveLen(5))
				Expect(ss.Spec.Template.Spec.Containers).To(HaveLen(3))
				// Get the glance-api container
				container := ss.Spec.Template.Spec.Containers[2]
				// Fail if glance-api doesn't have the right number of VolumeMounts
				// entries
				Expect(container.VolumeMounts).To(HaveLen(7))
				// Inspect VolumeMounts and make sure we have the Ceph MountPath
				// provided through extraMounts
				for _, vm := range container.VolumeMounts {
					if vm.Name == "ceph" {
						Expect(vm.MountPath).To(
							ContainSubstring(GlanceCephExtraMountsPath))
					}
				}
			}
		})
	})

	When("Glance CR references a Topology", func() {
		BeforeEach(func() {
			DeferCleanup(infra.DeleteMemcached, infra.CreateMemcached(namespace, glanceTest.MemcachedInstance, memcachedSpec))
			infra.SimulateMemcachedReady(glanceTest.GlanceMemcached)

			// Build the topology Spec
			topologySpec := GetSampleTopologySpec()
			// Create Test Topologies
			for _, t := range glanceTest.GlanceAPITopologies {
				CreateTopology(t, topologySpec)
			}
			rawSpec := GetGlanceEmptySpec()
			// Reference a top-level topology
			rawSpec["topologyRef"] = map[string]interface{}{
				"name": glanceTest.GlanceAPITopologies[0].Name,
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
			mariadb.SimulateMariaDBDatabaseCompleted(glanceTest.GlanceDatabaseName)
			mariadb.SimulateMariaDBAccountCompleted(glanceTest.GlanceDatabaseAccount)
			th.SimulateJobSuccess(glanceTest.GlanceDBSync)
			keystoneAPI := keystone.CreateKeystoneAPI(glanceTest.Instance.Namespace)
			DeferCleanup(keystone.DeleteKeystoneAPI, keystoneAPI)
			keystone.SimulateKeystoneServiceReady(glanceTest.KeystoneService)
		})

		It("Check the Topology has been applied to the resulting StatefulSets", func() {
			th.SimulateStatefulSetReplicaReady(glanceTest.GlanceInternalStatefulSet)
			th.SimulateStatefulSetReplicaReady(glanceTest.GlanceExternalStatefulSet)
			Eventually(func(g Gomega) {
				internalAPI := GetGlanceAPI(glanceTest.GlanceInternal)
				externalAPI := GetGlanceAPI(glanceTest.GlanceExternal)
				g.Expect(internalAPI.Status.LastAppliedTopology).To(Equal(glanceTest.GlanceAPITopologies[0].Name))
				g.Expect(externalAPI.Status.LastAppliedTopology).To(Equal(glanceTest.GlanceAPITopologies[0].Name))
			}, timeout, interval).Should(Succeed())
		})

		It("Update the Topology reference", func() {
			Eventually(func(g Gomega) {
				glance := GetGlance(glanceTest.Instance)
				glance.Spec.TopologyRef.Name = glanceTest.GlanceAPITopologies[1].Name
				g.Expect(k8sClient.Update(ctx, glance)).To(Succeed())
			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				internalAPI := GetGlanceAPI(glanceTest.GlanceInternal)
				externalAPI := GetGlanceAPI(glanceTest.GlanceExternal)
				g.Expect(internalAPI.Status.LastAppliedTopology).To(Equal(glanceTest.GlanceAPITopologies[1].Name))
				g.Expect(externalAPI.Status.LastAppliedTopology).To(Equal(glanceTest.GlanceAPITopologies[1].Name))
			}, timeout, interval).Should(Succeed())
		})

		It("Remove the Topology reference", func() {
			Eventually(func(g Gomega) {
				glance := GetGlance(glanceTest.Instance)
				// Remove the TopologyRef from the existing Glance .Spec
				glance.Spec.TopologyRef = nil
				g.Expect(k8sClient.Update(ctx, glance)).To(Succeed())
			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				internalAPI := GetGlanceAPI(glanceTest.GlanceInternal)
				externalAPI := GetGlanceAPI(glanceTest.GlanceExternal)
				g.Expect(internalAPI.Status.LastAppliedTopology).Should(BeEmpty())
				g.Expect(externalAPI.Status.LastAppliedTopology).Should(BeEmpty())
			}, timeout, interval).Should(Succeed())

			// Check the statefulSet has a default Affinity and no TopologySpreadConstraints:
			// Affinity is applied by DistributePods function provided by lib-common, while
			// TopologySpreadConstraints is part of the sample Topology used to test Glance
			Eventually(func(g Gomega) {
				ssInternal := th.GetStatefulSet(glanceTest.GlanceInternalStatefulSet)
				ssExternal := th.GetStatefulSet(glanceTest.GlanceExternalStatefulSet)
				for _, ss := range []*appsv1.StatefulSet{ssInternal, ssExternal} {
					// Check the resulting deployment fields
					g.Expect(ss.Spec.Template.Spec.Affinity).ToNot(BeNil())
					g.Expect(ss.Spec.Template.Spec.TopologySpreadConstraints).To(BeNil())
				}
			}, timeout, interval).Should(Succeed())
		})
	})

	// Run MariaDBAccount suite tests.  these are pre-packaged ginkgo tests
	// that exercise standard account create / update patterns that should be
	// common to all controllers that ensure MariaDBAccount CRs.
	mariadbSuite := &mariadb_test.MariaDBTestHarness{
		PopulateHarness: func(harness *mariadb_test.MariaDBTestHarness) {
			harness.Setup(
				"Glance",
				glanceName.Namespace,
				glance.DatabaseName,
				"openstack.org/glance",
				mariadb,
				timeout,
				interval,
			)
		},
		// Generate a fully running Glance service given an accountName
		// needs to make it all the way to the end where the mariadb finalizers
		// are removed from unused accounts since that's part of what we are testing
		SetupCR: func(accountName types.NamespacedName) {

			memcachedSpec = memcachedv1.MemcachedSpec{
				MemcachedSpecCore: memcachedv1.MemcachedSpecCore{
					Replicas: ptr.To[int32](3),
				},
			}

			DeferCleanup(infra.DeleteMemcached, infra.CreateMemcached(namespace, glanceTest.MemcachedInstance, memcachedSpec))
			infra.SimulateMemcachedReady(glanceTest.GlanceMemcached)

			spec := GetGlanceDefaultSpec()
			spec["databaseAccount"] = accountName.Name

			DeferCleanup(th.DeleteInstance, CreateGlance(glanceTest.Instance, spec))
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

			DeferCleanup(keystone.DeleteKeystoneAPI, keystone.CreateKeystoneAPI(glanceTest.Instance.Namespace))
			mariadb.SimulateMariaDBAccountCompleted(accountName)
			mariadb.SimulateMariaDBDatabaseCompleted(glanceTest.GlanceDatabaseName)
			th.SimulateJobSuccess(glanceTest.GlanceDBSync)
			keystone.SimulateKeystoneServiceReady(glanceTest.KeystoneService)
			keystone.SimulateKeystoneEndpointReady(glanceTest.GlanceSingle)
			GlanceAPIExists(glanceTest.GlanceSingle)
		},
		// Change the account name in the service to a new name
		UpdateAccount: func(newAccountName types.NamespacedName) {

			Eventually(func(g Gomega) {
				glance := GetGlance(glanceTest.Instance)
				glance.Spec.DatabaseAccount = newAccountName.Name
				g.Expect(th.K8sClient.Update(ctx, glance)).Should(Succeed())
			}, timeout, interval).Should(Succeed())

		},
		// delete the service, allowing finalizer removal tests
		DeleteCR: func() {
			th.DeleteInstance(GetGlance(glanceTest.Instance))
		},
	}

	mariadbSuite.RunBasicSuite()
})
