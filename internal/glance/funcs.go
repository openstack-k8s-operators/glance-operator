package glance

import (
	"github.com/openstack-k8s-operators/lib-common/modules/common/probes"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	"math"
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

// APISecurityContext -
func APISecurityContext(userID int64, privileged bool) *corev1.SecurityContext {
	return &corev1.SecurityContext{
		AllowPrivilegeEscalation: ptr.To(true),
		RunAsUser:                ptr.To(userID),
		Privileged:               &privileged,
		SeccompProfile: &corev1.SeccompProfile{
			Type: corev1.SeccompProfileTypeRuntimeDefault,
		},
	}
}

// GetDefaultProbesAPI -
func GetDefaultProbesAPI(apiTimeout int) probes.OverrideSpec {
	const failureCount = 3
	period := int32(math.Floor(float64(apiTimeout) / float64(failureCount)))
	// For startup probes, use shorter period for faster startup detection
	startupPeriod := int32(math.Max(5, float64(period)/2))

	// Default values applied to CinderAPI StatefulSets when no
	// overrides are provided
	return probes.OverrideSpec{
		LivenessProbes: &probes.ProbeConf{
			Path:                "/healthcheck",
			TimeoutSeconds:      period,
			PeriodSeconds:       period,
			InitialDelaySeconds: 5,
		},
		ReadinessProbes: &probes.ProbeConf{
			Path:                "/healthcheck",
			TimeoutSeconds:      period,
			PeriodSeconds:       period,
			InitialDelaySeconds: 5,
		},
		StartupProbes: &probes.ProbeConf{
			TimeoutSeconds:      5,
			PeriodSeconds:       startupPeriod,
			InitialDelaySeconds: 5,
			FailureThreshold:    12,
		},
	}
}
