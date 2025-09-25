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
	"strconv"

	"github.com/gophercloud/gophercloud"
	"github.com/openstack-k8s-operators/lib-common/modules/common/tls"
	rbacv1 "k8s.io/api/rbac/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/go-logr/logr"
	glancev1 "github.com/openstack-k8s-operators/glance-operator/api/v1beta1"
	"github.com/openstack-k8s-operators/glance-operator/pkg/glance"
	memcachedv1 "github.com/openstack-k8s-operators/infra-operator/apis/memcached/v1beta1"
	rabbitmqv1 "github.com/openstack-k8s-operators/infra-operator/apis/rabbitmq/v1beta1"
	keystonev1 "github.com/openstack-k8s-operators/keystone-operator/api/v1beta1"
	"github.com/openstack-k8s-operators/lib-common/modules/common"
	"github.com/openstack-k8s-operators/lib-common/modules/common/annotations"
	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	cronjob "github.com/openstack-k8s-operators/lib-common/modules/common/cronjob"
	"github.com/openstack-k8s-operators/lib-common/modules/common/endpoint"
	"github.com/openstack-k8s-operators/lib-common/modules/common/env"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	"github.com/openstack-k8s-operators/lib-common/modules/common/job"
	"github.com/openstack-k8s-operators/lib-common/modules/common/labels"
	common_rbac "github.com/openstack-k8s-operators/lib-common/modules/common/rbac"
	"github.com/openstack-k8s-operators/lib-common/modules/common/util"
	"github.com/openstack-k8s-operators/lib-common/modules/openstack"
	mariadbv1 "github.com/openstack-k8s-operators/mariadb-operator/api/v1beta1"
	"k8s.io/apimachinery/pkg/api/resource"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GlanceReconciler reconciles a Glance object
type GlanceReconciler struct {
	client.Client
	Kclient kubernetes.Interface
	Scheme  *runtime.Scheme
}

// GetLogger returns a logger object with a prefix of "controller.name" and additional controller context fields
func (r *GlanceReconciler) GetLogger(ctx context.Context) logr.Logger {
	return log.FromContext(ctx).WithName("Controllers").WithName("Glance")
}

//+kubebuilder:rbac:groups=glance.openstack.org,resources=glances,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=glance.openstack.org,resources=glances/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=glance.openstack.org,resources=glances/finalizers,verbs=update;patch
// +kubebuilder:rbac:groups=glance.openstack.org,resources=glanceapis,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=glance.openstack.org,resources=glanceapis/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=glance.openstack.org,resources=glanceapis/finalizers,verbs=update;patch
// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;create;update;delete;watch;patch
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete;
// +kubebuilder:rbac:groups=mariadb.openstack.org,resources=mariadbdatabases,verbs=get;list;watch;create;update;patch;delete;
// +kubebuilder:rbac:groups=mariadb.openstack.org,resources=mariadbaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=mariadb.openstack.org,resources=mariadbaccounts/finalizers,verbs=update;patch
// +kubebuilder:rbac:groups=memcached.openstack.org,resources=memcacheds,verbs=get;list;watch;
// +kubebuilder:rbac:groups=keystone.openstack.org,resources=keystoneapis,verbs=get;list;watch;
// +kubebuilder:rbac:groups=keystone.openstack.org,resources=keystoneservices,verbs=get;list;watch;create;update;patch;delete;
// +kubebuilder:rbac:groups=k8s.cni.cncf.io,resources=network-attachment-definitions,verbs=get;list;watch
// service account, role, rolebinding
// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=roles,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=rolebindings,verbs=get;list;watch;create;update;patch
// glance service account permissions that are needed to grant permission to the above
// +kubebuilder:rbac:groups="security.openshift.io",resourceNames=anyuid;privileged,resources=securitycontextconstraints,verbs=use
// +kubebuilder:rbac:groups="",resources=pods,verbs=create;delete;get;list;patch;update;watch
// +kubebuilder:rbac:groups=batch,resources=cronjobs,verbs=get;list;watch;create;update;patch;delete;
// +kubebuilder:rbac:groups=rabbitmq.openstack.org,resources=transporturls,verbs=get;list;watch;create;update;patch;delete

// Reconcile reconcile Glance requests
func (r *GlanceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, _err error) {
	Log := r.GetLogger(ctx)

	// Fetch the Glance instance
	instance := &glancev1.Glance{}
	err := r.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if k8s_errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected.
			// For additional cleanup logic use finalizers. Return and don't requeue.
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		Log.Error(err, fmt.Sprintf("could not fetch Glance instance %s", instance.Name))
		return ctrl.Result{}, err
	}

	helper, err := helper.NewHelper(
		instance,
		r.Client,
		r.Kclient,
		r.Scheme,
		Log,
	)
	if err != nil {
		Log.Error(err, fmt.Sprintf("could not instantiate helper for instance %s", instance.Name))
		return ctrl.Result{}, err
	}

	// initialize status if Conditions is nil, but do not reset if it already
	// exists
	isNewInstance := instance.Status.Conditions == nil
	if isNewInstance {
		instance.Status.Conditions = condition.Conditions{}
	}

	// Save a copy of the condtions so that we can restore the LastTransitionTime
	// when a condition's state doesn't change.
	savedConditions := instance.Status.Conditions.DeepCopy()

	// Always patch the instance status when exiting this function so we can
	// persist any changes.
	defer func() {
		// Don't update the status, if Reconciler Panics
		if rc := recover(); rc != nil {
			Log.Info(fmt.Sprintf("Panic during reconcile %v\n", rc))
			panic(rc)
		}
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

	// initialize conditions used later as Status=Unknown, except the ReadyCondition
	// that should be False when we start
	cl := condition.CreateList(
		// Mark ReadyCondition as Unknown from the beginning, because the
		// Reconcile function is in progress. If this condition is not marked
		// as True and is still in the "Unknown" state, we `Mirror(` the actual
		// failure
		condition.UnknownCondition(condition.ReadyCondition, condition.InitReason, condition.ReadyInitMessage),
		condition.UnknownCondition(condition.DBReadyCondition, condition.InitReason, condition.DBReadyInitMessage),
		condition.UnknownCondition(condition.DBSyncReadyCondition, condition.InitReason, condition.DBSyncReadyInitMessage),
		condition.UnknownCondition(condition.MemcachedReadyCondition, condition.InitReason, condition.MemcachedReadyInitMessage),
		condition.UnknownCondition(condition.InputReadyCondition, condition.InitReason, condition.InputReadyInitMessage),
		condition.UnknownCondition(condition.ServiceConfigReadyCondition, condition.InitReason, condition.ServiceConfigReadyInitMessage),
		condition.UnknownCondition(glancev1.GlanceAPIReadyCondition, condition.InitReason, glancev1.GlanceAPIReadyInitMessage),
		condition.UnknownCondition(condition.KeystoneServiceReadyCondition, condition.InitReason, ""),
		// service account, role, rolebinding conditions
		condition.UnknownCondition(condition.ServiceAccountReadyCondition, condition.InitReason, condition.ServiceAccountReadyInitMessage),
		condition.UnknownCondition(condition.RoleReadyCondition, condition.InitReason, condition.RoleReadyInitMessage),
		condition.UnknownCondition(condition.RoleBindingReadyCondition, condition.InitReason, condition.RoleBindingReadyInitMessage),
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
	if instance.Status.GlanceAPIReadyCounts == nil {
		instance.Status.GlanceAPIReadyCounts = map[string]int32{}
	}

	if instance.Spec.NotificationBusInstance != nil {
		c := condition.UnknownCondition(
			condition.NotificationBusInstanceReadyCondition,
			condition.InitReason,
			condition.NotificationBusInstanceReadyInitMessage)
		cl.Set(c)
	}

	// Handle service delete
	if !instance.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, instance, helper)
	}

	// Handle non-deleted clusters
	return r.reconcileNormal(ctx, instance, helper)
}

