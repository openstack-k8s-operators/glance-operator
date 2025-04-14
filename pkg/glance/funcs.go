package glance

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
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

// dbSyncSecurityContext - currently used to make sure we don't run db-sync as
// root user
func dbSyncSecurityContext() *corev1.SecurityContext {

	return &corev1.SecurityContext{
		RunAsUser:  ptr.To(GlanceUID),
		RunAsGroup: ptr.To(GlanceGID),
		Capabilities: &corev1.Capabilities{
			Drop: []corev1.Capability{
				"MKNOD",
			},
		},
		SeccompProfile: &corev1.SeccompProfile{
			Type: corev1.SeccompProfileTypeRuntimeDefault,
		},
	}
}

// BaseSecurityContext - currently used to make sure we don't run cronJob and Log
// Pods as root user, and we drop privileges and Capabilities we don't need
func BaseSecurityContext() *corev1.SecurityContext {

	return &corev1.SecurityContext{
		RunAsUser:                ptr.To(GlanceUID),
		RunAsGroup:               ptr.To(GlanceGID),
		RunAsNonRoot:             ptr.To(true),
		AllowPrivilegeEscalation: ptr.To(false),
		Capabilities: &corev1.Capabilities{
			Drop: []corev1.Capability{
				"ALL",
			},
		},
		SeccompProfile: &corev1.SeccompProfile{
			Type: corev1.SeccompProfileTypeRuntimeDefault,
		},
	}
}

// HttpdSecurityContext -
func HttpdSecurityContext(privileged bool) *corev1.SecurityContext {
	return &corev1.SecurityContext{
		Capabilities: &corev1.Capabilities{
			Drop: []corev1.Capability{
				"MKNOD",
			},
		},
		RunAsUser:                ptr.To(GlanceUID),
		RunAsGroup:               ptr.To(GlanceGID),
		Privileged:               &privileged,
		AllowPrivilegeEscalation: ptr.To(true),
		SeccompProfile: &corev1.SeccompProfile{
			Type: corev1.SeccompProfileTypeRuntimeDefault,
		},
	}
}
