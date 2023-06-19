package glance

import (
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

// GetEnabledBackends - Given a instance.Spec.CustomServiceConfig object, return
// a list of available stores in form of 'store_id':'backend'
func GetEnabledBackends(customServiceConfig string) []string {
	var available_backends []string
	svcConfigLines := strings.Split(customServiceConfig, "\n")
	for _, line := range svcConfigLines {
		tokenLine := strings.SplitN(strings.TrimSpace(line), "=", 2)
		token := tokenLine[0]

		if token == "" || strings.HasPrefix(token, "#") {
			// Skip blank lines and comments
			continue
		}
		if token == "enabled_backends" {
			backend_token := strings.SplitN(strings.TrimSpace(tokenLine[1]), ",", -1)
			for i := 0; i < len(backend_token); i++ {
				available_backends = append(available_backends, strings.TrimSpace(backend_token[i]))
			}
			break
		}
	}
	return available_backends
}