// SetupWithManager sets up the controller with the Manager.
func (r *GlanceReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	Log := r.GetLogger(ctx)
	// index passwordSecretField
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &glancev1.Glance{}, passwordSecretField, func(rawObj client.Object) []string {
		// Extract the secret name from the spec, if one is provided
		cr := rawObj.(*glancev1.Glance)
		if cr.Spec.Secret == "" {
			return nil
		}
		return []string{cr.Spec.Secret}
	}); err != nil {
		return err
	}

	memcachedFn := func(_ context.Context, o client.Object) []reconcile.Request {
		result := []reconcile.Request{}

		// get all Glance CRs
		glances := &glancev1.GlanceList{}
		listOpts := []client.ListOption{
			client.InNamespace(o.GetNamespace()),
		}
		if err := r.List(context.Background(), glances, listOpts...); err != nil {
			Log.Error(err, "Unable to retrieve Glance CRs %w")
			return nil
		}

		for _, cr := range glances.Items {
			if o.GetName() == cr.Spec.MemcachedInstance {
				name := client.ObjectKey{
					Namespace: o.GetNamespace(),
					Name:      cr.Name,
				}
				Log.Info(fmt.Sprintf("Memcached %s is used by Glance CR %s", o.GetName(), cr.Name))
				result = append(result, reconcile.Request{NamespacedName: name})
			}
		}
		if len(result) > 0 {
			return result
		}
		return nil
	}

	// transportURLSecretFn - Watch for changes made to the secret associated with the RabbitMQ
	// TransportURL created and used by Glance CRs.
	transportURLSecretFn := func(_ context.Context, o client.Object) []reconcile.Request {
		result := []reconcile.Request{}
		// get all Manila CRs
		glances := &glancev1.GlanceList{}
		listOpts := []client.ListOption{
			client.InNamespace(o.GetNamespace()),
		}
		if err := r.List(context.Background(), glances, listOpts...); err != nil {
			Log.Error(err, "Unable to retrieve Glance CRs %v")
			return nil
		}
		for _, ownerRef := range o.GetOwnerReferences() {
			if ownerRef.Kind == "TransportURL" {
				for _, cr := range glances.Items {
					if ownerRef.Name == fmt.Sprintf("%s-glance-transport", cr.Name) {
						// return namespace and Name of CR
						name := client.ObjectKey{
							Namespace: o.GetNamespace(),
							Name:      cr.Name,
						}
						Log.Info(fmt.Sprintf("TransportURL Secret %s belongs to TransportURL belonging to Glance CR %s", o.GetName(), cr.Name))
						result = append(result, reconcile.Request{NamespacedName: name})
					}
				}
			}
		}
		if len(result) > 0 {
			return result
		}
		return nil
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&glancev1.Glance{}).
		Owns(&glancev1.GlanceAPI{}).
		Owns(&mariadbv1.MariaDBDatabase{}).
		Owns(&mariadbv1.MariaDBAccount{}).
		Owns(&keystonev1.KeystoneService{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Owns(&batchv1.Job{}).
		Owns(&batchv1.CronJob{}).
		Owns(&corev1.Secret{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&rbacv1.Role{}).
		Owns(&rbacv1.RoleBinding{}).
		Owns(&rabbitmqv1.TransportURL{}).
		// Watch for TransportURL Secrets which belong to any TransportURLs created by Glance CRs
		Watches(&corev1.Secret{},
			handler.EnqueueRequestsFromMapFunc(transportURLSecretFn)).
		Watches(
			&corev1.Secret{},
			handler.EnqueueRequestsFromMapFunc(r.findObjectsForSrc),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		).
		Watches(&memcachedv1.Memcached{},
			handler.EnqueueRequestsFromMapFunc(memcachedFn)).
		Complete(r)
}

func (r *GlanceReconciler) findObjectsForSrc(ctx context.Context, src client.Object) []reconcile.Request {
	requests := []reconcile.Request{}

	Log := r.GetLogger(ctx)

	for _, field := range glanceWatchFields {
		crList := &glancev1.GlanceList{}
		listOps := &client.ListOptions{
			FieldSelector: fields.OneTermEqualSelector(field, src.GetName()),
			Namespace:     src.GetNamespace(),
		}
		err := r.List(ctx, crList, listOps)
		if err != nil {
			Log.Error(err, fmt.Sprintf("listing %s for field: %s - %s", crList.GroupVersionKind().Kind, field, src.GetNamespace()))
			return requests
		}

		for _, item := range crList.Items {
			Log.Info(fmt.Sprintf("input source %s changed, reconcile: %s - %s", src.GetName(), item.GetName(), item.GetNamespace()))

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

func (r *GlanceReconciler) reconcileDelete(ctx context.Context, instance *glancev1.Glance, helper *helper.Helper) (ctrl.Result, error) {
	Log := r.GetLogger(ctx)
	Log.Info(fmt.Sprintf("Reconciling Service '%s' delete", instance.Name))

	// remove db finalizer first
	db, err := mariadbv1.GetDatabaseByNameAndAccount(ctx, helper, glance.DatabaseName, instance.Spec.DatabaseAccount, instance.Namespace)
	if err != nil && !k8s_errors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	if !k8s_errors.IsNotFound(err) {
		if err := db.DeleteFinalizer(ctx, helper); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Remove the finalizer from our KeystoneService CR
	keystoneService, err := keystonev1.GetKeystoneServiceWithName(ctx, helper, glance.ServiceName, instance.Namespace)
	if err != nil && !k8s_errors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	if err == nil {
		if controllerutil.RemoveFinalizer(keystoneService, helper.GetFinalizer()) {
			err = r.Update(ctx, keystoneService)
			if err != nil && !k8s_errors.IsNotFound(err) {
				return ctrl.Result{}, err
			}
			util.LogForObject(helper, "Removed finalizer from our KeystoneService", instance)
		}
	}

	// Remove the finalizer on each GlanceAPI CR
	for name := range instance.Spec.GlanceAPIs {
		err = r.removeAPIFinalizer(ctx, instance, helper, name)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	// If the CR is removed (Glance undeployed/cleaned up), we usually
	// clean the resources created in the OpenStackControlPlane (e.g.,
	// we remove the database from mariadb, delete the service and the
	// endpoints in keystone). We should delete the limits created for
	// the Glance service, and do not leave leftovers in the ctlplane.

	// do not attempt to remove limits if keystoneAPI are not available
	_, err = keystonev1.GetKeystoneAPI(ctx, helper, instance.Namespace, map[string]string{})

	if err == nil && instance.IsQuotaEnabled() {
		err = r.registeredLimitsDelete(ctx, helper, instance)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	// Service is deleted so remove the finalizer.
	controllerutil.RemoveFinalizer(instance, helper.GetFinalizer())
	Log.Info(fmt.Sprintf("Reconciled Service '%s' delete successfully", instance.Name))

	return ctrl.Result{}, nil
}

// removeFinalizer - iterates over the supported GlanceAPI types and, if the
// associated resource is found, the finalizer is removed from the CR and the
// resource can be deleted
func (r *GlanceReconciler) removeAPIFinalizer(
	ctx context.Context,
	instance *glancev1.Glance,
	helper *helper.Helper,
	name string,
) error {
	var err error
	// Remove finalizers from any existing GlanceAPIs instance
	glanceAPI := &glancev1.GlanceAPI{}
	for _, apiType := range []string{glancev1.APIInternal, glancev1.APIExternal, glancev1.APISingle, glancev1.APIEdge} {
		err = r.Get(ctx, types.NamespacedName{Name: fmt.Sprintf("%s-%s-%s", glance.ServiceName, name, apiType), Namespace: instance.Namespace}, glanceAPI)
		if err != nil && !k8s_errors.IsNotFound(err) {
			return err
		}
		// GlanceAPI instance successfully found, remove the finalizer
		if err == nil {
			if controllerutil.RemoveFinalizer(glanceAPI, helper.GetFinalizer()) {
				err = r.Update(ctx, glanceAPI)
				if err != nil && !k8s_errors.IsNotFound(err) {
					return err
				}
				util.LogForObject(helper, fmt.Sprintf("Removed finalizer from GlanceAPI %s", glanceAPI.Name), glanceAPI)
			}
		}
	}
	return nil
}

func (r *GlanceReconciler) reconcileInit(
	ctx context.Context,
	instance *glancev1.Glance,
	helper *helper.Helper,
	serviceLabels map[string]string,
	serviceAnnotations map[string]string,
) (ctrl.Result, error) {
	Log := r.GetLogger(ctx)
	Log.Info(fmt.Sprintf("Reconciling Service '%s' init", instance.Name))

	//
	// create Keystone service and users - https://docs.openstack.org/Glance/latest/install/install-rdo.html#configure-user-and-endpoints
	//

	ksSvcSpec := keystonev1.KeystoneServiceSpec{
		ServiceType:        glance.ServiceType,
		ServiceName:        glance.ServiceName,
		ServiceDescription: "Glance Service",
		Enabled:            true,
		ServiceUser:        instance.Spec.ServiceUser,
		Secret:             instance.Spec.Secret,
		PasswordSelector:   instance.Spec.PasswordSelectors.Service,
	}

	ksSvc := keystonev1.NewKeystoneService(ksSvcSpec, instance.Namespace, serviceLabels, glance.NormalDuration)
	ctrlResult, err := ksSvc.CreateOrPatch(ctx, helper)
	if err != nil {
		return ctrlResult, err
	}

	// mirror the Status, Reason, Severity and Message of the latest keystoneservice condition
	// into a local condition with the type condition.KeystoneServiceReadyCondition
	c := ksSvc.GetConditions().Mirror(condition.KeystoneServiceReadyCondition)
	if c != nil {
		instance.Status.Conditions.Set(c)
	}

	if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	instance.Status.ServiceID = ksSvc.GetServiceID()

	if instance.Status.Hash == nil {
		instance.Status.Hash = map[string]string{}
	}

	//
	// run Glance db sync
	//
	dbSyncHash := instance.Status.Hash[glancev1.DbSyncHash]
	jobDef := glance.DbSyncJob(instance, serviceLabels, serviceAnnotations)
	dbSyncjob := job.NewJob(
		jobDef,
		glancev1.DbSyncHash,
		instance.Spec.PreserveJobs,
		glance.ShortDuration,
		dbSyncHash,
	)
	ctrlResult, err = dbSyncjob.DoJob(
		ctx,
		helper,
	)
	if (ctrlResult != ctrl.Result{}) {
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.DBSyncReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			condition.DBSyncReadyRunningMessage))
		return ctrlResult, nil
	}
	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.DBSyncReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			condition.DBSyncReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}
	if dbSyncjob.HasChanged() {
		instance.Status.Hash[glancev1.DbSyncHash] = dbSyncjob.GetHash()
		Log.Info(fmt.Sprintf("Service '%s' - Job %s hash added - %s", instance.Name, jobDef.Name, instance.Status.Hash[glancev1.DbSyncHash]))
	}
	instance.Status.Conditions.MarkTrue(condition.DBSyncReadyCondition, condition.DBSyncReadyMessage)
	// run Glance db sync - end

	// when job passed, mark NetworkAttachmentsReadyCondition ready, because we
	// pass NADs as serviceAnnotation to glance-dbsync job
	instance.Status.Conditions.MarkTrue(condition.NetworkAttachmentsReadyCondition, condition.NetworkAttachmentsReadyMessage)
	Log.Info(fmt.Sprintf("Reconciled Service '%s' init successfully", instance.Name))
	return ctrl.Result{}, nil
}

func (r *GlanceReconciler) reconcileNormal(ctx context.Context, instance *glancev1.Glance, helper *helper.Helper) (ctrl.Result, error) {
	Log := r.GetLogger(ctx)
	Log.Info(fmt.Sprintf("Reconciling Service '%s'", instance.Name))
	// Service account, role, binding
	rbacRules := []rbacv1.PolicyRule{
		{
			APIGroups:     []string{"security.openshift.io"},
			ResourceNames: []string{"anyuid", "privileged"},
			Resources:     []string{"securitycontextconstraints"},
			Verbs:         []string{"use"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"pods"},
			Verbs:     []string{"create", "get", "list", "watch", "update", "patch", "delete"},
		},
	}
	rbacResult, err := common_rbac.ReconcileRbac(ctx, helper, instance, rbacRules)
	if err != nil {
		return rbacResult, err
	} else if (rbacResult != ctrl.Result{}) {
		return rbacResult, nil
	}

	configVars := make(map[string]env.Setter)

	// ServiceLabels
	serviceLabels := map[string]string{
		common.AppSelector: glance.ServiceName,
	}

	//
	// create RabbitMQ transportURL CR and get the actual URL from the associated secret that is created
	//
	if instance.Spec.NotificationBusInstance != nil && *instance.Spec.NotificationBusInstance != "" {
		notificationTransportURL, op, err := r.transportURLCreateOrUpdate(ctx, instance, serviceLabels)
		if err != nil {
			instance.Status.Conditions.Set(condition.FalseCondition(
				condition.NotificationBusInstanceReadyCondition,
				condition.ErrorReason,
				condition.SeverityWarning,
				condition.NotificationBusInstanceReadyErrorMessage,
				err.Error()))
			return ctrl.Result{}, err
		}

		if op != controllerutil.OperationResultNone {
			Log.Info(fmt.Sprintf("TransportURL %s successfully reconciled - operation: %s", notificationTransportURL.Name, string(op)))
		}

		instance.Status.NotificationBusSecret = notificationTransportURL.Status.SecretName

		if instance.Status.NotificationBusSecret == "" {
			Log.Info(fmt.Sprintf("Waiting for notification TransportURL %s secret to be created", notificationTransportURL.Name))
			instance.Status.Conditions.Set(condition.FalseCondition(
				condition.NotificationBusInstanceReadyCondition,
				condition.RequestedReason,
				condition.SeverityInfo,
				condition.NotificationBusInstanceReadyRunningMessage))
			return glance.ResultRequeue, nil
		}
	} else {
		instance.Status.NotificationBusSecret = ""
	}
	// if we reach this point the condition can be marked as true by default
	instance.Status.Conditions.MarkTrue(condition.NotificationBusInstanceReadyCondition, condition.NotificationBusInstanceReadyMessage)
	// end transportURL

	//
	// Check for required memcached used for caching
	//
	var memcached *memcachedv1.Memcached
	memcached, err = memcachedv1.GetMemcachedByName(ctx, helper, instance.Spec.MemcachedInstance, instance.Namespace)
	if err != nil {
		if k8s_errors.IsNotFound(err) {
			// Memcached should be automatically created by the encompassing OpenStackControlPlane,
			// but we don't propagate its name into the "memcachedInstance" field of other sub-resources,
			// so we treat this as a warning because it means that the service will not be able to start.
			instance.Status.Conditions.Set(condition.FalseCondition(
				condition.MemcachedReadyCondition,
				condition.ErrorReason,
				condition.SeverityWarning,
				condition.MemcachedReadyWaitingMessage))
			Log.Info(fmt.Sprintf("%s...requeueing", condition.MemcachedReadyWaitingMessage))
			return glance.ResultRequeue, nil
		}
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.MemcachedReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			condition.MemcachedReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}

	if !memcached.IsReady() {
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.MemcachedReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			condition.MemcachedReadyWaitingMessage))
		Log.Info(fmt.Sprintf("%s...requeueing", condition.MemcachedReadyWaitingMessage))
		return glance.ResultRequeue, nil
	}
	// Mark the Memcached Service as Ready if we get to this point with no errors
	instance.Status.Conditions.MarkTrue(
		condition.MemcachedReadyCondition, condition.MemcachedReadyMessage)
	// run check memcached - end

	//
	// check for required OpenStack secret holding passwords for service/admin user and add hash to the vars map
	//
	ctrlResult, err := verifyServiceSecret(
		ctx,
		types.NamespacedName{Namespace: instance.Namespace, Name: instance.Spec.Secret},
		[]string{
			instance.Spec.PasswordSelectors.Service,
		},
		helper.GetClient(),
		&instance.Status.Conditions,
		glance.NormalDuration,
		&configVars,
	)
	if (err != nil || ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	instance.Status.Conditions.MarkTrue(condition.InputReadyCondition, condition.InputReadyMessage)
	// run check OpenStack secret - end

	db, ctrlResult, err := r.ensureDB(ctx, helper, instance)
	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.DBReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			condition.DBReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	} else if (ctrlResult != ctrl.Result{}) {
		instance.Status.Conditions.Set(condition.UnknownCondition(
			condition.DBReadyCondition,
			condition.RequestedReason,
			condition.DBReadyRunningMessage))
		return ctrlResult, nil
	}

	//
	// Create Secrets required as input for the Service and calculate an overall hash of hashes
	//

	//
	err = r.generateServiceConfig(ctx, helper, instance, &configVars, serviceLabels, db)
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

	var serviceAnnotations map[string]string
	// networks to attach to
	for _, glanceAPI := range instance.Spec.GlanceAPIs {
		serviceAnnotations, ctrlResult, err = ensureNAD(ctx, &instance.Status.Conditions, glanceAPI.NetworkAttachments, helper)
		if err != nil {
			return ctrlResult, err
		}
	}
	// Handle service init
	ctrlResult, err = r.reconcileInit(ctx, instance, helper, serviceLabels, serviceAnnotations)
	if err != nil {
		return ctrlResult, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	//
	// Reconcile the GlanceAPI deployment
	//
	for name, glanceAPI := range instance.Spec.GlanceAPIs {
		err = r.apiDeployment(ctx, instance, name, glanceAPI, helper, serviceLabels)
		if err != nil {
			return ctrl.Result{}, err
		}
	}
	err = r.glanceAPICleanup(ctx, instance)
	if err != nil {
		return ctrl.Result{}, err
	}

	// remove finalizers from unused MariaDBAccount records
	err = mariadbv1.DeleteUnusedMariaDBAccountFinalizers(ctx, helper, glance.DatabaseName, instance.Spec.DatabaseAccount, instance.Namespace)
	if err != nil {
		return ctrl.Result{}, err
	}

	// create DBPurge CronJob

	// DBPurge is not optional and always created to purge all soft deleted
	// records. This command should be executed periodically to avoid glance
	// database becomes bigger by getting filled by soft-deleted records
	ctrlResult, err = r.ensureDBPurgeJob(ctx, helper, instance, serviceLabels, serviceAnnotations)
	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.CronJobReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			condition.CronJobReadyErrorMessage,
			err.Error()))
		return ctrlResult, err
	}
	instance.Status.Conditions.MarkTrue(condition.CronJobReadyCondition, condition.CronJobReadyMessage)
	// create CronJob - end

	// We reached the end of the Reconcile, update the Ready condition based on
	// the sub conditions
	subGen, err := r.checkGlanceAPIsGeneration(ctx, instance)
	if err != nil {
		return ctrlResult, err
	}
	if instance.Status.Conditions.AllSubConditionIsTrue() && subGen {
		instance.Status.Conditions.MarkTrue(
			condition.ReadyCondition, condition.ReadyMessage)
	}
	return ctrl.Result{}, nil
}

// apiDeployment represents the logic of deploying GlanceAPI instances specified
// in the main CR according to a given strategy (split vs single). It handles
// the deployment logic itself, as well as the output settings mirrored in the
// main Glance CR status
func (r *GlanceReconciler) apiDeployment(
	ctx context.Context,
	instance *glancev1.Glance,
	instanceName string,
	current glancev1.GlanceAPITemplate,
	helper *helper.Helper,
	serviceLabels map[string]string,
) error {
	Log := r.GetLogger(ctx)

	// By default internal and external points to diff instances, but we might
	// want to override "external" with "single" in case APIType == "single":
	// in this case we only deploy the External instance and skip the internal
	// one
	var internal = glancev1.APIInternal
	var external = glancev1.APIExternal
	var wsgi bool

	// We deploy:
	// - an internal + external instances if layout is Split
	// - an external instance only if layout is Single (File / NFS backends)
	// - an internal instance only if layout is Edge

	// If we're deploying a "single" instance, we skip GlanceAPI.Internal, and
	// we only deploy the External instance passing "glancev1.APISingle" to the
	// GlanceAPI controller, so we can properly handle this use case (nad and
	// service creation).
	if current.Type == glancev1.APISingle {
		external = glancev1.APISingle
	}
	if current.Type == glancev1.APIEdge {
		external = glancev1.APIEdge
	}

	// Retrieve WSGI annotation and fail if the format is not what is expected
	// by the glance-operator
	wsgi, exists, err := annotations.GetBoolFromAnnotation(
		instance.GetAnnotations(), glancev1.GlanceWSGILabel)
	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			glancev1.GlanceAPIReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			glancev1.GlanceAPIReadyErrorMessage,
			err.Error()))
		return err
	}
	// If there's no associated annotation, glance-operators defaults to WSGI
	// and we explicitly set this variable to true. As a result of this, the
	// glance.openstack.org/wsgi annotation is applied to the underlying
	// resources and glance is deplyed in WSGI mode.
	if !exists {
		wsgi = true
	}

	glanceAPI, op, err := r.apiDeploymentCreateOrUpdate(
		ctx,
		instance,
		instance.Spec.GlanceAPIs[instanceName],
		external,
		instanceName,
		helper,
		serviceLabels,
		wsgi,
	)
	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			glancev1.GlanceAPIReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			glancev1.GlanceAPIReadyErrorMessage,
			err.Error()))
		return err
	}
	if op != controllerutil.OperationResultNone {
		Log.Info(fmt.Sprintf("StatefulSet %s successfully reconciled - operation: %s", instance.Name, string(op)))
	}
	if instance.Status.GlanceAPIReadyCounts == nil {
		instance.Status.GlanceAPIReadyCounts = map[string]int32{}
	}
	instance.Status.GlanceAPIReadyCounts[instanceName] = glanceAPI.Status.ReadyCount

	apiPubEndpoint := fmt.Sprintf("%s-%s", instanceName, string(endpoint.EndpointPublic))
	apiIntEndpoint := fmt.Sprintf("%s-%s", instanceName, string(endpoint.EndpointInternal))
	// Mirror single/external GlanceAPI status' APIEndpoints and ReadyCount to this parent CR
	if glanceAPI.Status.APIEndpoints != nil {
		// Do not register a public endpoint for Edge instances
		if current.Type != glancev1.APIEdge {
			instance.Status.APIEndpoints[apiPubEndpoint] = glanceAPI.Status.APIEndpoints[string(endpoint.EndpointPublic)]
		}
		// if we don't split, both apiEndpoints (public and internal) should be
		// reflected to the main Glance CR
		if current.Type == glancev1.APISingle || current.Type == glancev1.APIEdge {
			instance.Status.APIEndpoints[apiIntEndpoint] = glanceAPI.Status.APIEndpoints[string(endpoint.EndpointInternal)]
		}
	}

	// Get external GlanceAPI's condition status and compare it against priority of internal GlanceAPI's condition
	apiCondition := glanceAPI.Status.Conditions.Mirror(glancev1.GlanceAPIReadyCondition)

	// split is the default use case unless type: "single" is passed to the top
	// level CR: in this case we deploy an additional glanceAPI instance (Internal)
	if current.Type == "split" || len(current.Type) == 0 {
		// we force "internal" here by passing glancev1.APIInternal for the apiType arg
		glanceAPI, op, err := r.apiDeploymentCreateOrUpdate(
			ctx,
			instance,
			instance.Spec.GlanceAPIs[instanceName],
			internal,
			instanceName,
			helper,
			serviceLabels,
			wsgi,
		)
		if err != nil {
			instance.Status.Conditions.Set(condition.FalseCondition(
				glancev1.GlanceAPIReadyCondition,
				condition.ErrorReason,
				condition.SeverityWarning,
				glancev1.GlanceAPIReadyErrorMessage,
				err.Error()))
			return err
		}
		if op != controllerutil.OperationResultNone {
			Log.Info(fmt.Sprintf("StatefulSet %s successfully reconciled - operation: %s", instance.Name, string(op)))
		}

		// It is possible that an earlier call to update the status has also set
		// APIEndpoints to nil (if the APIEndpoints map was not nil but was empty,
		// saving the status unfortunately re-initializes it as nil)
		if instance.Status.APIEndpoints == nil {
			instance.Status.APIEndpoints = map[string]string{}
		}

		// Mirror internal GlanceAPI status' APIEndpoints and ReadyCount to this parent CR
		if glanceAPI.Status.APIEndpoints != nil {
			instance.Status.APIEndpoints[apiIntEndpoint] = glanceAPI.Status.APIEndpoints[string(endpoint.EndpointInternal)]
		}

		// Get internal GlanceAPI's condition status for comparison with external below
		internalAPICondition := glanceAPI.Status.Conditions.Mirror(glancev1.GlanceAPIReadyCondition)
		apiCondition = condition.GetHigherPrioCondition(internalAPICondition, apiCondition).DeepCopy()
	}

	if apiCondition != nil {
		instance.Status.Conditions.Set(apiCondition)
	}

	Log.Info(fmt.Sprintf("Reconciled Service '%s' successfully", instance.Name))
	return nil
}

