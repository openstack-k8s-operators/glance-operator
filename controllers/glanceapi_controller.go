/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"
	"strings"

	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/go-logr/logr"
	networkv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	cinderv1 "github.com/openstack-k8s-operators/cinder-operator/api/v1beta1"
	glancev1 "github.com/openstack-k8s-operators/glance-operator/api/v1beta1"
	"github.com/openstack-k8s-operators/glance-operator/pkg/glance"
	"github.com/openstack-k8s-operators/glance-operator/pkg/glanceapi"
	memcachedv1 "github.com/openstack-k8s-operators/infra-operator/apis/memcached/v1beta1"
	keystonev1 "github.com/openstack-k8s-operators/keystone-operator/api/v1beta1"
	"github.com/openstack-k8s-operators/lib-common/modules/common"
	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	cronjob "github.com/openstack-k8s-operators/lib-common/modules/common/cronjob"
	"github.com/openstack-k8s-operators/lib-common/modules/common/endpoint"
	"github.com/openstack-k8s-operators/lib-common/modules/common/env"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	"github.com/openstack-k8s-operators/lib-common/modules/common/labels"
	nad "github.com/openstack-k8s-operators/lib-common/modules/common/networkattachment"
	"github.com/openstack-k8s-operators/lib-common/modules/common/secret"
	oko_secret "github.com/openstack-k8s-operators/lib-common/modules/common/secret"
	"github.com/openstack-k8s-operators/lib-common/modules/common/service"
	"github.com/openstack-k8s-operators/lib-common/modules/common/statefulset"
	"github.com/openstack-k8s-operators/lib-common/modules/common/tls"
	"github.com/openstack-k8s-operators/lib-common/modules/common/util"
	mariadbv1 "github.com/openstack-k8s-operators/mariadb-operator/api/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
)

// GlanceAPIReconciler reconciles a GlanceAPI object
type GlanceAPIReconciler struct {
	client.Client
	Kclient kubernetes.Interface
	Log     logr.Logger
	Scheme  *runtime.Scheme
}

//+kubebuilder:rbac:groups=glance.openstack.org,resources=glanceapis,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=glance.openstack.org,resources=glanceapis/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=glance.openstack.org,resources=glanceapis/finalizers,verbs=update
// +kubebuilder:rbac:groups=cinder.openstack.org,resources=cinders,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete;
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=statefulsets/status,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete;
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;
// +kubebuilder:rbac:groups=keystone.openstack.org,resources=keystoneapis,verbs=get;list;watch;
// +kubebuilder:rbac:groups=keystone.openstack.org,resources=keystoneendpoints,verbs=get;list;watch;create;update;patch;delete;
// +kubebuilder:rbac:groups=k8s.cni.cncf.io,resources=network-attachment-definitions,verbs=get;list;watch
// +kubebuilder:rbac:groups=memcached.openstack.org,resources=memcacheds,verbs=get;list;watch;

// Reconcile reconcile Glance API requests
func (r *GlanceAPIReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, _err error) {
	_ = log.FromContext(ctx)

	// Fetch the GlanceAPI instance
	instance := &glancev1.GlanceAPI{}
	err := r.Client.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if k8s_errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected.
			// For additional cleanup logic use finalizers. Return and don't requeue.
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		r.Log.Error(err, fmt.Sprintf("could not fetch GlanceAPI instance %s", instance.Name))
		return ctrl.Result{}, err
	}

	helper, err := helper.NewHelper(
		instance,
		r.Client,
		r.Kclient,
		r.Scheme,
		r.Log,
	)
	if err != nil {
		r.Log.Error(err, fmt.Sprintf("could not instantiate helper for instance %s", instance.Name))
		return ctrl.Result{}, err
	}

	//
	// initialize status
	//
	// initialize status if Conditions is nil, but do not reset if it
	// already exists
	isNewInstance := instance.Status.Conditions == nil
	if isNewInstance {
		instance.Status.Conditions = condition.Conditions{}
	}

	// Save a copy of the condtions so that we can restore the LastTransitionTime
	// when a condition's state doesn't change.
	savedConditions := instance.Status.Conditions.DeepCopy()

	// Always patch the instance status when exiting this function so we can persist any changes.
	defer func() {
		condition.RestoreLastTransitionTimes(
			&instance.Status.Conditions, savedConditions)
		if instance.Status.Conditions.IsUnknown(condition.ReadyCondition) {
			instance.Status.Conditions.Set(
				instance.Status.Conditions.Mirror(condition.ReadyCondition))
		}
		err := helper.PatchInstance(ctx, instance)
		if err != nil {
			_err = err
			return
		}
	}()

	// initialize conditions used later as Status=Unknown
	cl := condition.CreateList(
		// Mark ReadyCondition as Unknown from the beginning, because the
		// Reconcile function is in progress. If this condition is not marked
		// as True and is still in the "Unknown" state, we `Mirror(` the actual
		// failure
		condition.UnknownCondition(condition.ReadyCondition, condition.InitReason, condition.ReadyInitMessage),
		condition.UnknownCondition(glancev1.CinderCondition, condition.InitReason, glancev1.CinderInitMessage),
		condition.UnknownCondition(condition.ExposeServiceReadyCondition, condition.InitReason, condition.ExposeServiceReadyInitMessage),
		condition.UnknownCondition(condition.InputReadyCondition, condition.InitReason, condition.InputReadyInitMessage),
		condition.UnknownCondition(condition.ServiceConfigReadyCondition, condition.InitReason, condition.ServiceConfigReadyInitMessage),
		condition.UnknownCondition(condition.DeploymentReadyCondition, condition.InitReason, condition.DeploymentReadyInitMessage),
		condition.UnknownCondition(condition.TLSInputReadyCondition, condition.InitReason, condition.InputReadyInitMessage),
		// right now we have no dedicated KeystoneEndpointReadyInitMessage
		condition.UnknownCondition(condition.KeystoneEndpointReadyCondition, condition.InitReason, ""),
		condition.UnknownCondition(condition.NetworkAttachmentsReadyCondition, condition.InitReason, condition.NetworkAttachmentsReadyInitMessage),
		condition.UnknownCondition(condition.CronJobReadyCondition, condition.InitReason, condition.CronJobReadyInitMessage),
	)

	instance.Status.Conditions.Init(&cl)
	instance.Status.ObservedGeneration = instance.Generation

	// If we're not deleting this and the service object doesn't have our finalizer, add it.
	if instance.DeletionTimestamp.IsZero() && controllerutil.AddFinalizer(instance, helper.GetFinalizer()) || isNewInstance {
		// Register overall status immediately to have an early feedback e.g. in the cli
		return ctrl.Result{}, nil
	}
	if instance.Status.Hash == nil {
		instance.Status.Hash = map[string]string{}
	}
	if instance.Status.APIEndpoints == nil {
		instance.Status.APIEndpoints = map[string]string{}
	}
	if instance.Status.NetworkAttachments == nil {
		instance.Status.NetworkAttachments = map[string][]string{}
	}
	// Handle service delete
	if !instance.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, instance, helper)
	}
	// Handle non-deleted clusters
	return r.reconcileNormal(ctx, instance, helper, req)
}

