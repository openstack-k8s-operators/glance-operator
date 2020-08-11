package glance

import (
	glancev1beta1 "github.com/openstack-k8s-operators/glance-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// Service func
func Service(api *glancev1beta1.GlanceAPI, scheme *runtime.Scheme) *corev1.Service {

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      api.Name,
			Namespace: api.Namespace,
			Labels:    GetLabels(api.Name),
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"app": "glance"},
			Ports: []corev1.ServicePort{
				{Name: "api", Port: 9292, Protocol: corev1.ProtocolTCP},
			},
		},
	}
	controllerutil.SetControllerReference(api, svc, scheme)
	return svc
}