func (r *GlanceReconciler) apiDeploymentCreateOrUpdate(
	ctx context.Context,
	instance *glancev1.Glance,
	apiTemplate glancev1.GlanceAPITemplate,
	apiType string,
	apiName string,
	helper *helper.Helper,
	serviceLabels map[string]string,
	wsgi bool,
) (*glancev1.GlanceAPI, controllerutil.OperationResult, error) {
	apiAnnotations := map[string]string{}
	apiSpec := glancev1.GlanceAPISpec{
		GlanceAPITemplate:     apiTemplate,
		APIType:               apiType,
		DatabaseHostname:      instance.Status.DatabaseHostname,
		DatabaseAccount:       instance.Spec.DatabaseAccount,
		Secret:                instance.Spec.Secret,
		ExtraMounts:           instance.Spec.ExtraMounts,
		PasswordSelectors:     instance.Spec.PasswordSelectors,
		ServiceUser:           instance.Spec.ServiceUser,
		ServiceAccount:        instance.RbacResourceName(),
		Quota:                 instance.IsQuotaEnabled(),
		NotificationBusSecret: instance.Status.NotificationBusSecret,
		MemcachedInstance:     instance.Spec.MemcachedInstance,
	}

	if apiSpec.NodeSelector == nil {
		apiSpec.NodeSelector = instance.Spec.NodeSelector
	}

	// Inherit the ImageCacheSize from the top level if not specified
	if apiSpec.ImageCache.Size == "" {
		apiSpec.ImageCache.Size = instance.Spec.ImageCache.Size
	}

	// Inherit the values required for PVC creation from the top-level CR
	if apiSpec.Storage.StorageRequest == "" {
		apiSpec.Storage.StorageRequest = instance.Spec.Storage.StorageRequest
	}
	if apiSpec.Storage.StorageClass == "" {
		apiSpec.Storage.StorageClass = instance.Spec.Storage.StorageClass
	}
	if !apiSpec.Storage.External {
		apiSpec.Storage.External = instance.Spec.Storage.External
	}

	// Make sure to inject the ContainerImage passed by the OpenStackVersions
	// resource to all the underlying instances and rollout a new StatefulSet
	// if it has been changed
	apiSpec.ContainerImage = instance.Spec.ContainerImage

	// We select which glanceAPI should register the keystoneEndpoint by using
	// an API selector defined in the main glance CR; if it matches with the
	// current APIName, an annotation is added to the glanceAPI instance
	if instance.Spec.KeystoneEndpoint == apiName {
		apiAnnotations[glance.KeystoneEndpoint] = "true"
	}

	// If topology is not present in the underlying GlanceAPI,
	// inherit from the top-level CR
	if apiSpec.TopologyRef == nil {
		apiSpec.TopologyRef = instance.Spec.TopologyRef
	}

	// Set deployment mode (proxypass vs mod_wsgi)
	apiAnnotations[glancev1.GlanceWSGILabel] = strconv.FormatBool(wsgi)

	// Add the API name to the GlanceAPI instance as a label
	serviceLabels[glancev1.APINameLabel] = apiName
	glanceStatefulset := &glancev1.GlanceAPI{
		ObjectMeta: metav1.ObjectMeta{
			Name:        fmt.Sprintf("%s-%s-%s", instance.Name, apiName, apiType),
			Annotations: apiAnnotations,
			Labels:      serviceLabels,
			Namespace:   instance.Namespace,
		},
	}
	defer delete(serviceLabels, glancev1.APINameLabel)
	op, err := controllerutil.CreateOrUpdate(ctx, r.Client, glanceStatefulset, func() error {
		// Assign the created spec containing both field provided via GlanceAPITemplate
		// and what is inherited from the top-level CR (ExtraMounts)
		glanceStatefulset.Annotations = apiAnnotations
		glanceStatefulset.Spec = apiSpec

		// We might want to create instances pointing to different backends in
		// the future, hence we inherit the customServiceConfig (where the backends
		// are defined) only if it's not specified in the GlanceAPITemplate.
		// Same comment applies to CustomServiceConfigSecrets
		if len(glanceStatefulset.Spec.CustomServiceConfig) == 0 {
			glanceStatefulset.Spec.CustomServiceConfig = instance.Spec.CustomServiceConfig
		}

		if len(glanceStatefulset.Spec.CustomServiceConfigSecrets) == 0 {
			glanceStatefulset.Spec.CustomServiceConfigSecrets = instance.Spec.CustomServiceConfigSecrets
		}

		// QuotaLimits are global values for Glance service in keystone, it's not
		// supported having different quotas per Glance instances, hence we always
		// inherit this parameter from the parent CR
		if instance.IsQuotaEnabled() {
			err := r.ensureRegisteredLimits(ctx, helper, instance, instance.GetQuotaLimits())
			if err != nil {
				return err
			}
		}

		err := controllerutil.SetControllerReference(instance, glanceStatefulset, r.Scheme)
		if err != nil {
			return err
		}
		return nil
	})

	return glanceStatefulset, op, err
}

