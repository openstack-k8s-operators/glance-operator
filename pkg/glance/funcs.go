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
	var availableBackends []string
	svcConfigLines := strings.Split(customServiceConfig, "\n")
	for _, line := range svcConfigLines {
		tokenLine := strings.SplitN(strings.TrimSpace(line), "=", 2)
		token := strings.ReplaceAll(tokenLine[0], " ", "")

		if token == "" || strings.HasPrefix(token, "#") {
			// Skip blank lines and comments
			continue
		}
		if token == "enabled_backends" {
			backendToken := strings.SplitN(strings.TrimSpace(tokenLine[1]), ",", -1)
			for i := 0; i < len(backendToken); i++ {
				availableBackends = append(availableBackends, strings.TrimSpace(backendToken[i]))
			}
			break
		}
	}
	return availableBackends
}
