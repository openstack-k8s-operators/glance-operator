package glance

import (
	glancev1 "github.com/openstack-k8s-operators/glance-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
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

// GetGlanceAPIName - For a given full glanceAPIName passed as input, this utility
// resolves the name used in the glance CR to identify the API.
func GetGlanceAPIName(name string) string {

	/***
	A given GlanceAPI name can be found in the form:

	+--------------------------------------------------------+
	|   "glance.ServiceName + instance.Name + instance.Type" |
	+--------------------------------------------------------+

	but only "instance.Name" is used to identify the glanceAPI instance in
	the main CR. For this reason we cut the string passed as input and we
	trim both prefix and suffix.

	Example:
	input = "glance-api1-internal"
	output = "api1"
	***/
	var api = ""
	prefix := ServiceName + "-"
	suffixes := []string{
		glancev1.APIInternal,
		glancev1.APIExternal,
		glancev1.APISingle,
		glancev1.APIEdge,
	}
	for _, suffix := range suffixes {
		if strings.Contains(name, suffix) {
			apiName := strings.TrimSuffix(name, "-"+suffix)
			api = apiName[len(prefix):]
		}
	}
	return api
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