// generateServiceConfig - create secrets which hold scripts and service configuration (*used for DBSync only*)
func (r *GlanceReconciler) generateServiceConfig(
	ctx context.Context,
	h *helper.Helper,
	instance *glancev1.Glance,
	envVars *map[string]env.Setter,
	serviceLabels map[string]string,
	db *mariadbv1.Database,
) error {
	labels := labels.GetLabels(instance, labels.GetGroupLabel(glance.ServiceName), serviceLabels)

	databaseAccount := db.GetAccount()
	dbSecret := db.GetSecret()

	// We only need a minimal 00-config.conf that is only used by db-sync job,
	// hence only passing the database related parameters
	templateParameters := map[string]any{
		"MinimalConfig": true, // This tells the template to generate a minimal config
		"DatabaseConnection": fmt.Sprintf("mysql+pymysql://%s:%s@%s/%s?read_default_file=/etc/my.cnf",
			databaseAccount.Spec.UserName,
			string(dbSecret.Data[mariadbv1.DatabasePasswordSelector]),
			instance.Status.DatabaseHostname,
			glance.DatabaseName,
		),
	}

	// We set in the main 00-config-default.conf the image-cache bits that will
	// be used by CronJobs cleaner and pruner
	if len(instance.Spec.ImageCache.Size) > 0 {
		// if ImageCache.Size is not a valid k8s Quantity, return an error
		cacheSize, err := resource.ParseQuantity(instance.Spec.ImageCache.Size)
		if err != nil {
			return err
		}
		templateParameters["CacheEnabled"] = true
		templateParameters["CacheMaxSize"] = cacheSize.Value()
		templateParameters["ImageCacheDir"] = glance.ImageCacheDir
	}

	var tlsCfg *tls.Service
	for _, api := range instance.Spec.GlanceAPIs {
		if api.TLS.CaBundleSecretName != "" {
			tlsCfg = &tls.Service{}
			break
		}
	}
	customData := map[string]string{
		glance.CustomConfigFileName: instance.Spec.CustomServiceConfig,
		"my.cnf":                    db.GetDatabaseClientConfig(tlsCfg), //(mschuppert) for now just get the default my.cnf
	}

	// Generate both default 00-config.conf and -scripts
	return GenerateConfigsGeneric(ctx, h, instance, envVars, templateParameters, customData, labels, true)
}

