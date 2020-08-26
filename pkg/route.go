package glance

import (
	routev1 "github.com/openshift/api/route/v1"
	glancev1beta1 "github.com/openstack-k8s-operators/glance-operator/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	intstr "k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// Route func
func Route(cr *glancev1beta1.GlanceAPI, scheme *runtime.Scheme) *routev1.Route {

	labels := map[string]string{
		"app": "glance-api",
	}
	serviceRef := routev1.RouteTargetReference{
		Kind: "Service",
		Name: cr.Name,
	}
	routePort := &routev1.RoutePort{
		TargetPort: intstr.FromString("api"),
	}
	route := &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name,
			Namespace: cr.Namespace,
			Labels:    labels,
		},
		Spec: routev1.RouteSpec{
			To:   serviceRef,
			Port: routePort,
		},
	}
	controllerutil.SetControllerReference(cr, route, scheme)
	return route
}
