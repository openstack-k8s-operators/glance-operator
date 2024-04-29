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
	"golang.org/x/exp/maps"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/gomega" //revive:disable:dot-imports
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
			"memcachedInstance": "memcached",
			"keystoneEndpoint":  "default",
			"databaseInstance":  "openstack",
			"databaseAccount":   glanceTest.GlanceDatabaseAccount.Name,
			"storageDetails": map[string]interface{}{
				"storageRequest": glanceTest.GlancePVCSize,
			},
			"glanceAPIs": GetAPIList(),
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
		"storageDetails": map[string]interface{}{
			"storageRequest": glanceTest.GlancePVCSize,
		},
		"glanceAPIs": map[string]interface{}{},
	}
}

func GetGlanceDefaultSpec() map[string]interface{} {
	return map[string]interface{}{
		"keystoneEndpoint": "default",
		"databaseInstance": "openstack",
		"databaseAccount":  glanceTest.GlanceDatabaseAccount.Name,
		"serviceUser":      glanceName.Name,
		"secret":           SecretName,
		"glanceAPIs":       GetAPIList(),
		"storageDetails": map[string]interface{}{
			"storageRequest": glanceTest.GlancePVCSize,
		},
	}
}

func GetGlanceDefaultSpecWithQuota() map[string]interface{} {
	return map[string]interface{}{
		"keystoneEndpoint": "default",
		"databaseInstance": "openstack",
		"databaseAccount":  glanceTest.GlanceDatabaseAccount.Name,
		"serviceUser":      glanceName.Name,
		"secret":           SecretName,
		"glanceAPIs":       GetAPIList(),
		"storageDetails": map[string]interface{}{
			"storageRequest": glanceTest.GlancePVCSize,
		},
		"quotas":    glanceTest.GlanceQuotas,
		"memcached": glanceTest.MemcachedInstance,
	}
}

// By default we're splitting here
func GetAPIList() map[string]interface{} {
	apiList := map[string]interface{}{
		"default": GetDefaultGlanceAPISpec(GlanceAPITypeSingle),
	}
	return apiList
}

func GetGlanceAPIDefaultSpec() map[string]interface{} {
	return map[string]interface{}{
		"replicas": 1,
		"storageDetails": map[string]interface{}{
			"storageRequest": glanceTest.GlancePVCSize,
		},
		"databaseAccount": glanceTest.GlanceDatabaseAccount.Name,
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
			"GlancePassword": []byte(glanceTest.GlancePassword),
		},
	)
}

// GetDefaultGlanceSpec - It returns a default API built for testing purposes
func GetDefaultGlanceSpec() map[string]interface{} {
	return map[string]interface{}{
		"databaseInstance": "openstack",
		"secret":           SecretName,
		"databaseAccount":  glanceTest.GlanceDatabaseAccount.Name,
		"glanceAPIs":       GetAPIList(),
	}
}

// CreateGlanceAPISpec -
func CreateGlanceAPISpec(apiType APIType) map[string]interface{} {
	spec := map[string]interface{}{
		"replicas":       1,
		"serviceAccount": glanceTest.GlanceSA.Name,
		"containerImage": glanceTest.ContainerImage,
		"storageDetails": map[string]interface{}{
			"storageRequest": glanceTest.GlancePVCSize,
		},
		"apiType":          apiType,
		"name":             "default",
		"databaseHostname": "openstack",
		"secret":           SecretName,
		"databaseAccount":  glanceTest.GlanceDatabaseAccount.Name,
	}
	return spec
}

// GetDefaultGlanceAPISpec -
func GetDefaultGlanceAPISpec(apiType APIType) map[string]interface{} {
	spec := map[string]interface{}{
		"replicas":       1,
		"containerImage": glanceTest.ContainerImage,
		"serviceAccount": glanceTest.GlanceSA.Name,
		"storageDetails": map[string]interface{}{
			"storageRequest": glanceTest.GlancePVCSize,
		},
		"type":             apiType,
		"name":             "default",
		"databaseHostname": "openstack",
		"secret":           SecretName,
		"databaseAccount":  glanceTest.GlanceDatabaseAccount.Name,
	}
	return spec
}

// GetTLSGlanceAPISpec -
func GetTLSGlanceAPISpec(apiType APIType) map[string]interface{} {
	spec := CreateGlanceAPISpec(apiType)
	maps.Copy(spec, map[string]interface{}{
		"databaseHostname": "openstack",
		"databaseAccount":  glanceTest.GlanceDatabaseAccount.Name,
		"secret":           SecretName,
		"tls": map[string]interface{}{
			"api": map[string]interface{}{
				"internal": map[string]interface{}{
					"secretName": InternalCertSecretName,
				},
				"public": map[string]interface{}{
					"secretName": PublicCertSecretName,
				},
			},
			"caBundleSecretName": CABundleSecretName,
		},
	})
	return spec
}

// GlanceAPINotExists - Asserts the GlanceAPI does not exist
func GlanceAPINotExists(name types.NamespacedName) {
	Consistently(func(g Gomega) {
		instance := &glancev1.GlanceAPI{}
		err := k8sClient.Get(ctx, name, instance)
		g.Expect(k8s_errors.IsNotFound(err)).To(BeTrue())
	}, timeout, interval).Should(Succeed())
}

// GlanceAPIExists - Asserts the GlanceAPI exist
func GlanceAPIExists(name types.NamespacedName) {
	Consistently(func(g Gomega) {
		instance := &glancev1.GlanceAPI{}
		err := k8sClient.Get(ctx, name, instance)
		g.Expect(k8s_errors.IsNotFound(err)).To(BeFalse())
	}, timeout, interval).Should(Succeed())
}

// AssertPVCDoesNotExist ensures the local PVC resource does not exist in a k8s
// cluster.
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

// AssertCronJobDoesNotExist ensures the CronJob resource does not exist in a
// k8s cluster.
func AssertCronJobDoesNotExist(name types.NamespacedName) {
	instance := &batchv1.CronJob{}
	Eventually(func(g Gomega) {
		err := th.K8sClient.Get(th.Ctx, name, instance)
		g.Expect(k8s_errors.IsNotFound(err)).To(BeTrue())
	}, th.Timeout, th.Interval).Should(Succeed())
}

// GetDummyBackend - Utility function that simulates a customServiceConfig
// where a Ceph backend has been set
func GetDummyBackend() string {
	section := "[DEFAULT]"
	dummyBackend := "enabled_backends=backend1:rbd"
	return fmt.Sprintf("%s\n%s", section, dummyBackend)
}
