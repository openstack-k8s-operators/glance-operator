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
	"golang.org/x/exp/maps"

	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"

	glancev1 "github.com/openstack-k8s-operators/glance-operator/api/v1beta1"
	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	batchv1 "k8s.io/api/batch/v1"
)

func GetGlance(name types.NamespacedName) *glancev1.Glance {
	instance := &glancev1.Glance{}
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, name, instance)).Should(Succeed())
	}, timeout, interval).Should(Succeed())
	return instance
}

func GetGlanceAPI(name types.NamespacedName) *glancev1.GlanceAPI {
	instance := &glancev1.GlanceAPI{}
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, name, instance)).Should(Succeed())
	}, timeout, interval).Should(Succeed())
	return instance
}

func GlanceConditionGetter(name types.NamespacedName) condition.Conditions {
	instance := GetGlance(name)
	return instance.Status.Conditions
}

func GlanceAPIConditionGetter(name types.NamespacedName) condition.Conditions {
	instance := GetGlanceAPI(name)
	return instance.Status.Conditions
}

func CreateDefaultGlance(name types.NamespacedName) client.Object {
	raw := map[string]interface{}{
		"apiVersion": "glance.openstack.org/v1beta1",
		"kind":       "Glance",
		"metadata": map[string]interface{}{
			"name":      name.Name,
			"namespace": name.Namespace,
		},
		"spec": map[string]interface{}{
			"keystoneEndpoint": "default",
			"databaseInstance": "openstack",
			"storageRequest":   glanceTest.GlancePVCSize,
			"glanceAPIs": map[string]interface{}{
				"default": GetGlanceAPIDefaultSpec(GlanceAPITypeSingle),
			},
		},
	}
	return th.CreateUnstructured(raw)
}

// GetGlanceEmptySpec - the resulting map is usually assigned to the top-level
// Glance spec
func GetGlanceEmptySpec() map[string]interface{} {
	return map[string]interface{}{
		"keystoneEndpoint": "default",
		"secret":           SecretName,
		"databaseInstance": "openstack",
		"storageRequest":   glanceTest.GlancePVCSize,
		"glanceAPIs":       map[string]interface{}{},
	}
}

func GetGlanceDefaultSpec() map[string]interface{} {
	return map[string]interface{}{
		"keystoneEndpoint": "default",
		"databaseInstance": "openstack",
		"databaseUser":     glanceTest.GlanceDatabaseUser,
		"serviceUser":      glanceName.Name,
		"secret":           SecretName,
		"glanceAPIs":       GetAPIList(),
		"type":             "split",
		"storageRequest":   glanceTest.GlancePVCSize,
	}
}

func GetGlanceDefaultSpecWithQuota() map[string]interface{} {
	return map[string]interface{}{
		"keystoneEndpoint": "default",
		"databaseInstance": "openstack",
		"databaseUser":     glanceTest.GlanceDatabaseUser,
		"serviceUser":      glanceName.Name,
		"secret":           SecretName,
		"glanceAPIs":       GetAPIList(),
		"storageRequest":   glanceTest.GlancePVCSize,
		"quotas":           glanceTest.GlanceQuotas,
	}
}

// By default we're splitting here
func GetAPIList() map[string]interface{} {
	apiList := map[string]interface{}{
		"default": GetDefaultGlanceAPISpec(GlanceAPITypeSingle),
	}
	return apiList
}

func GetGlanceAPIDefaultSpec(apiType APIType) map[string]interface{} {
	return map[string]interface{}{
		"replicas":       1,
		"storageRequest": glanceTest.GlancePVCSize,
	}
}

func CreateGlance(name types.NamespacedName, spec map[string]interface{}) client.Object {

	raw := map[string]interface{}{
		"apiVersion": "glance.openstack.org/v1beta1",
		"kind":       "Glance",
		"metadata": map[string]interface{}{
			"name":      name.Name,
			"namespace": name.Namespace,
		},
		"spec": spec,
	}
	return th.CreateUnstructured(raw)
}