// ensureRegisteredLimits - create registered limits in keystone that will be
// used by glance to enforce per-tenant quotas
func (r *GlanceReconciler) ensureRegisteredLimits(
	ctx context.Context,
	h *helper.Helper,
	instance *glancev1.Glance,
	quota map[string]int,
) error {
	Log := r.GetLogger(ctx)
	// get admin
	var err error
	keystoneAPI, err := keystonev1.GetKeystoneAPI(ctx, h, instance.Namespace, map[string]string{})
	if err != nil {
		return err
	}
	scope := &gophercloud.AuthScope{System: true}
	o, _, err := keystonev1.GetScopedAdminServiceClient(ctx, h, keystoneAPI, scope)
	if err != nil {
		return err
	}
	for lName, lValue := range quota {
		defaultRegion := o.GetRegion()
		m := openstack.RegisteredLimit{
			RegionID:     defaultRegion,
			ServiceID:    instance.Status.ServiceID,
			Description:  "default limit for  " + lName,
			ResourceName: lName,
			DefaultLimit: lValue,
		}
		_, err = o.CreateOrUpdateRegisteredLimit(Log, m)
		if err != nil {
			return err
		}
	}
	return nil
}

// ensureCronJobs - Create the required CronJobs to clean DB entries and image-cache
// if enabled
func (r *GlanceReconciler) ensureDBPurgeJob(
	ctx context.Context,
	h *helper.Helper,
	instance *glancev1.Glance,
	serviceLabels map[string]string,
	serviceAnnotations map[string]string,
) (ctrl.Result, error) {

	cronSpec := glance.CronJobSpec{
		Name:        fmt.Sprintf("%s-db-purge", instance.Name),
		PvcClaim:    nil,
		Command:     glance.GlanceManage,
		Schedule:    instance.Spec.DBPurge.Schedule,
		CjType:      glance.DBPurge,
		Labels:      serviceLabels,
		Annotations: serviceAnnotations,
	}

	cronjobDef := glance.DBPurgeJob(
		instance,
		cronSpec,
	)

	dbPurgeCronJob := cronjob.NewCronJob(
		cronjobDef,
		glance.ShortDuration,
	)
	ctrlResult, err := dbPurgeCronJob.CreateOrPatch(ctx, h)
	if err != nil {
		return ctrlResult, err
	}

	return ctrlResult, err
}

