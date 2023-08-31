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

// Package functional implements the envTest coverage for glance-operator
package functional

import (
	"fmt"
	"k8s.io/apimachinery/pkg/types"
)

// GlanceTestData is the data structure used to provide input data to envTest
type APIType string

const (
	GlanceAPITypeInternal APIType = "internal"
	GlanceAPITypeExternal APIType = "external"
)

type GlanceTestData struct {
	ContainerImage              string
	GlanceDatabaseUser          string
	GlancePassword              string
	GlanceServiceUser           string
	GlancePVCSize               string
	GlancePort                  string
	GlanceQuotas                map[string]interface{}
	Instance                    types.NamespacedName
	GlanceInternal              types.NamespacedName
	GlanceExternal              types.NamespacedName
	GlanceRole                  types.NamespacedName
	GlanceRoleBinding           types.NamespacedName
	GlanceSA                    types.NamespacedName
	GlanceDBSync                types.NamespacedName
	GlancePublicRoute           types.NamespacedName
	GlanceInternalRoute         types.NamespacedName
	GlanceService               types.NamespacedName
	GlanceConfigMapData         types.NamespacedName
	GlanceInternalConfigMapData types.NamespacedName
	GlanceConfigMapScripts      types.NamespacedName
	GlanceInternalAPI           types.NamespacedName
	GlanceExternalAPI           types.NamespacedName
	InternalAPINAD              types.NamespacedName
}

// GetGlanceTestData is a function that initialize the GlanceTestData
// used in the test
func GetGlanceTestData(glanceName types.NamespacedName) GlanceTestData {

	m := glanceName
	return GlanceTestData{
		Instance: m,

		GlanceDBSync: types.NamespacedName{
			Namespace: glanceName.Namespace,
			Name:      fmt.Sprintf("%s-db-sync", glanceName.Name),
		},
		GlanceInternalAPI: types.NamespacedName{
			Namespace: glanceName.Namespace,
			Name:      fmt.Sprintf("%s-internal-api", glanceName.Name),
		},
		GlanceExternalAPI: types.NamespacedName{
			Namespace: glanceName.Namespace,
			Name:      fmt.Sprintf("%s-external-api", glanceName.Name),
		},
		GlanceInternal: types.NamespacedName{
			Namespace: glanceName.Namespace,
			Name:      fmt.Sprintf("%s-internal", glanceName.Name),
		},
		GlanceExternal: types.NamespacedName{
			Namespace: glanceName.Namespace,
			Name:      fmt.Sprintf("%s-external", glanceName.Name),
		},
		GlanceRole: types.NamespacedName{
			Namespace: glanceName.Namespace,
			Name:      fmt.Sprintf("glance-%s-role", glanceName.Name),
		},
		GlanceRoleBinding: types.NamespacedName{
			Namespace: glanceName.Namespace,
			Name:      fmt.Sprintf("glance-%s-rolebinding", glanceName.Name),
		},
		GlanceSA: types.NamespacedName{
			Namespace: glanceName.Namespace,
			Name:      fmt.Sprintf("glance-%s", glanceName.Name),
		},
		GlanceConfigMapData: types.NamespacedName{
			Namespace: glanceName.Namespace,
			Name:      fmt.Sprintf("%s-%s", glanceName.Name, "config-data"),
		},
		GlanceConfigMapScripts: types.NamespacedName{
			Namespace: glanceName.Namespace,
			Name:      fmt.Sprintf("%s-%s", glanceName.Name, "scripts"),
		},
		GlanceInternalConfigMapData: types.NamespacedName{
			Namespace: glanceName.Namespace,
			Name:      fmt.Sprintf("%s-%s", glanceName.Name, "internal-config-data"),
		},
		GlanceService: types.NamespacedName{
			Namespace: glanceName.Namespace,
			Name:      "image",
		},
		// Also used to identify GlanceKeystoneService
		GlanceInternalRoute: types.NamespacedName{
			Namespace: glanceName.Namespace,
			Name:      fmt.Sprintf("%s-internal", glanceName.Name),
		},
		GlancePublicRoute: types.NamespacedName{
			Namespace: glanceName.Namespace,
			Name:      fmt.Sprintf("%s-public", glanceName.Name),
		},
		GlanceQuotas: map[string]interface{}{
			"imageSizeTotal":   1000,
			"imageStageTotal":  1000,
			"imageCountUpload": 100,
			"imageCountTotal":  100,
		},
		InternalAPINAD: types.NamespacedName{
			Namespace: glanceName.Namespace,
			Name:      "internalapi",
		},
		GlanceDatabaseUser: "glance",
		// Password used for both db and service
		GlancePassword:    "12345678",
		GlanceServiceUser: "glance",
		GlancePVCSize:     "10G",
		ContainerImage:    "test://glance",
		GlancePort:        "9292",
	}
}
