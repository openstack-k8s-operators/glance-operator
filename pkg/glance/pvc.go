package glance

import (
	"context"
	"fmt"

	glancev1beta1 "github.com/openstack-k8s-operators/glance-operator/api/v1beta1"
	"github.com/openstack-k8s-operators/lib-common/pkg/common"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// Pvc - creates and returns a PVC object for a backing store
func Pvc(ctx context.Context, r common.ReconcilerCommon, api *glancev1beta1.GlanceAPI, labels map[string]string) (*corev1.PersistentVolumeClaim, error) {
	pv := &common.Pvc{
		Name:         api.Name,
		Namespace:    api.Namespace,
		Size:         api.Spec.StorageRequest,
		Labels:       labels,
		StorageClass: api.Spec.StorageClass,
		AccessMode: []corev1.PersistentVolumeAccessMode{
			corev1.ReadWriteMany,
		},
	}

	pvc, op, err := common.CreateOrUpdatePvc(ctx, r, api, pv)

	if op != controllerutil.OperationResultNone {
		r.GetLogger().Info(fmt.Sprintf("PVC %s - %s", pvc.Name, op))
	}

	return pvc, err
}
