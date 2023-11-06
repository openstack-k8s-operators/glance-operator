package glance

import (
	"fmt"
	glancev1 "github.com/openstack-k8s-operators/glance-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetPvc - creates and returns a PVC object for a backing store
func GetPvc(api *glancev1.GlanceAPI, labels map[string]string, pvcType PvcType) corev1.PersistentVolumeClaim {
	// By default we point to a local storage pvc request
	// that will be customized in case the pvc is requested
	// for cache purposes
	requestSize := api.Spec.StorageRequest
	pvcName := ServiceName
	if pvcType == "cache" {
		requestSize = api.Spec.ImageCacheSize
		// append -cache to avoid confusion when listing PVCs
		pvcName = fmt.Sprintf("%s-cache", ServiceName)
	}
	pvc := corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pvcName,
			Namespace: api.Namespace,
			Labels:    labels,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse(requestSize),
				},
			},
			StorageClassName: &api.Spec.StorageClass,
		},
	}
	return pvc
}
