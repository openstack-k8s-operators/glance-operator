package glance

import (
	"fmt"

	glancev1beta1 "github.com/openstack-k8s-operators/glance-operator/api/v1beta1"
	util "github.com/openstack-k8s-operators/lib-common/pkg/util"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"strings"

	"k8s.io/apimachinery/pkg/util/yaml"
)

type databaseOptions struct {
	DatabaseHostname string
	DatabaseName     string
	Secret           string
}

// DatabaseObject func
func DatabaseObject(cr *glancev1beta1.GlanceAPI) (unstructured.Unstructured, error) {
	opts := databaseOptions{cr.Spec.DatabaseHostname, cr.Name, cr.Spec.Secret}

	templatesPath := util.GetTemplatesPath()

	mariadbSchemaTemplate := fmt.Sprintf("%s/%s/internal/mariadb_database.yaml", templatesPath, strings.ToLower(cr.Kind))
	decoder := yaml.NewYAMLOrJSONDecoder(strings.NewReader(util.ExecuteTemplate(mariadbSchemaTemplate, &opts)), 4096)
	u := unstructured.Unstructured{}
	err := decoder.Decode(&u)
	u.SetNamespace(cr.Namespace)
	return u, err
}
