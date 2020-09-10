package glance

import (
	"path/filepath"

	glancev1beta1 "github.com/openstack-k8s-operators/glance-operator/api/v1beta1"

	util "github.com/openstack-k8s-operators/lib-common/pkg/util"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type glanceConfigOptions struct {
	KeystoneEndpoint string
}

// ScriptsConfigMap - scripts config map
func ScriptsConfigMap(api *glancev1beta1.GlanceAPI, scheme *runtime.Scheme) *corev1.ConfigMap {
	opts := glanceConfigOptions{"FIXME"}

	// get templates base path, either running local or deployed as container
	templatesPath := util.GetTemplatesPath()

	// get all scripts templates which are in ../templesPath/api.Kind/bin
	templatesFiles := util.GetAllTemplates(templatesPath, api.Kind, "bin")

	data := make(map[string]string)
	// render all template files
	for _, file := range templatesFiles {
		data[filepath.Base(file)] = util.ExecuteTemplate(file, opts)
	}

	cm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      api.Name + "-scripts",
			Namespace: api.Namespace,
		},
		Data: data,
	}
	controllerutil.SetControllerReference(api, cm, scheme)

	return cm
}

// ConfigMap - config map containing mandatory auto rendered config files for the service
func ConfigMap(api *glancev1beta1.GlanceAPI, scheme *runtime.Scheme) *corev1.ConfigMap {
	opts := glanceConfigOptions{"FIXME"}

	// get templates base path, either running local or deployed as container
	templatesPath := util.GetTemplatesPath()

	// get all scripts templates which are in ../templesPath/api.Kind/config
	templatesFiles := util.GetAllTemplates(templatesPath, api.Kind, "config")

	data := make(map[string]string)
	// render all template files
	for _, file := range templatesFiles {
		data[filepath.Base(file)] = util.ExecuteTemplate(file, opts)
	}

	cm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      api.Name + "-config-data",
			Namespace: api.Namespace,
			Labels:    GetLabels(api.Name),
		},
		Data: data,
	}
	controllerutil.SetControllerReference(api, cm, scheme)

	return cm
}

// CustomConfigMap - config map used by the user to customize the service
func CustomConfigMap(api *glancev1beta1.GlanceAPI, scheme *runtime.Scheme) *corev1.ConfigMap {

	cm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      api.Name + "-config-data-custom",
			Namespace: api.Namespace,
			Labels:    GetLabels(api.Name),
		},
		Data: map[string]string{},
	}
	controllerutil.SetControllerReference(api, cm, scheme)
	return cm
}
