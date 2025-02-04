package glanceapi

import (
	glancev1 "github.com/openstack-k8s-operators/glance-operator/api/v1beta1"

	glance "github.com/openstack-k8s-operators/glance-operator/pkg/glance"
	"github.com/openstack-k8s-operators/lib-common/modules/common/endpoint"
	"github.com/openstack-k8s-operators/lib-common/modules/common/labels"
	"github.com/openstack-k8s-operators/lib-common/modules/common/service"

	"fmt"

	corev1 "k8s.io/api/core/v1"
)

// GetGlanceEndpoints - returns the glance endpoints map based on the apiType of the glance-api
// default is split, in case of single both internal and public endpoint get returned
func GetGlanceEndpoints(apiType string) map[service.Endpoint]endpoint.Data {
	glanceEndpoints := map[service.Endpoint]endpoint.Data{}
	// edge - we don't need a public endpoint
	if apiType == glancev1.APIEdge {
		glanceEndpoints[service.EndpointInternal] = endpoint.Data{
			Port: glance.GlanceInternalPort,
		}
		return glanceEndpoints
	}
	// split
	if apiType == glancev1.APIInternal {
		glanceEndpoints[service.EndpointInternal] = endpoint.Data{
			Port: glance.GlanceInternalPort,
		}
	} else {
		glanceEndpoints[service.EndpointPublic] = endpoint.Data{
			Port: glance.GlancePublicPort,
		}
	}
	// if we're not splitting the API and deploying a single instance, we have
	// to add both internal and public endpoints
	if apiType == glancev1.APISingle {
		glanceEndpoints[service.EndpointInternal] = endpoint.Data{
			Port: glance.GlanceInternalPort,
		}
	}
	return glanceEndpoints
}

// GetGlanceAPIPodAffinity - Returns a corev1.Affinity reference for a given GlanceAPI
func GetGlanceAPIPodAffinity(instance *glancev1.GlanceAPI) *corev1.Affinity {
	// The PodAffinity is used to co-locate a glanceAPI Pod and an associated
	// imageCache cronJob. This allows to mount the RWO PVC and successfully
	// run pruner and cleaner tools against the mountpoint
	labelSelector := labels.GetSingleLabelSelector(
		glance.GlanceAPIName,
		fmt.Sprintf("%s-%s-%s", glance.ServiceName, instance.APIName(), instance.Spec.APIType),
	)
	return &corev1.Affinity{
		PodAffinity: &corev1.PodAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
				{
					LabelSelector: &labelSelector,
					// usually corev1.LabelHostname "kubernetes.io/hostname"
					// https://github.com/kubernetes/api/blob/master/core/v1/well_known_labels.go#L20
					TopologyKey: corev1.LabelHostname,
				},
			},
		},
	}
}