// SetupWithManager sets up the controller with the Manager.
func (r *GlanceAPIReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// index passwordSecretField
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &glancev1.GlanceAPI{}, passwordSecretField, func(rawObj client.Object) []string {
		// Extract the secret name from the spec, if one is provided
		cr := rawObj.(*glancev1.GlanceAPI)
		if cr.Spec.Secret == "" {
			return nil
		}
		return []string{cr.Spec.Secret}
	}); err != nil {
		return err
	}

	// index caBundleSecretNameField
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &glancev1.GlanceAPI{}, caBundleSecretNameField, func(rawObj client.Object) []string {
		// Extract the secret name from the spec, if one is provided
		cr := rawObj.(*glancev1.GlanceAPI)
		if cr.Spec.TLS.CaBundleSecretName == "" {
			return nil
		}
		return []string{cr.Spec.TLS.CaBundleSecretName}
	}); err != nil {
		return err
	}

	// index tlsAPIInternalField
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &glancev1.GlanceAPI{}, tlsAPIInternalField, func(rawObj client.Object) []string {
		// Extract the secret name from the spec, if one is provided
		cr := rawObj.(*glancev1.GlanceAPI)
		if cr.Spec.TLS.API.Internal.SecretName == nil {
			return nil
		}
		return []string{*cr.Spec.TLS.API.Internal.SecretName}
	}); err != nil {
		return err
	}

	// index tlsAPIPublicField
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &glancev1.GlanceAPI{}, tlsAPIPublicField, func(rawObj client.Object) []string {
		// Extract the secret name from the spec, if one is provided
		cr := rawObj.(*glancev1.GlanceAPI)
		if cr.Spec.TLS.API.Public.SecretName == nil {
			return nil
		}
		return []string{*cr.Spec.TLS.API.Public.SecretName}
	}); err != nil {
		return err
	}

	// Watch for changes to any CustomServiceConfigSecrets. Global secrets
	svcSecretFn := func(ctx context.Context, o client.Object) []reconcile.Request {
		var namespace string = o.GetNamespace()
		var secretName string = o.GetName()
		result := []reconcile.Request{}

		// get all API CRs
		apis := &glancev1.GlanceAPIList{}
		listOpts := []client.ListOption{
			client.InNamespace(namespace),
		}
		if err := r.Client.List(context.Background(), apis, listOpts...); err != nil {
			r.Log.Error(err, "Unable to retrieve API CRs %v")
			return nil
		}
		for _, cr := range apis.Items {
			for _, v := range cr.Spec.CustomServiceConfigSecrets {
				if v == secretName {
					name := client.ObjectKey{
						Namespace: namespace,
						Name:      cr.Name,
					}
					r.Log.Info(fmt.Sprintf("Secret %s is used by Glance CR %s", secretName, cr.Name))
					result = append(result, reconcile.Request{NamespacedName: name})
				}
			}
		}
		if len(result) > 0 {
			return result
		}
		return nil
	}

	// Watch for changes to NADs
	nadFn := func(ctx context.Context, o client.Object) []reconcile.Request {
		result := []reconcile.Request{}

		// get all GlanceAPI CRs
		glanceAPIs := &glancev1.GlanceAPIList{}
		listOpts := []client.ListOption{
			client.InNamespace(o.GetNamespace()),
		}
		if err := r.Client.List(context.Background(), glanceAPIs, listOpts...); err != nil {
			r.Log.Error(err, "Unable to retrieve GlanceAPI CRs %w")
			return nil
		}
		for _, cr := range glanceAPIs.Items {
			if util.StringInSlice(o.GetName(), cr.Spec.NetworkAttachments) {
				name := client.ObjectKey{
					Namespace: cr.GetNamespace(),
					Name:      cr.GetName(),
				}
				r.Log.Info(fmt.Sprintf("NAD %s is used by GlanceAPI CR %s", o.GetName(), cr.GetName()))
				result = append(result, reconcile.Request{NamespacedName: name})
			}
		}
		if len(result) > 0 {
			return result
		}
		return nil
	}

	memcachedFn := func(ctx context.Context, o client.Object) []reconcile.Request {
		result := []reconcile.Request{}

		// get all GlanceAPIs CRs
		glanceAPIs := &glancev1.GlanceAPIList{}
		listOpts := []client.ListOption{
			client.InNamespace(o.GetNamespace()),
		}
		if err := r.Client.List(context.Background(), glanceAPIs, listOpts...); err != nil {
			r.Log.Error(err, "Unable to retrieve GlanceAPI CRs %w")
			return nil
		}

		for _, cr := range glanceAPIs.Items {
			if o.GetName() == cr.Spec.MemcachedInstance {
				name := client.ObjectKey{
					Namespace: o.GetNamespace(),
					Name:      cr.Name,
				}
				r.Log.Info(fmt.Sprintf("Memcached %s is used by GlanceAPI CR %s", o.GetName(), cr.Name))
				result = append(result, reconcile.Request{NamespacedName: name})
			}
		}
		if len(result) > 0 {
			return result
		}
		return nil
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&glancev1.GlanceAPI{}).
		Owns(&keystonev1.KeystoneEndpoint{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.Secret{}).
		Owns(&appsv1.StatefulSet{}).
		Watches(&corev1.Secret{},
			handler.EnqueueRequestsFromMapFunc(svcSecretFn)).
		Watches(&networkv1.NetworkAttachmentDefinition{},
			handler.EnqueueRequestsFromMapFunc(nadFn)).
		Watches(
			&corev1.Secret{},
			handler.EnqueueRequestsFromMapFunc(r.findObjectsForSrc),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		).
		Watches(&memcachedv1.Memcached{},
			handler.EnqueueRequestsFromMapFunc(memcachedFn)).
		Complete(r)
}