// registeredLimitsDelete - cleanup registered limits in keystone
func (r *GlanceReconciler) registeredLimitsDelete(
	ctx context.Context,
	h *helper.Helper,
	instance *glancev1.Glance,
) error {
	Log := r.GetLogger(ctx)
	// get admin
	var err error
	keystoneAPI, err := keystonev1.GetKeystoneAPI(ctx, h, instance.Namespace, map[string]string{})
	if err != nil {
		return err
	}
	scope := &gophercloud.AuthScope{System: true}
	o, _, err := keystonev1.GetScopedAdminServiceClient(ctx, h, keystoneAPI, scope)
	if err != nil {
		return err
	}
	fetchRegLimits, err := o.ListRegisteredLimitsByServiceID(Log, instance.Status.ServiceID)
	if err != nil {
		return err
	}
	for _, l := range fetchRegLimits {
		err = o.DeleteRegisteredLimit(Log, l.ID)
		if err != nil {
			return err
		}
	}
	return nil
}

// GlanceAPICleanup - Delete the glanceAPI instance if it no longer appears
// in the spec.
func (r *GlanceReconciler) glanceAPICleanup(ctx context.Context, instance *glancev1.Glance) error {
	Log := r.GetLogger(ctx)
	// Generate a list of GlanceAPI CRs
	apis := &glancev1.GlanceAPIList{}
	listOpts := []client.ListOption{
		client.InNamespace(instance.Namespace),
	}
	if err := r.List(ctx, apis, listOpts...); err != nil {
		Log.Error(err, "Unable to retrieve GlanceAPI CRs %v")
		return nil
	}
	for _, glanceAPI := range apis.Items {
		// Skip any GlanceAPI that we don't own
		if glance.GetOwningGlanceName(&glanceAPI) != instance.Name {
			continue
		}
		apiName := glanceAPI.APIName()
		// Simply return if the apiName doesn't match the existing pattern, log but do not
		// raise an error
		if apiName == "" {
			Log.Info(fmt.Sprintf("GlanceAPI %s does not match the pattern", glanceAPI.Name))
			return nil
		}
		_, exists := instance.Spec.GlanceAPIs[apiName]
		// Delete the api if it's no longer in the spec
		if !exists && glanceAPI.DeletionTimestamp.IsZero() {
			err := r.Delete(ctx, &glanceAPI)
			if err != nil && !k8s_errors.IsNotFound(err) {
				err = fmt.Errorf("error cleaning up %s: %w", glanceAPI.Name, err)
				return err
			}
			// Update the APIEndpoints in the top-level CR
			endpoints := []endpoint.Endpoint{endpoint.EndpointPublic, endpoint.EndpointInternal}
			for _, ep := range endpoints {
				endpointKey := fmt.Sprintf("%s-%s", apiName, ep)
				delete(instance.Status.APIEndpoints, endpointKey)
			}
			delete(instance.Status.GlanceAPIReadyCounts, apiName)
		}
	}
	return nil
}

