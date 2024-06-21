package glance

import (
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

// glanceSecurityContext - currently used to make sure we don't run db-sync as
// root user
func glanceSecurityContext() *corev1.SecurityContext {
	trueVal := true
	runAsUser := int64(GlanceUID)
	runAsGroup := int64(GlanceGID)

	return &corev1.SecurityContext{
		RunAsUser:    &runAsUser,
		RunAsGroup:   &runAsGroup,
		RunAsNonRoot: &trueVal,
		Capabilities: &corev1.Capabilities{
			Drop: []corev1.Capability{
				"MKNOD",
			},
		},
	}
}