func (r *GlanceAPIReconciler) findObjectsForSrc(ctx context.Context, src client.Object) []reconcile.Request {
	requests := []reconcile.Request{}

	l := log.FromContext(context.Background()).WithName("Controllers").WithName("GlanceAPI")

	for _, field := range glanceAPIWatchFields {
		crList := &glancev1.GlanceAPIList{}
		listOps := &client.ListOptions{
			FieldSelector: fields.OneTermEqualSelector(field, src.GetName()),
			Namespace:     src.GetNamespace(),
		}
		err := r.List(context.TODO(), crList, listOps)
		if err != nil {
			return []reconcile.Request{}
		}

		for _, item := range crList.Items {
			l.Info(fmt.Sprintf("input source %s changed, reconcile: %s - %s", src.GetName(), item.GetName(), item.GetNamespace()))

			requests = append(requests,
				reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      item.GetName(),
						Namespace: item.GetNamespace(),
					},
				},
			)
		}
	}

	return requests
}

func (r *GlanceAPIReconciler) reconcileDelete(ctx context.Context, instance *glancev1.GlanceAPI, helper *helper.Helper) (ctrl.Result, error) {
	r.Log.Info(fmt.Sprintf("Reconciling Service '%s' delete", instance.Name))
	if ctrlResult, err := r.ensureDeletedEndpoints(ctx, instance, helper); err != nil {
		return ctrlResult, err
	}

	// Endpoints are deleted so remove the finalizer.
	controllerutil.RemoveFinalizer(instance, helper.GetFinalizer())
	r.Log.Info(fmt.Sprintf("Reconciled Service '%s' delete successfully", instance.Name))

	return ctrl.Result{}, nil
}

func (r *GlanceAPIReconciler) reconcileInit(
	ctx context.Context,
	instance *glancev1.GlanceAPI,
	helper *helper.Helper,
	serviceLabels map[string]string,
) (ctrl.Result, error) {
	r.Log.Info(fmt.Sprintf("Reconciling Service '%s' init", instance.Name))

	//
	// create service/s
	//
	glanceEndpoints := glanceapi.GetGlanceEndpoints(instance.Spec.APIType)
	apiEndpoints := make(map[string]string)

	for endpointType, data := range glanceEndpoints {
		endpointTypeStr := string(endpointType)
		apiName := glance.GetGlanceAPIName(instance.Name)
		endpointName := fmt.Sprintf("%s-%s-%s", glance.ServiceName, apiName, endpointTypeStr)
		svcOverride := instance.Spec.Override.Service[endpointType]
		if svcOverride.EmbeddedLabelsAnnotations == nil {
			svcOverride.EmbeddedLabelsAnnotations = &service.EmbeddedLabelsAnnotations{}
		}

		exportLabels := util.MergeStringMaps(
			serviceLabels,
			map[string]string{
				service.AnnotationEndpointKey: endpointTypeStr,
			},
		)

		// Create the internal/externl service(s) associated to the current API
		svc, err := service.NewService(
			service.GenericService(&service.GenericServiceDetails{
				Name:      endpointName,
				Namespace: instance.Namespace,
				Labels:    exportLabels,
				Selector:  serviceLabels,
				Port: service.GenericServicePort{
					Name:     endpointName,
					Port:     data.Port,
					Protocol: corev1.ProtocolTCP,
				},
			}),
			5,
			&svcOverride.OverrideSpec,
		)
		if err != nil {
			instance.Status.Conditions.Set(condition.FalseCondition(
				condition.ExposeServiceReadyCondition,
				condition.ErrorReason,
				condition.SeverityWarning,
				condition.ExposeServiceReadyErrorMessage,
				err.Error()))

			return ctrl.Result{}, err
		}

		svc.AddAnnotation(map[string]string{
			service.AnnotationEndpointKey: endpointTypeStr,
		})

		// add Annotation to whether creating an ingress is required or not
		// A route should get created for the glance-api instance which has
		// - annotation with glance.KeystoneEndpoint -> true
		// - it is the service.EndpointPublic
		// - and the k8s service is corev1.ServiceTypeClusterIP
		if keystoneEndpoint, ok := instance.GetAnnotations()[glance.KeystoneEndpoint]; ok &&
			keystoneEndpoint == "true" && endpointType == service.EndpointPublic &&
			svc.GetServiceType() == corev1.ServiceTypeClusterIP {
			svc.AddAnnotation(map[string]string{
				service.AnnotationIngressCreateKey: "true",
			})
		} else {
			svc.AddAnnotation(map[string]string{
				service.AnnotationIngressCreateKey: "false",
			})
			if svc.GetServiceType() == corev1.ServiceTypeLoadBalancer {
				svc.AddAnnotation(map[string]string{
					service.AnnotationHostnameKey: svc.GetServiceHostname(), // add annotation to register service name in dnsmasq
				})
			}
		}

		ctrlResult, err := svc.CreateOrPatch(ctx, helper)
		if err != nil {
			instance.Status.Conditions.Set(condition.FalseCondition(
				condition.ExposeServiceReadyCondition,
				condition.ErrorReason,
				condition.SeverityWarning,
				condition.ExposeServiceReadyErrorMessage,
				err.Error()))

			return ctrlResult, err
		} else if (ctrlResult != ctrl.Result{}) {
			instance.Status.Conditions.Set(condition.FalseCondition(
				condition.ExposeServiceReadyCondition,
				condition.RequestedReason,
				condition.SeverityInfo,
				condition.ExposeServiceReadyRunningMessage))
			return ctrlResult, nil
		}

		// For each StatefulSet associated with a given glanceAPI (single, internal, external)
		// we create a headless service that allow to resolve pods by hostname (using kube-dns)
		// and it allows to enable the glance-direct import method
		ctrlResult, err = GetHeadlessService(
			ctx,
			helper,
			instance,
			serviceLabels,
		)
		if err != nil {
			// The ExposeServiceReadyCondition is already marked as False
			// within the GetHeadlessService function: we can return
			return ctrlResult, err
		}

		// create service - end

		// if TLS is enabled
		if instance.Spec.TLS.API.Enabled(endpointType) {
			// set endpoint protocol to https
			data.Protocol = ptr.To(service.ProtocolHTTPS)
		}

		apiEndpoints[string(endpointType)], err = svc.GetAPIEndpoint(
			svcOverride.EndpointURL, data.Protocol, data.Path)
		if err != nil {
			instance.Status.Conditions.MarkFalse(
				condition.ExposeServiceReadyCondition,
				condition.ErrorReason,
				condition.SeverityWarning,
				condition.ExposeServiceReadyErrorMessage,
				err.Error())
			return ctrl.Result{}, err
		}
	}
	instance.Status.Conditions.MarkTrue(condition.ExposeServiceReadyCondition, condition.ExposeServiceReadyMessage)

	//
	// Update instance status with service endpoint url from route host information
	//
	if instance.Status.APIEndpoints == nil {
		instance.Status.APIEndpoints = map[string]string{}
	}
	instance.Status.APIEndpoints = apiEndpoints

	// expose service - end

	// Create/Patch the KeystoneEndpoints
	ctrlResult, err := r.ensureKeystoneEndpoints(ctx, helper, instance, serviceLabels)
	if err != nil {
		instance.Status.Conditions.MarkFalse(
			condition.KeystoneEndpointReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			"Creating KeyStoneEndpoint CR %s",
			err.Error())
		return ctrlResult, err
	}
	r.Log.Info(fmt.Sprintf("Reconciled Service '%s' init successfully", instance.Name))
	return ctrl.Result{}, nil
}