func (r *GlanceReconciler) ensureDB(
	ctx context.Context,
	h *helper.Helper,
	instance *glancev1.Glance,
) (*mariadbv1.Database, ctrl.Result, error) {
	// ensure a MariaDBAccount CR and Secret exist.
	// this mariadb API function will as an interim step actually generate a
	// new MariaDBAccount and Secret if one does not exist already.   in a
	// future release, this function may change to emit an error if the
	// MariaDBAccount was not already created ahead of time (e.g. by openstack-operator
	// or end-user YAML declaration)
	_, _, err := mariadbv1.EnsureMariaDBAccount(
		ctx, h, instance.Spec.DatabaseAccount,
		instance.Namespace, false, "glance",
	)

	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			mariadbv1.MariaDBAccountReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			mariadbv1.MariaDBAccountNotReadyMessage,
			err.Error()))

		return nil, ctrl.Result{}, err
	}
	instance.Status.Conditions.MarkTrue(
		mariadbv1.MariaDBAccountReadyCondition,
		mariadbv1.MariaDBAccountReadyMessage,
	)

	// create service DB instance
	//
	db := mariadbv1.NewDatabaseForAccount(
		instance.Spec.DatabaseInstance, // mariadb/galera service to target
		glance.DatabaseName,            // name used in CREATE DATABASE in mariadb
		glance.DatabaseName,            // CR name for MariaDBDatabase
		instance.Spec.DatabaseAccount,  // CR name for MariaDBAccount
		instance.Namespace,             // namespace
	)

	// create or patch the DB
	ctrlResult, err := db.CreateOrPatchAll(ctx, h)

	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.DBReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			condition.DBReadyErrorMessage,
			err.Error()))
		return db, ctrl.Result{}, err
	}
	if (ctrlResult != ctrl.Result{}) {
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.DBReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			condition.DBReadyRunningMessage))
		return db, ctrlResult, nil
	}
	// wait for the DB to be setup
	// (ksambor) should we use WaitForDBCreatedWithTimeout instead?
	ctrlResult, err = db.WaitForDBCreated(ctx, h)
	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.DBReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			condition.DBReadyErrorMessage,
			err.Error()))
		return db, ctrlResult, err
	}
	if (ctrlResult != ctrl.Result{}) {
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.DBReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			condition.DBReadyRunningMessage))
		return db, ctrlResult, nil
	}

	// update Status.DatabaseHostname, used to config the service
	instance.Status.DatabaseHostname = db.GetDatabaseHostname()
	instance.Status.Conditions.MarkTrue(condition.DBReadyCondition, condition.DBReadyMessage)
	return db, ctrlResult, nil
}

