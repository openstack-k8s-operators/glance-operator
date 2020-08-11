package glance

import (
	glancev1beta1 "github.com/openstack-k8s-operators/glance-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

//Returns the deployment object for the Database
func Pvc(api *glancev1beta1.GlanceAPI, scheme *runtime.Scheme) *corev1.PersistentVolumeClaim {
	pv := &corev1.PersistentVolumeClaim{
		ObjectMeta: v1.ObjectMeta{
			Name:      api.Name,
			Namespace: api.Namespace,
			Labels:    GetLabels(api.Name),
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			StorageClassName: &api.Spec.StorageClass,
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse(api.Spec.StorageRequest),
				},
			},
		},
	}
	controllerutil.SetControllerReference(api, pv, scheme)
	return pv
}