func (r *GlanceAPIReconciler) reconcileNormal(ctx context.Context, instance *glancev1.GlanceAPI, helper *helper.Helper, req ctrl.Request) (ctrl.Result, error) {
	r.Log.Info(fmt.Sprintf("Reconciling Service '%s'", instance.Name))

	configVars := make(map[string]env.Setter)
	privileged := false
	imageConv := false

	//
	// check for required OpenStack secret holding passwords for service/admin user and add hash to the vars map
	//
	ospSecret, hash, err := oko_secret.GetSecret(ctx, helper, instance.Spec.Secret, instance.Namespace)
	if err != nil {
		if k8s_errors.IsNotFound(err) {
			instance.Status.Conditions.Set(condition.FalseCondition(
				condition.InputReadyCondition,
				condition.RequestedReason,
				condition.SeverityInfo,
				condition.InputReadyWaitingMessage))
			return glance.ResultRequeue, fmt.Errorf("OpenStack secret %s not found", instance.Spec.Secret)
		}
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.InputReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			condition.InputReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}
	configVars[ospSecret.Name] = env.SetValue(hash)
	instance.Status.Conditions.MarkTrue(condition.InputReadyCondition, condition.InputReadyMessage)
	// run check OpenStack secret - end

	//
	// Check for required memcached used for caching
	//
	memcached, err := memcachedv1.GetMemcachedByName(ctx, helper, instance.Spec.MemcachedInstance, instance.Namespace)
	if err != nil {
		if k8s_errors.IsNotFound(err) {
			instance.Status.Conditions.Set(condition.FalseCondition(
				condition.MemcachedReadyCondition,
				condition.RequestedReason,
				condition.SeverityInfo,
				condition.MemcachedReadyWaitingMessage))
			return glance.ResultRequeue, fmt.Errorf("memcached %s not found", instance.Spec.MemcachedInstance)
		}
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.MemcachedReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			condition.MemcachedReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}

	// Get Enabled backends from customServiceConfig and run pre backend conditions
	availableBackends := glancev1.GetEnabledBackends(instance.Spec.CustomServiceConfig)
	_, hashChanged, err := r.createHashOfBackendConfig(instance, availableBackends)
	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.InputReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			condition.InputReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}
	// Update the current StateFulSet (by recreating it) only when a backend is
	// added or removed from an already existing API
	if hashChanged {
		if err = r.glanceAPIRefresh(ctx, helper, instance); err != nil {
			instance.Status.Conditions.MarkFalse(
				condition.DeploymentReadyCondition,
				condition.ErrorReason,
				condition.SeverityWarning,
				condition.DeploymentReadyErrorMessage,
				err.Error(),
			)
			return ctrl.Result{}, err
		}
	}
	// iterate over availableBackends for backend specific cases
	for i := 0; i < len(availableBackends); i++ {
		backendToken := strings.SplitN(availableBackends[i], ":", 2)
		switch {
		case backendToken[1] == "cinder":
			cinder := &cinderv1.Cinder{}
			err := r.Get(ctx, types.NamespacedName{Namespace: instance.Namespace, Name: glance.CinderName}, cinder)
			if err != nil && !k8s_errors.IsNotFound(err) {
				// Request object not found, GlanceAPI can't be executed with this config
				r.Log.Info("Cinder resource not found. Waiting for it to be deployed")
				instance.Status.Conditions.Set(condition.FalseCondition(
					glancev1.CinderCondition,
					condition.RequestedReason,
					condition.SeverityInfo,
					glancev1.CinderInitMessage),
				)
				return glance.ResultRequeue, nil
			}
			// Cinder CR is found, we can unblock glance deployment because
			// it represents a valid backend.
			privileged = true
		case backendToken[1] == "rbd":
			// enable image conversion by default
			r.Log.Info("Ceph config detected: enable image conversion by default")
			imageConv = true
		}
	}
	// If we reach this point, it means that either Cinder is not a backend for Glance
	// or, in case Cinder is a backend for the current GlanceAPI, the associated resources
	// are present in the control plane
	instance.Status.Conditions.MarkTrue(glancev1.CinderCondition, glancev1.CinderReadyMessage)

	// Generate service config
	err = r.generateServiceConfig(ctx, helper, instance, &configVars, imageConv, memcached)
	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.ServiceConfigReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			condition.ServiceConfigReadyErrorMessage,
			err.Error()))
		r.Log.Info("Glance config is not Ready, requeueing...")
		return glance.ResultRequeue, nil
	}

	configVars[glance.KeystoneEndpoint] = env.SetValue(instance.ObjectMeta.Annotations[glance.KeystoneEndpoint])
	//
	// TLS input validation
	//
	// Validate the CA cert secret if provided
	if instance.Spec.TLS.CaBundleSecretName != "" {
		hash, ctrlResult, err := tls.ValidateCACertSecret(
			ctx,
			helper.GetClient(),
			types.NamespacedName{
				Name:      instance.Spec.TLS.CaBundleSecretName,
				Namespace: instance.Namespace,
			},
		)
		if err != nil {
			instance.Status.Conditions.Set(condition.FalseCondition(
				condition.TLSInputReadyCondition,
				condition.ErrorReason,
				condition.SeverityWarning,
				condition.TLSInputErrorMessage,
				err.Error()))
			return ctrlResult, err
		} else if (ctrlResult != ctrl.Result{}) {
			// Marking the condition as Unknown because we are not returining
			// an err, but comparing the ctrlResult: this represents an in
			// progress operation rather than something that failed
			instance.Status.Conditions.MarkUnknown(
				condition.TLSInputReadyCondition,
				condition.RequestedReason,
				condition.InputReadyWaitingMessage)
			return ctrlResult, nil
		}
		if hash != "" {
			configVars[tls.CABundleKey] = env.SetValue(hash)
		}
	}

	// Validate API service certs secrets
	certsHash, ctrlResult, err := instance.Spec.TLS.API.ValidateCertSecrets(ctx, helper, instance.Namespace)
	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.TLSInputReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			condition.TLSInputErrorMessage,
			err.Error()))
		return ctrlResult, err
	} else if (ctrlResult != ctrl.Result{}) {
		// Marking the condition as Unknown because we are not returining
		// an err, but comparing the ctrlResult: this represents an in
		// progress operation rather than something that failed
		instance.Status.Conditions.MarkUnknown(
			condition.TLSInputReadyCondition,
			condition.RequestedReason,
			condition.InputReadyWaitingMessage)
		return ctrlResult, nil
	}
	configVars[tls.TLSHashName] = env.SetValue(certsHash)
	// all cert input checks out so report InputReady
	instance.Status.Conditions.MarkTrue(condition.TLSInputReadyCondition, condition.InputReadyMessage)

	//
	// create hash over all the different input resources to identify if any those changed
	// and a restart/recreate is required.
	//
	inputHash, hashChanged, err := r.createHashOfInputHashes(ctx, instance, configVars)
	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.ServiceConfigReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			condition.ServiceConfigReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}
	instance.Status.Conditions.MarkTrue(condition.ServiceConfigReadyCondition, condition.ServiceConfigReadyMessage)
	// Create Secrets - end

	//
	// TODO check when/if Init, Update, or Upgrade should/could be skipped
	//
	var serviceAnnotations map[string]string
	// networks to attach to
	serviceAnnotations, ctrlResult, err = ensureNAD(ctx, &instance.Status.Conditions, instance.Spec.NetworkAttachments, helper)
	if err != nil {
		instance.Status.Conditions.MarkFalse(
			condition.NetworkAttachmentsReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			condition.NetworkAttachmentsReadyErrorMessage,
			err,
		)
		return ctrlResult, err
	}

	// Handle service init
	ctrlResult, err = r.reconcileInit(ctx, instance, helper, GetServiceLabels(instance))
	if err != nil {
		return ctrlResult, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	//
	// normal reconcile tasks
	//

	// Define a new StatefuleSet object
	deplDef, err := glanceapi.StatefulSet(instance,
		inputHash,
		GetServiceLabels(instance),
		serviceAnnotations,
		privileged,
		imageConv,
	)
	if err != nil {
		return ctrlResult, err
	}
	depl := statefulset.NewStatefulSet(
		deplDef,
		glance.ShortDuration,
	)

	ctrlResult, err = depl.CreateOrPatch(ctx, helper)
	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.DeploymentReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			condition.DeploymentReadyErrorMessage,
			err.Error()))
		return ctrlResult, err
	} else if (ctrlResult != ctrl.Result{}) {
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.DeploymentReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			condition.DeploymentReadyRunningMessage))
		return ctrlResult, nil
	}

	if depl.GetStatefulSet().Generation <= depl.GetStatefulSet().Status.ObservedGeneration {
		instance.Status.ReadyCount = depl.GetStatefulSet().Status.ReadyReplicas
		// verify if network attachment matches expectations
		networkReady := false
		networkAttachmentStatus := map[string][]string{}
		if *instance.Spec.Replicas > 0 {
			networkReady, networkAttachmentStatus, err = nad.VerifyNetworkStatusFromAnnotation(
				ctx,
				helper,
				instance.Spec.NetworkAttachments,
				GetServiceLabels(instance),
				instance.Status.ReadyCount,
			)
			if err != nil {
				err = fmt.Errorf("verifying API NetworkAttachments (%s) %w", instance.Spec.NetworkAttachments, err)
				instance.Status.Conditions.MarkFalse(
					condition.NetworkAttachmentsReadyCondition,
					condition.ErrorReason,
					condition.SeverityWarning,
					condition.NetworkAttachmentsReadyErrorMessage,
					err)
				return ctrl.Result{}, err
			}
		} else {
			networkReady = true
		}
		instance.Status.NetworkAttachments = networkAttachmentStatus
		if networkReady {
			instance.Status.Conditions.MarkTrue(condition.NetworkAttachmentsReadyCondition, condition.NetworkAttachmentsReadyMessage)
		} else {
			err := fmt.Errorf("not all pods have interfaces with ips as configured in NetworkAttachments: %s", instance.Spec.NetworkAttachments)
			instance.Status.Conditions.Set(condition.FalseCondition(
				condition.NetworkAttachmentsReadyCondition,
				condition.ErrorReason,
				condition.SeverityWarning,
				condition.NetworkAttachmentsReadyErrorMessage,
				err.Error()))
			return ctrl.Result{}, err
		}
		// Mark the Deployment as Ready only if the number of Replicas is equals
		// to the Deployed instances (ReadyCount), but mark it as True is Replicas
		// is zero. In addition, make sure the controller sees the last Generation
		// by comparing it with the ObservedGeneration set in the StateFulSet.
		if instance.Status.ReadyCount == *instance.Spec.Replicas {
			instance.Status.Conditions.MarkTrue(
				condition.DeploymentReadyCondition,
				condition.DeploymentReadyMessage,
			)
		} else {
			instance.Status.Conditions.Set(condition.FalseCondition(
				condition.DeploymentReadyCondition,
				condition.RequestedReason,
				condition.SeverityInfo,
				condition.DeploymentReadyRunningMessage))
		}
	}
	// create StatefulSet - end

	// create ImageCache cronJobs

	if len(instance.Spec.ImageCache.Size) > 0 {
		// If image-cache has been enabled, create two additional cronJobs:
		// - CacheCleanerJob: clean stalled images or in an invalid state
		// - CachePrunerJob: clean the image-cache folder to stay under ImageCacheSize
		//   limit
		for _, item := range []glance.CronJobType{glance.CacheCleaner, glance.CachePruner} {
			ctrlResult, err = r.ensureImageCacheJob(
				ctx,
				helper,
				instance,
				GetServiceLabels(instance),
				serviceAnnotations,
				item,
			)
			if err != nil {
				instance.Status.Conditions.Set(condition.FalseCondition(
					condition.CronJobReadyCondition,
					condition.ErrorReason,
					condition.SeverityWarning,
					condition.CronJobReadyErrorMessage,
					err.Error()))
				return ctrlResult, err
			}
		}
		// Cleanup any existing (but not used) ImageCache cronJob
		if ctrlResult, err := r.deleteImageCacheJob(
			ctx,
			helper,
			instance,
			GetServiceLabels(instance),
		); err != nil {
			return ctrlResult, err
		}
	}

	// If we reach this point, we can mark the CronJobReadyCondition as True
	instance.Status.Conditions.MarkTrue(
		condition.CronJobReadyCondition,
		condition.CronJobReadyMessage,
	)
	// create ImageCache cronJobs - end

	// We reached the end of the Reconcile, update the Ready condition based on
	// the sub conditions
	if instance.Status.Conditions.AllSubConditionIsTrue() {
		instance.Status.Conditions.MarkTrue(
			condition.ReadyCondition, condition.ReadyMessage)
	}
	r.Log.Info(fmt.Sprintf("Reconciled Service '%s' successfully", instance.Name))
	return ctrl.Result{}, nil
}