func CreateGlanceAPI(name types.NamespacedName, spec map[string]interface{}) client.Object {
	raw := map[string]interface{}{
		"apiVersion": "glance.openstack.org/v1beta1",
		"kind":       "GlanceAPI",
		"metadata": map[string]interface{}{
			"annotations": map[string]interface{}{
				"keystoneEndpoint": "true",
			},
			"name":      name.Name,
			"namespace": name.Namespace,
		},
		"spec": spec,
	}
	return th.CreateUnstructured(raw)
}

func CreateGlanceSecret(namespace string, name string) *corev1.Secret {
	return th.CreateSecret(
		types.NamespacedName{Namespace: namespace, Name: name},
		map[string][]byte{
			"GlancePassword":         []byte(glanceTest.GlancePassword),
			"GlanceDatabasePassword": []byte(glanceTest.GlancePassword),
		},
	)
}

func GetDefaultGlanceSpec() map[string]interface{} {
	return map[string]interface{}{
		"databaseInstance": "openstack",
		"secret":           SecretName,
		"glanceAPIs":       GetAPIList(),
	}
}

func GetDefaultGlanceAPITemplate(apiType APIType) map[string]interface{} {
	return map[string]interface{}{
		"replicas":       1,
		"containerImage": glanceTest.ContainerImage,
		"serviceAccount": glanceTest.GlanceSA.Name,
		"apiType":        apiType,
		"storageRequest": glanceTest.GlancePVCSize,
	}
}

func GetDefaultGlanceAPISpec(apiType APIType) map[string]interface{} {
	spec := GetDefaultGlanceAPITemplate(apiType)
	maps.Copy(spec, map[string]interface{}{
		"databaseHostname": "openstack",
		"secret":           SecretName,
		"name":             "default",
	})
	return spec
}

func GlanceAPINotExists(name types.NamespacedName) {
	Consistently(func(g Gomega) {
		instance := &glancev1.GlanceAPI{}
		err := k8sClient.Get(ctx, name, instance)
		g.Expect(k8s_errors.IsNotFound(err)).To(BeTrue())
	}, timeout, interval).Should(Succeed())
}

func GlanceAPIExists(name types.NamespacedName) {
	Consistently(func(g Gomega) {
		instance := &glancev1.GlanceAPI{}
		err := k8sClient.Get(ctx, name, instance)
		g.Expect(k8s_errors.IsNotFound(err)).To(BeFalse())
	}, timeout, interval).Should(Succeed())
}

// AssertPVCDoesNotExist ensures the local PVC resource does not exist in a k8s cluster.
func AssertPVCDoesNotExist(name types.NamespacedName) {
	instance := &corev1.PersistentVolumeClaim{}
	Eventually(func(g Gomega) {
		err := th.K8sClient.Get(th.Ctx, name, instance)
		g.Expect(k8s_errors.IsNotFound(err)).To(BeTrue())
	}, th.Timeout, th.Interval).Should(Succeed())
}

// AssertPVCExist ensures the local PVC resource exist in a k8s cluster.
func AssertPVCExist(name types.NamespacedName) {
	instance := &corev1.PersistentVolumeClaim{}
	Eventually(func(g Gomega) {
		err := th.K8sClient.Get(th.Ctx, name, instance)
		g.Expect(k8s_errors.IsNotFound(err)).To(BeFalse())
	}, th.Timeout, th.Interval).Should(Succeed())
}

// AssertCronJobDoesNotExist ensures the CronJob resource does not exist in a k8s cluster.
func AssertCronJobDoesNotExist(name types.NamespacedName) {
	instance := &batchv1.CronJob{}
	Eventually(func(g Gomega) {
		err := th.K8sClient.Get(th.Ctx, name, instance)
		g.Expect(k8s_errors.IsNotFound(err)).To(BeTrue())
	}, th.Timeout, th.Interval).Should(Succeed())
}
