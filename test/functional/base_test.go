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
	glancev1 "github.com/openstack-k8s-operators/glance-operator/api/v1beta1"
	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func GetCronJob(name types.NamespacedName) *batchv1.CronJob {
	cron := &batchv1.CronJob{}
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, name, cron)).Should(Succeed())
	}, timeout, interval).Should(Succeed())
	return cron
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
	raw := map[string]any{
		"apiVersion": "glance.openstack.org/v1beta1",
		"kind":       "Glance",
		"metadata": map[string]any{
			"name":      name.Name,
			"namespace": name.Namespace,
		},
		"spec": map[string]any{
			"memcachedInstance": "memcached",
			"keystoneEndpoint":  "default",
			"databaseInstance":  "openstack",
			"databaseAccount":   glanceTest.GlanceDatabaseAccount.Name,
			"storage": map[string]any{
				"storageRequest": glanceTest.GlancePVCSize,
			},
			"glanceAPIs": GetAPIList(),
		},
	}
	return th.CreateUnstructured(raw)
}

// GetGlanceEmptySpec - the resulting map is usually assigned to the top-level
// Glance spec
func GetGlanceEmptySpec() map[string]any {
	return map[string]any{
		"keystoneEndpoint":        "default",
		"notificationBusInstance": glanceTest.NotificationsBusInstance,
		"secret":                  SecretName,
		"databaseInstance":        "openstack",
		"storage": map[string]any{
			"storageRequest": glanceTest.GlancePVCSize,
		},
		"glanceAPIs": map[string]any{},
	}
}

func GetGlanceDefaultSpec() map[string]any {
	return map[string]any{
		"keystoneEndpoint":        "default",
		"databaseInstance":        "openstack",
		"databaseAccount":         glanceTest.GlanceDatabaseAccount.Name,
		"serviceUser":             glanceName.Name,
		"secret":                  SecretName,
		"notificationBusInstance": glanceTest.NotificationsBusInstance,
		"glanceAPIs":              GetAPIList(),
		"storage": map[string]any{
			"storageRequest": glanceTest.GlancePVCSize,
		},
	}
}

func GetGlanceDefaultSpecWithQuota() map[string]any {
	return map[string]any{
		"keystoneEndpoint":        "default",
		"databaseInstance":        "openstack",
		"databaseAccount":         glanceTest.GlanceDatabaseAccount.Name,
		"serviceUser":             glanceName.Name,
		"notificationBusInstance": glanceTest.NotificationsBusInstance,
		"secret":                  SecretName,
		"glanceAPIs":              GetAPIList(),
		"storage": map[string]any{
			"storageRequest": glanceTest.GlancePVCSize,
		},
		"quotas":    glanceTest.GlanceQuotas,
		"memcached": glanceTest.MemcachedInstance,
	}
}

// By default we're splitting here
func GetAPIList() map[string]any {
	apiList := map[string]any{
		"default": GetDefaultGlanceAPISpec(GlanceAPITypeSingle),
	}
	return apiList
}

func GetGlanceAPIDefaultSpec() map[string]any {
	return map[string]any{
		"replicas": 1,
		"storage": map[string]any{
			"storageRequest": glanceTest.GlancePVCSize,
		},
		"databaseAccount": glanceTest.GlanceDatabaseAccount.Name,
	}
}

func CreateGlance(name types.NamespacedName, spec map[string]any) client.Object {

	raw := map[string]any{
		"apiVersion": "glance.openstack.org/v1beta1",
		"kind":       "Glance",
		"metadata": map[string]any{
			"name":      name.Name,
			"namespace": name.Namespace,
		},
		"spec": spec,
	}
	return th.CreateUnstructured(raw)
}