// generateServiceConfig - create create secrets which hold scripts and service configuration
func (r *GlanceAPIReconciler) generateServiceConfig(
	ctx context.Context,
	h *helper.Helper,
	instance *glancev1.GlanceAPI,
	envVars *map[string]env.Setter,
	imageConv bool,
	memcached *memcachedv1.Memcached,
) error {
	labels := labels.GetLabels(instance, labels.GetGroupLabel(glance.ServiceName), GetServiceLabels(instance))

	db, err := mariadbv1.GetDatabaseByNameAndAccount(ctx, h, glance.DatabaseName, instance.Spec.DatabaseAccount, instance.Namespace)
	if err != nil {
		return err
	}

	var tlsCfg *tls.Service
	if instance.Spec.TLS.Ca.CaBundleSecretName != "" {
		tlsCfg = &tls.Service{}
	}
	// 02-config.conf
	customData := map[string]string{
		glance.CustomServiceConfigFileName: instance.Spec.CustomServiceConfig,
		"my.cnf":                           db.GetDatabaseClientConfig(tlsCfg), //(mschuppert) for now just get the default my.cnf
	}

	// 03-config.conf
	customSecrets := ""
	for _, secretName := range instance.Spec.CustomServiceConfigSecrets {
		secret, _, err := secret.GetSecret(ctx, h, secretName, instance.Namespace)
		if err != nil {
			return err
		}
		for _, data := range secret.Data {
			customSecrets += string(data) + "\n"
		}
	}
	customData[glance.CustomServiceConfigSecretsFileName] = customSecrets

	keystoneAPI, err := keystonev1.GetKeystoneAPI(ctx, h, instance.Namespace, map[string]string{})
	// KeystoneAPI not available we should not aggregate the error and continue
	if err != nil {
		return err
	}
	keystoneInternalURL, err := keystoneAPI.GetEndpoint(endpoint.EndpointInternal)
	if err != nil {
		return err
	}
	keystonePublicURL, err := keystoneAPI.GetEndpoint(endpoint.EndpointPublic)
	if err != nil {
		return err
	}

	ospSecret, _, err := secret.GetSecret(ctx, h, instance.Spec.Secret, instance.Namespace)
	if err != nil {
		return err
	}

	databaseAccount := db.GetAccount()
	dbSecret := db.GetSecret()

	glanceEndpoints := glanceapi.GetGlanceEndpoints(instance.Spec.APIType)
	httpdVhostConfig := map[string]interface{}{}
	for endpt := range glanceEndpoints {
		endptConfig := map[string]interface{}{}
		endptConfig["ServerName"] = fmt.Sprintf("glance-%s.%s.svc", endpt.String(), instance.Namespace)
		endptConfig["TLS"] = false // default TLS to false, and set it bellow to true if enabled
		if instance.Spec.TLS.API.Enabled(endpt) {
			endptConfig["TLS"] = true
			endptConfig["SSLCertificateFile"] = fmt.Sprintf("/etc/pki/tls/certs/%s.crt", endpt.String())
			endptConfig["SSLCertificateKeyFile"] = fmt.Sprintf("/etc/pki/tls/private/%s.key", endpt.String())
		}
		httpdVhostConfig[endpt.String()] = endptConfig
	}

	templateParameters := map[string]interface{}{
		"ServiceUser":         instance.Spec.ServiceUser,
		"ServicePassword":     string(ospSecret.Data[instance.Spec.PasswordSelectors.Service]),
		"KeystoneInternalURL": keystoneInternalURL,
		"KeystonePublicURL":   keystonePublicURL,
		"DatabaseConnection": fmt.Sprintf("mysql+pymysql://%s:%s@%s/%s?read_default_file=/etc/my.cnf",
			databaseAccount.Spec.UserName,
			string(dbSecret.Data[mariadbv1.DatabasePasswordSelector]),
			instance.Spec.DatabaseHostname,
			glance.DatabaseName,
		),
		// If Quota values are defined in the top level spec (they are global values),
		// each GlanceAPI instance should build the config file according to
		// https://docs.openstack.org/glance/latest/admin/quotas.html
		"QuotaEnabled": instance.Spec.Quota,
		"LogFile":      fmt.Sprintf("%s%s.log", glance.GlanceLogPath, instance.Name),
		"VHosts":       httpdVhostConfig,
	}

	// Configure the internal GlanceAPI to provide image location data, and the
	// external version to *not* provide it; if we don't split, the resulting
	// GlanceAPI instance will provide it.
	if instance.Spec.APIType == glancev1.APIInternal ||
		instance.Spec.APIType == glancev1.APISingle ||
		instance.Spec.APIType == glancev1.APIEdge {
		templateParameters["ShowImageDirectUrl"] = true
		templateParameters["ShowMultipleLocations"] = true
	} else {
		templateParameters["ShowImageDirectUrl"] = false
		templateParameters["ShowMultipleLocations"] = false
		templateParameters["ImageConversion"] = imageConv
	}

	// Configure the cache bits accordingly as global options (00-config.conf)
	if len(instance.Spec.ImageCache.Size) > 0 {
		// if ImageCacheSize is not a valid k8s Quantity, return an error
		cacheSize, err := resource.ParseQuantity(instance.Spec.ImageCache.Size)
		if err != nil {
			return err
		}
		templateParameters["CacheEnabled"] = true
		templateParameters["CacheMaxSize"] = cacheSize.Value()
		templateParameters["ImageCacheDir"] = glance.ImageCacheDir
	}
	templateParameters["MemcachedServersWithInet"] = memcached.GetMemcachedServerListWithInetString()

	// 00-default.conf will be regenerated as we have a ln -s of the
	// templates/glance/config directory
	// Do not generate -scripts as they are inherited from the top-level CR
	return GenerateConfigsGeneric(ctx, h, instance, envVars, templateParameters, customData, labels, false)
}

