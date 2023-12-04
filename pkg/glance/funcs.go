package glance

import (
	glancev1 "github.com/openstack-k8s-operators/glance-operator/api/v1beta1"
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
	}
	for _, suffix := range suffixes {
		if strings.Contains(name, suffix) {
			apiName := strings.TrimSuffix(name, "-"+suffix)
			api = apiName[len(prefix):]
		}
	}
	return api
}
