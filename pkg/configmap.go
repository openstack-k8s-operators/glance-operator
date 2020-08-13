package glance

import (
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

func ConfigMap(api *glancev1beta1.GlanceAPI, scheme *runtime.Scheme) *corev1.ConfigMap {
	opts := glanceConfigOptions{"FIXME"}

	cm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      api.Name,
			Namespace: api.Namespace,
			Labels:    GetLabels(api.Name),
		},
		Data: map[string]string{
			"glance-api.conf":     util.ExecuteTemplateFile("glance-api.conf", opts),
			"config.json":         util.ExecuteTemplateFile("config.json", nil),
			"db-sync-config.json": util.ExecuteTemplateFile("db-sync-config.json", nil),
			"logging.conf":        util.ExecuteTemplateFile("logging.conf", nil),
		},
	}
	controllerutil.SetControllerReference(api, cm, scheme)
	return cm
}