// createHashOfInputHashes - creates a hash of hashes which gets added to the resources which requires a restart
// if any of the input resources change, like configs, passwords, ...
//
// returns the hash, whether the hash changed (as a bool) and any error
func (r *GlanceAPIReconciler) createHashOfInputHashes(
	ctx context.Context,
	instance *glancev1.GlanceAPI,
	envVars map[string]env.Setter,
) (string, bool, error) {
	var hashMap map[string]string
	changed := false
	mergedMapVars := env.MergeEnvs([]corev1.EnvVar{}, envVars)
	hash, err := util.ObjectHash(mergedMapVars)
	if err != nil {
		return hash, changed, err
	}
	if hashMap, changed = util.SetHash(instance.Status.Hash, common.InputHashName, hash); changed {
		instance.Status.Hash = hashMap
		r.Log.Info(fmt.Sprintf("Input maps hash %s - %s", common.InputHashName, hash))
	}
	return hash, changed, nil
}

// createHashOfBackendConfig - It creates an Hash of the current "enabledBackend"
// string, and it's set in the .Status.Hash of the current GlanceAPI.
// If a backend is added or removed, we're able to attach a new PVC for an existing
// API by recreating the StateFulSet through the glanceAPIRefresh function. This
// function helps to figure out if the glanceAPIRefresh should be triggered or not
func (r *GlanceAPIReconciler) createHashOfBackendConfig(
	instance *glancev1.GlanceAPI,
	backends []string,
) (string, bool, error) {
	var hashMap map[string]string
	changed := false
	hash, err := util.ObjectHash(backends)
	if err != nil {
		return hash, changed, err
	}
	if hashMap, changed = util.SetHash(instance.Status.Hash, "backendHash", hash); changed {
		instance.Status.Hash = hashMap
		r.Log.Info(fmt.Sprintf("Backend hash %s - %s", "backendHash", hash))
	}
	return hash, changed, nil
}