func CreateGlanceAPI(name types.NamespacedName, spec map[string]any) client.Object {
	raw := map[string]any{
		"apiVersion": "glance.openstack.org/v1beta1",
		"kind":       "GlanceAPI",
		"metadata": map[string]any{
			"annotations": map[string]any{
				"keystoneEndpoint": "true",
			},
			"name":      name.Name,
			"namespace": name.Namespace,
			"labels":    map[string]string{"api-name": "default"},
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
func GetDefaultGlanceSpec() map[string]any {
	return map[string]any{
		"databaseInstance":        glanceTest.GlanceDatabaseName.Name,
		"databaseAccount":         glanceTest.GlanceDatabaseAccount.Name,
		"secret":                  SecretName,
		"customServiceConfig":     GlanceDummyBackend,
		"notificationBusInstance": glanceTest.NotificationsBusInstance,
		"glanceAPIs":              GetAPIList(),
		"storage": map[string]any{
			"storageRequest": glanceTest.GlancePVCSize,
		},
	}
}

// CreateGlanceAPISpec -
func CreateGlanceAPISpec(apiType APIType) map[string]any {
	spec := map[string]any{
		"replicas":       1,
		"serviceAccount": glanceTest.GlanceSA.Name,
		"containerImage": glanceTest.ContainerImage,
		"storage": map[string]any{
			"storageRequest": glanceTest.GlancePVCSize,
		},
		"apiType":            apiType,
		"name":               "default",
		"databaseHostname":   "openstack",
		"secret":             SecretName,
		"databaseAccount":    glanceTest.GlanceDatabaseAccount.Name,
		"transportURLSecret": glanceTest.RabbitmqSecretName,
	}
	return spec
}

// CreateGlanceAPIWithTopologySpec - It returns a GlanceAPISpec where a
// topology is referenced. It also overrides the top-level parameter of
// the top-level glance controller
func CreateGlanceAPIWithTopologySpec() map[string]any {
	rawSpec := GetDefaultGlanceSpec()
	// Add top-level topologyRef
	rawSpec["topologyRef"] = map[string]any{
		"name": glanceTest.GlanceAPITopologies[0].Name,
	}
	// Override topologyRef for the subCR
	rawSpec["glanceAPIs"] = map[string]any{
		"default": map[string]any{
			"topologyRef": map[string]any{
				"name": glanceTest.GlanceAPITopologies[1].Name,
			},
		},
	}
	return rawSpec
}

// GetDefaultGlanceAPISpec -
func GetDefaultGlanceAPISpec(apiType APIType) map[string]any {
	spec := map[string]any{
		"replicas":       1,
		"containerImage": glanceTest.ContainerImage,
		"serviceAccount": glanceTest.GlanceSA.Name,
		"storage": map[string]any{
			"storageRequest": glanceTest.GlancePVCSize,
		},
		"type":               apiType,
		"name":               "default",
		"databaseHostname":   "openstack",
		"secret":             SecretName,
		"databaseAccount":    glanceTest.GlanceDatabaseAccount.Name,
		"transportURLSecret": glanceTest.RabbitmqSecretName,
	}
	return spec
}

// GetTLSGlanceAPISpec -
func GetTLSGlanceAPISpec(apiType APIType) map[string]any {
	spec := CreateGlanceAPISpec(apiType)
	maps.Copy(spec, map[string]any{
		"databaseHostname":        "openstack",
		"databaseAccount":         glanceTest.GlanceDatabaseAccount.Name,
		"secret":                  SecretName,
		"notificationBusInstance": glanceTest.NotificationsBusInstance,
		"tls": map[string]any{
			"api": map[string]any{
				"internal": map[string]any{
					"secretName": InternalCertSecretName,
				},
				"public": map[string]any{
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

// GetExtraMounts - Utility function that simulates extraMounts pointing
// to a Ceph secret
func GetExtraMounts() []map[string]any {
	return []map[string]any{
		{
			"name":   glanceTest.Instance.Name,
			"region": "az0",
			"extraVol": []map[string]any{
				{
					"extraVolType": GlanceCephExtraMountsSecretName,
					"propagation": []string{
						"GlanceAPI",
					},
					"volumes": []map[string]any{
						{
							"name": GlanceCephExtraMountsSecretName,
							"secret": map[string]any{
								"secretName": GlanceCephExtraMountsSecretName,
							},
						},
					},
					"mounts": []map[string]any{
						{
							"name":      GlanceCephExtraMountsSecretName,
							"mountPath": GlanceCephExtraMountsPath,
							"readOnly":  true,
						},
					},
				},
			},
		},
	}
}

// GetSampleTopologySpec - A sample (and opinionated) Topology Spec used to
// test GlanceAPI
// Note this is just an example that should not be used in production for
// multiple reasons:
// 1. It relies on `service=glance` instead of spreading per GlanceAPI
// 2. It uses ScheduleAnyway as strategy, which is something we might
// want to avoid by default
// 3. Usually a topologySpreadConstraints is used to take care about
// multi AZ, which is not applicable in this context
func GetSampleTopologySpec(
	label string,
) (map[string]any, []corev1.TopologySpreadConstraint) {
	// Build the topology Spec
	topologySpec := map[string]any{
		"topologySpreadConstraints": []map[string]any{
			{
				"maxSkew":           1,
				"topologyKey":       corev1.LabelHostname,
				"whenUnsatisfiable": "ScheduleAnyway",
				"labelSelector": map[string]any{
					"matchLabels": map[string]any{
						"component": label,
					},
				},
			},
		},
	}
	// Build the topologyObj representation
	topologySpecObj := []corev1.TopologySpreadConstraint{
		{
			MaxSkew:           1,
			TopologyKey:       corev1.LabelHostname,
			WhenUnsatisfiable: corev1.ScheduleAnyway,
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"component": label,
				},
			},
		},
	}
	return topologySpec, topologySpecObj
}

// CreateDefaultCinderInstance - Creates a default Cinder CR used as a
// dependency when a Cinder backend is defined in glance
func CreateDefaultCinderInstance(cinderName types.NamespacedName) client.Object {
	raw := map[string]any{
		"apiVersion": "cinder.openstack.org/v1beta1",
		"kind":       "Cinder",
		"metadata": map[string]any{
			"name":      cinderName.Name,
			"namespace": cinderName.Namespace,
		},
	}
	return th.CreateUnstructured(raw)
}

// CreateGlanceMessageBusSecret -
func CreateGlanceMessageBusSecret(namespace string, name string) *corev1.Secret {
	s := th.CreateSecret(
		types.NamespacedName{Namespace: namespace, Name: name},
		map[string][]byte{
			"transport_url": fmt.Appendf(nil, "rabbit://%s/fake", name),
		},
	)
	logger.Info("Secret created", "name", name)
	return s
}
