package glance

import (
	"bytes"
	"github.com/openstack-k8s-operators/lib-common/modules/common/util"
	"path/filepath"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"text/template"
)

// GetOwningGlanceName - Given a GlanceAPI (both internal and external)
// object, return the parent Glance object that created it (if any)
func GetOwningGlanceName(instance client.Object) string {
	for _, ownerRef := range instance.GetOwnerReferences() {
		if ownerRef.Kind == "Glance" {
			return ownerRef.Name
		}
	}
	return ""
}

// GetDeploymentConfigData - Process 01-deployment secret
// TODO: this function might be moved to lib-common
func GetDeploymentConfigData(
	deployment map[string]interface{},
	secretName string,
) ([]byte, error) {

	tmplPath, err := util.GetTemplatesPath()
	if err != nil {
		return nil, err
	}
	depFileName := filepath.Join(tmplPath, secretName+".conf")
	tmpl, err := template.ParseFiles(depFileName)
	if err != nil {
		return nil, err
	}
	var d bytes.Buffer
	if err := tmpl.Execute(&d, deployment); err != nil {
		return nil, err
	}
	return d.Bytes(), nil
}