// ensureKeystoneEndpoints -  create or update keystone endpoints
func (r *GlanceAPIReconciler) ensureKeystoneEndpoints(
	ctx context.Context,
	helper *helper.Helper,
	instance *glancev1.GlanceAPI,
	serviceLabels map[string]string,
) (ctrl.Result, error) {
	var ctrlResult ctrl.Result
	var err error

	// If the parent controller didn't set the annotation, the current glanceAPIs
	// shouldn't register the endpoints in keystone
	if len(instance.ObjectMeta.Annotations) == 0 ||
		instance.ObjectMeta.Annotations[glance.KeystoneEndpoint] != "true" {
		// Mark the KeystoneEndpointReadyCondition as True because there's nothing
		// to do here
		instance.Status.Conditions.MarkTrue(
			condition.KeystoneEndpointReadyCondition, condition.ReadyMessage)
		// If the current glanceAPI was the main one and the annotation has been removed, there is
		// an associated keystone endpoint that should be removed to keep the 1:1 relation between
		// image service - keystone Endpoint. For this reason here we try to delete the existing
		// KeystoneEndpoints associated to the current glanceAPI, so that the new API can update
		// the registered endpoints with the new URL.
		err = keystonev1.DeleteKeystoneEndpointWithName(ctx, helper, instance.Name, instance.Namespace)
		if err != nil {
			return glance.ResultRequeue, fmt.Errorf("Endpoint %s not found", instance.Name)
		}
		return ctrlResult, nil
	}
	// Build the keystoneEndpoints Spec
	ksEndpointSpec := keystonev1.KeystoneEndpointSpec{
		ServiceName: glance.ServiceName,
		Endpoints:   instance.Status.APIEndpoints,
	}
	ksSvc := keystonev1.NewKeystoneEndpoint(
		instance.Name,
		instance.Namespace,
		ksEndpointSpec,
		serviceLabels,
		glance.NormalDuration,
	)
	ctrlResult, err = ksSvc.CreateOrPatch(ctx, helper)
	if err != nil {
		return ctrlResult, err
	}
	// mirror the Status, Reason, Severity and Message of the latest keystoneendpoint condition
	// into a local condition with the type condition.KeystoneEndpointReadyCondition
	c := ksSvc.GetConditions().Mirror(condition.KeystoneEndpointReadyCondition)
	if c != nil {
		instance.Status.Conditions.Set(c)
	}
	return ctrlResult, nil
}