// checkGlanceAPIsGeneration -
func (r *GlanceReconciler) checkGlanceAPIsGeneration(
	ctx context.Context,
	instance *glancev1.Glance,
) (bool, error) {
	Log := r.GetLogger(ctx)
	// get all GlanceAPI CRs
	glances := &glancev1.GlanceAPIList{}
	listOpts := []client.ListOption{
		client.InNamespace(instance.Namespace),
	}
	if err := r.List(ctx, glances, listOpts...); err != nil {
		Log.Error(err, "Unable to retrieve Glance CRs %w")
		return false, err
	}
	for _, item := range glances.Items {
		if item.Generation != item.Status.ObservedGeneration {
			return false, nil
		}
	}
	return true, nil
}

// transportURLCreateOrUpdate -
func (r *GlanceReconciler) transportURLCreateOrUpdate(
	ctx context.Context,
	instance *glancev1.Glance,
	serviceLabels map[string]string,
) (*rabbitmqv1.TransportURL, controllerutil.OperationResult, error) {
	transportURL := &rabbitmqv1.TransportURL{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-glance-transport", instance.Name),
			Namespace: instance.Namespace,
			Labels:    serviceLabels,
		},
	}

	op, err := controllerutil.CreateOrUpdate(ctx, r.Client, transportURL, func() error {
		transportURL.Spec.RabbitmqClusterName = *instance.Spec.NotificationBusInstance

		err := controllerutil.SetControllerReference(instance, transportURL, r.Scheme)
		return err
	})

	return transportURL, op, err
}
