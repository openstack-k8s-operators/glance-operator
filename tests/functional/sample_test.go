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
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/types"
)

// Test the basic glance samples
const SamplesDir = "../../config/samples/"

func ReadSample(sampleFileName string) map[string]interface{} {
	rawSample := make(map[string]interface{})

	bytes, err := os.ReadFile(filepath.Join(SamplesDir, sampleFileName))
	Expect(err).ShouldNot(HaveOccurred())
	Expect(yaml.Unmarshal(bytes, rawSample)).Should(Succeed())

	return rawSample
}

func CreateGlanceFromSample(sampleFileName string, name types.NamespacedName) types.NamespacedName {
	raw := ReadSample(sampleFileName)
	instance := CreateGlance(name, raw["spec"].(map[string]interface{}))
	DeferCleanup(th.DeleteInstance, instance)
	return types.NamespacedName{Name: instance.GetName(), Namespace: instance.GetNamespace()}
}

func CreateGlanceAPIFromSample(sampleFileName string, name types.NamespacedName) types.NamespacedName {
	raw := ReadSample(sampleFileName)
	instance := CreateGlanceAPI(name, raw["spec"].(map[string]interface{}))
	DeferCleanup(th.DeleteInstance, instance)
	return types.NamespacedName{Name: instance.GetName(), Namespace: instance.GetNamespace()}
}

// This is a set of test for our samples. It only validates that the sample
// file has all the required field with proper types. But it does not
// validate that using a sample file will result in a working deployment.
var _ = Describe("Samples", func() {

	When("glance_v1beta1_glance.yaml sample is applied", func() {
		It("Glance is created", func() {
			Eventually(func(g Gomega) {
				name := CreateGlanceFromSample("glance_v1beta1_glance.yaml", glanceTest.Instance)
				glance := GetGlance(name)
				g.Expect(glance.Status.Conditions).To(HaveLen(11))
				g.Expect(glance.Status.DatabaseHostname).To(Equal(""))
				g.Expect(glance.Status.GlanceAPIExternalReadyCount).To(Equal(int32(0)))
				g.Expect(glance.Status.GlanceAPIInternalReadyCount).To(Equal(int32(0)))
			}, timeout, interval).Should(Succeed())
		})
	})
	When("glance_v1beta1_glance_quota.yaml sample is applied", func() {
		It("Glance is created", func() {
			name := CreateGlanceFromSample("glance_v1beta1_glance_quota.yaml", glanceTest.Instance)
			GetGlance(name)
		})
	})
	When("glance_v1beta1_glanceapi.yaml sample is applied", func() {
		It("GlanceAPI is created", func() {
			name := CreateGlanceAPIFromSample("glance_v1beta1_glanceapi.yaml", glanceTest.Instance)
			GetGlanceAPI(name)
		})
	})
})