// ensureDeletedEndpoints -
func (r *GlanceAPIReconciler) ensureDeletedEndpoints(
	ctx context.Context,
	instance *glancev1.GlanceAPI,
	h *helper.Helper,
) (ctrl.Result, error) {
	// Remove the finalizer from our KeystoneEndpoint CR
	keystoneEndpoint, err := keystonev1.GetKeystoneEndpointWithName(ctx, h, instance.Name, instance.Namespace)

	// It might happen that the resource is not found because it does not match
	// with the one exposing the keystone endpoints. If the keystoneendpoints
	// CR does not exist it doesn't mean there's an issue, hence we don't have
	// to do nothing, just return without an error
	if k8s_errors.IsNotFound(err) {
		return ctrl.Result{}, nil
	}
	if err != nil && !k8s_errors.IsNotFound(err) {
		return ctrl.Result{}, err
	}
	if err == nil {
		if controllerutil.RemoveFinalizer(keystoneEndpoint, h.GetFinalizer()) {
			err = r.Update(ctx, keystoneEndpoint)
			if err != nil && !k8s_errors.IsNotFound(err) {
				return ctrl.Result{}, err
			}
			util.LogForObject(h, "Removed finalizer from our KeystoneEndpoint", instance)
		}
	}
	return ctrl.Result{}, err
}

func (r *GlanceAPIReconciler) ensureImageCacheJob(
	ctx context.Context,
	h *helper.Helper,
	instance *glancev1.GlanceAPI,
	serviceLabels map[string]string,
	serviceAnnotations map[string]string,
	cjType glance.CronJobType,
) (ctrl.Result, error) {

	var err error
	var ctrlResult ctrl.Result

	command := glance.GlanceCacheCleaner
	schedule := instance.Spec.ImageCache.CleanerScheduler

	if cjType == glance.CachePruner {
		command = glance.GlanceCachePruner
		schedule = instance.Spec.ImageCache.PrunerScheduler
	}
	cachePVCs, _ := GetPvcListWithLabel(ctx, h, instance.Namespace, serviceLabels)
	for _, vc := range cachePVCs.Items {
		var pvcName string = vc.GetName()
		cacheAnnotations := vc.GetAnnotations()
		if _, ok := cacheAnnotations["image-cache"]; ok {
			cronSpec := glance.CronJobSpec{
				Name:        fmt.Sprintf("%s-%s", pvcName, cjType),
				PvcClaim:    &pvcName,
				Command:     command,
				CjType:      cjType,
				Schedule:    schedule,
				Labels:      serviceLabels,
				Annotations: serviceAnnotations,
			}
			cronjobDef := glanceapi.ImageCacheJob(
				instance,
				cronSpec,
			)
			imageCacheCronJob := cronjob.NewCronJob(
				cronjobDef,
				glance.ShortDuration,
			)
			ctrlResult, err := imageCacheCronJob.CreateOrPatch(ctx, h)
			if err != nil {
				return ctrlResult, err
			}
		}
	}
	return ctrlResult, err
}

// deleteImageCacheJob - delete the ImageCache cronJobs associated to a given
// GlanceAPI
func (r *GlanceAPIReconciler) deleteImageCacheJob(
	ctx context.Context,
	h *helper.Helper,
	instance *glancev1.GlanceAPI,
	serviceLabels map[string]string,
) (ctrl.Result, error) {
	var err error
	var ctrlResult ctrl.Result
	// Get the PVCs using labelSelector (only the PVCs associated to the current
	// GlanceAPI are retrieved)
	cachePVCs, _ := GetPvcListWithLabel(ctx, h, instance.Namespace, serviceLabels)

	// For each PVC that present the image-cache annotation, check if there's an
	// associated POD (pvc and pod shares the same StatefulSet templating mechanism)
	for _, vc := range cachePVCs.Items {
		cacheAnnotations := vc.GetAnnotations()
		if _, ok := cacheAnnotations["image-cache"]; ok {
			var pvcName string = vc.GetName()
			// Get the pod (by name) associated to the current pvc
			var pod corev1.Pod
			if err := r.Client.Get(ctx, types.NamespacedName{
				Name:      strings.TrimPrefix(pvcName, "glance-cache-"),
				Namespace: instance.Namespace,
			}, &pod); err != nil && k8s_errors.IsNotFound(err) {
				// if we have no pod Running with the associated cache pvc,
				// we can delete the imageCache cronJob if still exists
				if ctrlResult, err = r.deleteJob(ctx, instance, pvcName); err != nil {
					return ctrl.Result{}, nil
				}
			}
		}
	}
	return ctrlResult, err
}

// deleteJob - delete an imageCache cronJob no longer used
func (r *GlanceAPIReconciler) deleteJob(
	ctx context.Context,
	instance *glancev1.GlanceAPI,
	pvcName string,
) (ctrl.Result, error) {
	var err error
	var ctrlResult ctrl.Result
	var cronJob batchv1.CronJob
	// For each imageCache we have both cleaner and pruner cronJobs to check and
	// cleanup if the conditions are met
	for _, cj := range []glance.CronJobType{glance.CachePruner, glance.CacheCleaner} {
		if err = r.Client.Get(
			ctx,
			types.NamespacedName{
				Name:      fmt.Sprintf("%s-%s", pvcName, cj),
				Namespace: instance.Namespace},
			&cronJob,
		); err != nil {
			// It is possible that the pvc still exists, but the GlanceAPI has no
			// associated replicas anymore: in this case there's no cronJob and
			// we should move to the next item: we don't have to raise any exception
			continue
		}
		// A cronJob is found and the delete is called
		if err = r.Delete(ctx, &cronJob, client.PropagationPolicy(metav1.DeletePropagationBackground)); err != nil {
			return ctrlResult, err
		}
	}
	return ctrlResult, err
}

// glanceAPIRefresh - delete a StateFulSet when a configuration for a Forbidden
// parameter happens: it might be required if we add / remove a backend (including
// ceph) where imageConversion is enabled and a dedicated PVC is created using
// statefulsets volume templates
func (r *GlanceAPIReconciler) glanceAPIRefresh(
	ctx context.Context,
	h *helper.Helper,
	instance *glancev1.GlanceAPI,
) error {
	sts, err := statefulset.GetStatefulSetWithName(ctx, h, fmt.Sprintf("%s-api", instance.Name), instance.Namespace)
	if err != nil {
		if k8s_errors.IsNotFound(err) {
			// Request object not found
			r.Log.Info(fmt.Sprintf("GlanceAPI %s-api: Statefulset not found.", instance.Name))
			return nil
		}
	}
	err = r.Client.Delete(ctx, sts)
	if err != nil && !k8s_errors.IsNotFound(err) {
		err = fmt.Errorf("Error deleting %s: %w", instance.Name, err)
		return err
	}
	return nil
}
