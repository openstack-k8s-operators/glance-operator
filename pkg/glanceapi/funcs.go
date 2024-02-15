package glanceapi

import (
	glancev1 "github.com/openstack-k8s-operators/glance-operator/api/v1beta1"

	glance "github.com/openstack-k8s-operators/glance-operator/pkg/glance"
	"github.com/openstack-k8s-operators/lib-common/modules/common/endpoint"
	"github.com/openstack-k8s-operators/lib-common/modules/common/service"
)

// GetGlanceEndpoints - returns the glance endpoints map based on the apiType of the glance-api
// default is split, in case of single both internal and public endpoint get returned
func GetGlanceEndpoints(apiType string) map[service.Endpoint]endpoint.Data {
	glanceEndpoints := map[service.Endpoint]endpoint.Data{}
	// split
	if apiType == glancev1.APIInternal {
		glanceEndpoints["private"] = endpoint.Data{
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
