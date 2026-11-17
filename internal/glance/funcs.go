package glance

import (
	"math"
	"strings"

	glancev1 "github.com/openstack-k8s-operators/glance-operator/api/v1beta1"
	"github.com/openstack-k8s-operators/lib-common/modules/common/probes"
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

// GetDefaultProbesAPI - Calculate dynamic probe configuration based on apiTimeout
// This ensures probe timing aligns with HAProxy and Apache timeout settings
func GetDefaultProbesAPI(apiTimeout int) probes.OverrideSpec {
	const failureCount = 3
	const minTimeout = 5

	// 0.3 instead of doing apiTimeout / failureCount that can drive to
	// to integer overflow in the conversion between int and int32
	period := int32(math.Floor(float64(apiTimeout) * 0.3))
	// Timeout should be less than period to avoid overlapping probes
	// Use 80% of period as a safe margin
	timeout := int32(math.Max(float64(period)*0.8, minTimeout))

	// For startup probes, use shorter period for faster startup detection
	// Cap at reasonable values to avoid excessive startup times
	startupPeriod := int32(math.Max(5, math.Min(10, float64(period)/2)))

	// Default values applied to GlanceAPI StatefulSets when no
	// overrides are provided
	return probes.OverrideSpec{
		LivenessProbes: &probes.ProbeConf{
			Path:                "/healthcheck",
			TimeoutSeconds:      timeout,
			PeriodSeconds:       period,
			InitialDelaySeconds: minTimeout,
			FailureThreshold:    failureCount,
		},
		ReadinessProbes: &probes.ProbeConf{
			Path:                "/healthcheck",
			TimeoutSeconds:      timeout,
			PeriodSeconds:       period,
			InitialDelaySeconds: minTimeout,
			FailureThreshold:    failureCount,
		},
		StartupProbes: &probes.ProbeConf{
			TimeoutSeconds:      minTimeout,
			PeriodSeconds:       startupPeriod,
			InitialDelaySeconds: minTimeout,
			FailureThreshold:    12,
		},
	}
}

// GetFilesystemStoreDatadirs - return the filesystem_store_datadir of every
// file store in enabled_backends, deduplicated and in enabled_backends order
func GetFilesystemStoreDatadirs(customServiceConfig string) []string {
	var datadirs []string
	seen := map[string]struct{}{}

	for _, backend := range glancev1.GetEnabledBackends(customServiceConfig) {
		// a backend is expressed as 'store_id':'store_type', and the store_id
		// is required: it is the config section the datadir is read from
		storeID, storeType, found := strings.Cut(backend, ":")
		if !found || storeID == "" || storeType != "file" {
			continue
		}
		// each store reads the option from its own section
		datadir, ok := glancev1.GetCustomServiceConfigValue(
			customServiceConfig, storeID, "filesystem_store_datadir")
		if !ok {
			datadir = DefaultFilesystemStoreDatadir
		}
		if _, ok := seen[datadir]; ok {
			continue
		}
		seen[datadir] = struct{}{}
		datadirs = append(datadirs, datadir)
	}
	return datadirs
}
