package glanceapi

import (
	glancev1 "github.com/openstack-k8s-operators/glance-operator/api/v1beta1"

	glance "github.com/openstack-k8s-operators/glance-operator/internal/glance"
	"github.com/openstack-k8s-operators/lib-common/modules/common/endpoint"
	"github.com/openstack-k8s-operators/lib-common/modules/common/service"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// ColocateWithPod - Returns a corev1.Affinity that pins a pod to the same
// node as the named StatefulSet pod. Required for sharing RWO volumes.
func ColocateWithPod(podName string) *corev1.Affinity {
	return &corev1.Affinity{
		PodAffinity: &corev1.PodAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
				{
					LabelSelector: &metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{
							{
								Key:      "statefulset.kubernetes.io/pod-name",
								Operator: metav1.LabelSelectorOpIn,
								Values:   []string{podName},
							},
						},
					},
					TopologyKey: corev1.LabelHostname,
				},
			},
		},
	}
}
