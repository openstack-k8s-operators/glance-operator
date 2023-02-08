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
	"time"

	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/go-logr/logr"
	glancev1 "github.com/openstack-k8s-operators/glance-operator/api/v1beta1"
	"github.com/openstack-k8s-operators/glance-operator/pkg/glance"
	keystonev1 "github.com/openstack-k8s-operators/keystone-operator/api/v1beta1"
	"github.com/openstack-k8s-operators/lib-common/modules/common"
	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/configmap"
	"github.com/openstack-k8s-operators/lib-common/modules/common/endpoint"
	"github.com/openstack-k8s-operators/lib-common/modules/common/env"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	"github.com/openstack-k8s-operators/lib-common/modules/common/job"
	"github.com/openstack-k8s-operators/lib-common/modules/common/labels"
	"github.com/openstack-k8s-operators/lib-common/modules/common/pvc"
	oko_secret "github.com/openstack-k8s-operators/lib-common/modules/common/secret"
	"github.com/openstack-k8s-operators/lib-common/modules/common/util"
	"github.com/openstack-k8s-operators/lib-common/modules/database"
	mariadbv1 "github.com/openstack-k8s-operators/mariadb-operator/api/v1beta1"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetClient -
func (r *GlanceReconciler) GetClient() client.Client {
	return r.Client
}

// GetKClient -
func (r *GlanceReconciler) GetKClient() kubernetes.Interface {
	return r.Kclient
}

// GetLogger -
func (r *GlanceReconciler) GetLogger() logr.Logger {
	return r.Log
}

// GetScheme -
func (r *GlanceReconciler) GetScheme() *runtime.Scheme {
	return r.Scheme
}

// GlanceReconciler reconciles a Glance object
type GlanceReconciler struct {
	client.Client
	Kclient kubernetes.Interface
	Log     logr.Logger
	Scheme  *runtime.Scheme
}

//+kubebuilder:rbac:groups=glance.openstack.org,resources=glances,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=glance.openstack.org,resources=glances/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=glance.openstack.org,resources=glances/finalizers,verbs=update
// +kubebuilder:rbac:groups=glance.openstack.org,resources=glanceapis,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=glance.openstack.org,resources=glanceapis/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=glance.openstack.org,resources=glanceapis/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;create;update;delete;watch
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete;
// +kubebuilder:rbac:groups=mariadb.openstack.org,resources=mariadbdatabases,verbs=get;list;watch;create;update;patch;delete;
// +kubebuilder:rbac:groups=keystone.openstack.org,resources=keystoneapis,verbs=get;list;watch;
// +kubebuilder:rbac:groups=keystone.openstack.org,resources=keystoneservices,verbs=get;list;watch;create;update;patch;delete;

// Reconcile reconcile Glance requests
func (r *GlanceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, _err error) {
	_ = log.FromContext(ctx)

	// Fetch the Glance instance
	instance := &glancev1.Glance{}
	err := r.Client.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if k8s_errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected.
			// For additional cleanup logic use finalizers. Return and don't requeue.
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
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
		return ctrl.Result{}, err
	}

	// Always patch the instance status when exiting this function so we can persist any changes.
	defer func() {
		// update the overall status condition if service is ready
		if instance.IsReady() {
			instance.Status.Conditions.MarkTrue(condition.ReadyCondition, condition.ReadyMessage)
		}

		err := helper.PatchInstance(ctx, instance)
		if err != nil {
			_err = err
			return
		}
	}()

	// If we're not deleting this and the service object doesn't have our finalizer, add it.
	if instance.DeletionTimestamp.IsZero() && controllerutil.AddFinalizer(instance, helper.GetFinalizer()) {
		return ctrl.Result{}, nil
	}

	//
	// initialize status
	//
	if instance.Status.Conditions == nil {
		instance.Status.Conditions = condition.Conditions{}
		// initialize conditions used later as Status=Unknown
		cl := condition.CreateList(
			condition.UnknownCondition(condition.DBReadyCondition, condition.InitReason, condition.DBReadyInitMessage),
			condition.UnknownCondition(condition.DBSyncReadyCondition, condition.InitReason, condition.DBSyncReadyInitMessage),
			condition.UnknownCondition(condition.InputReadyCondition, condition.InitReason, condition.InputReadyInitMessage),
			condition.UnknownCondition(condition.ServiceConfigReadyCondition, condition.InitReason, condition.ServiceConfigReadyInitMessage),
			condition.UnknownCondition(glancev1.GlanceAPIReadyCondition, condition.InitReason, glancev1.GlanceAPIReadyInitMessage),
			// right now we have no dedicated KeystoneServiceReadyInitMessage
			condition.UnknownCondition(condition.KeystoneServiceReadyCondition, condition.InitReason, ""),
		)

		instance.Status.Conditions.Init(&cl)

		// Register overall status immediately to have an early feedback e.g. in the cli
		return ctrl.Result{}, nil
	}
	if instance.Status.Hash == nil {
		instance.Status.Hash = map[string]string{}
	}
	if instance.Status.APIEndpoints == nil {
		instance.Status.APIEndpoints = map[string]string{}
	}

	// Handle service delete
	if !instance.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, instance, helper)
	}

	// Handle non-deleted clusters
	return r.reconcileNormal(ctx, instance, helper)
}

// SetupWithManager sets up the controller with the Manager.
func (r *GlanceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&glancev1.Glance{}).
		Owns(&glancev1.GlanceAPI{}).
		Owns(&mariadbv1.MariaDBDatabase{}).
		Owns(&keystonev1.KeystoneService{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Owns(&batchv1.Job{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.Secret{}).
		Complete(r)
}

func (r *GlanceReconciler) reconcileDelete(ctx context.Context, instance *glancev1.Glance, helper *helper.Helper) (ctrl.Result, error) {
	r.Log.Info(fmt.Sprintf("Reconciling Service '%s' delete", instance.Name))

	// remove db finalizer first
	db, err := database.GetDatabaseByName(ctx, helper, instance.Name)
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
		controllerutil.RemoveFinalizer(keystoneService, helper.GetFinalizer())
		if err = helper.GetClient().Update(ctx, keystoneService); err != nil && !k8s_errors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
		util.LogForObject(helper, "Removed finalizer from our KeystoneService", instance)
	}

	// Remove finalizers from any existing child GlanceAPIs
	for _, apiType := range []string{glancev1.APIInternal, glancev1.APIExternal} {
		glanceAPI := &glancev1.GlanceAPI{}
		err = helper.GetClient().Get(ctx, types.NamespacedName{Name: fmt.Sprintf("%s-%s", instance.Name, apiType), Namespace: instance.Namespace}, glanceAPI)
		if err != nil && !k8s_errors.IsNotFound(err) {
			return ctrl.Result{}, err
		}

		if err == nil {
			controllerutil.RemoveFinalizer(glanceAPI, helper.GetFinalizer())
			if err = helper.GetClient().Update(ctx, glanceAPI); err != nil && !k8s_errors.IsNotFound(err) {
				return ctrl.Result{}, err
			}
			util.LogForObject(helper, fmt.Sprintf("Removed finalizer from GlanceAPI %s", glanceAPI.Name), glanceAPI)

			// if err = helper.GetClient().Delete(ctx, glanceAPI); err != nil && !k8s_errors.IsNotFound(err) {
			// 	return ctrl.Result{}, err
			// }
			// util.LogForObject(helper, fmt.Sprintf("Deleted GlanceAPI %s", glanceAPI.Name), glanceAPI)
		}
	}

	// Service is deleted so remove the finalizer.
	controllerutil.RemoveFinalizer(instance, helper.GetFinalizer())
	r.Log.Info(fmt.Sprintf("Reconciled Service '%s' delete successfully", instance.Name))

	return ctrl.Result{}, nil
}

func (r *GlanceReconciler) reconcileInit(
	ctx context.Context,
	instance *glancev1.Glance,
	helper *helper.Helper,
	serviceLabels map[string]string,
) (ctrl.Result, error) {
	r.Log.Info(fmt.Sprintf("Reconciling Service '%s' init", instance.Name))

	// Define a new PVC object
	// TODO: Once conditions added to PVC lib-common logic, handle
	//       the returned condition here
	// TODO: This could be superfluous if backed by Ceph?
	pvc := pvc.NewPvc(
		glance.Pvc(instance, serviceLabels),
		time.Duration(5)*time.Second,
	)

	ctrlResult, err := pvc.CreateOrPatch(ctx, helper)

	if err != nil {
		return ctrlResult, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}
	// End PVC creation/patch

	//
	// create service DB instance
	//
	db := database.NewDatabase(
		instance.Name,
		instance.Spec.DatabaseUser,
		instance.Spec.Secret,
		map[string]string{
			"dbName": instance.Spec.DatabaseInstance,
		},
	)
	// create or patch the DB
	ctrlResult, err = db.CreateOrPatchDB(
		ctx,
		helper,
	)
	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.DBReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			condition.DBReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}
	if (ctrlResult != ctrl.Result{}) {
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.DBReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			condition.DBReadyRunningMessage))
		return ctrlResult, nil
	}
	// wait for the DB to be setup
	ctrlResult, err = db.WaitForDBCreated(ctx, helper)
	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.DBReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			condition.DBReadyErrorMessage,
			err.Error()))
		return ctrlResult, err
	}
	if (ctrlResult != ctrl.Result{}) {
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.DBReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			condition.DBReadyRunningMessage))
		return ctrlResult, nil
	}
	// update Status.DatabaseHostname, used to config the service
	instance.Status.DatabaseHostname = db.GetDatabaseHostname()
	instance.Status.Conditions.MarkTrue(condition.DBReadyCondition, condition.DBReadyMessage)
	// create service DB - end

	//
	// create Keystone service and users - https://docs.openstack.org/Glance/latest/install/install-rdo.html#configure-user-and-endpoints
	//
	_, _, err = oko_secret.GetSecret(ctx, helper, instance.Spec.Secret, instance.Namespace)
	if err != nil {
		if k8s_errors.IsNotFound(err) {
			return ctrl.Result{RequeueAfter: time.Duration(10) * time.Second}, fmt.Errorf("OpenStack secret %s not found", instance.Spec.Secret)
		}
		return ctrl.Result{}, err
	}

	ksSvcSpec := keystonev1.KeystoneServiceSpec{
		ServiceType:        glance.ServiceType,
		ServiceName:        glance.ServiceName,
		ServiceDescription: "Glance Service",
		Enabled:            true,
		ServiceUser:        instance.Spec.ServiceUser,
		Secret:             instance.Spec.Secret,
		PasswordSelector:   instance.Spec.PasswordSelectors.Service,
	}

	ksSvc := keystonev1.NewKeystoneService(ksSvcSpec, instance.Namespace, serviceLabels, time.Duration(10)*time.Second)
	ctrlResult, err = ksSvc.CreateOrPatch(ctx, helper)
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
	jobDef := glance.DbSyncJob(instance, serviceLabels)
	dbSyncjob := job.NewJob(
		jobDef,
		glancev1.DbSyncHash,
		instance.Spec.PreserveJobs,
		time.Duration(5)*time.Second,
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
		r.Log.Info(fmt.Sprintf("Service '%s' - Job %s hash added - %s", instance.Name, jobDef.Name, instance.Status.Hash[glancev1.DbSyncHash]))
	}
	instance.Status.Conditions.MarkTrue(condition.DBSyncReadyCondition, condition.DBSyncReadyMessage)

	// run Glance db sync - end

	r.Log.Info(fmt.Sprintf("Reconciled Service '%s' init successfully", instance.Name))
	return ctrl.Result{}, nil
}

func (r *GlanceReconciler) reconcileUpdate(ctx context.Context, instance *glancev1.Glance, helper *helper.Helper) (ctrl.Result, error) {
	r.Log.Info(fmt.Sprintf("Reconciling Service '%s' update", instance.Name))

	// TODO: should have minor update tasks if required
	// - delete dbsync hash from status to rerun it?

	r.Log.Info(fmt.Sprintf("Reconciled Service '%s' update successfully", instance.Name))
	return ctrl.Result{}, nil
}

func (r *GlanceReconciler) reconcileUpgrade(ctx context.Context, instance *glancev1.Glance, helper *helper.Helper) (ctrl.Result, error) {
	r.Log.Info(fmt.Sprintf("Reconciling Service '%s' upgrade", instance.Name))

	// TODO: should have major version upgrade tasks
	// -delete dbsync hash from status to rerun it?

	r.Log.Info(fmt.Sprintf("Reconciled Service '%s' upgrade successfully", instance.Name))
	return ctrl.Result{}, nil
}

func (r *GlanceReconciler) reconcileNormal(ctx context.Context, instance *glancev1.Glance, helper *helper.Helper) (ctrl.Result, error) {
	r.Log.Info(fmt.Sprintf("Reconciling Service '%s'", instance.Name))

	// ConfigMap
	configMapVars := make(map[string]env.Setter)

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
			return ctrl.Result{RequeueAfter: time.Duration(10) * time.Second}, fmt.Errorf("OpenStack secret %s not found", instance.Spec.Secret)
		}
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.InputReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			condition.InputReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}
	configMapVars[ospSecret.Name] = env.SetValue(hash)
	instance.Status.Conditions.MarkTrue(condition.InputReadyCondition, condition.InputReadyMessage)
	// run check OpenStack secret - end

	//
	// Create ConfigMaps and Secrets required as input for the Service and calculate an overall hash of hashes
	//

	//
	// create Configmap required for glance input
	// - %-scripts configmap holding scripts to e.g. bootstrap the service
	// - %-config configmap holding minimal glance config required to get the service up, user can add additional files to be added to the service
	// - parameters which has passwords gets added from the OpenStack secret via the init container
	//
	err = r.generateServiceConfigMaps(ctx, helper, instance, &configMapVars)
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
	// Create ConfigMaps and Secrets - end

	//
	// TODO check when/if Init, Update, or Upgrade should/could be skipped
	//

	serviceLabels := map[string]string{
		common.AppSelector: glance.ServiceName,
	}

	// Handle service init
	ctrlResult, err := r.reconcileInit(ctx, instance, helper, serviceLabels)
	if err != nil {
		return ctrlResult, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	// Handle service update
	ctrlResult, err = r.reconcileUpdate(ctx, instance, helper)
	if err != nil {
		return ctrlResult, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	// Handle service upgrade
	ctrlResult, err = r.reconcileUpgrade(ctx, instance, helper)
	if err != nil {
		return ctrlResult, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	//
	// normal reconcile tasks
	//

	// deploy internal glance-api

	// Regardless of what the user may have set in GlanceAPIInternal.EndpointType,
	// we force "internal" here
	instance.Spec.GlanceAPIInternal.APIType = glancev1.APIInternal
	glanceAPI, op, err := r.apiDeploymentCreateOrUpdate(instance, instance.Spec.GlanceAPIInternal, helper)
	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			glancev1.GlanceAPIReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			glancev1.GlanceAPIReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}
	if op != controllerutil.OperationResultNone {
		r.Log.Info(fmt.Sprintf("Deployment %s successfully reconciled - operation: %s", instance.Name, string(op)))
	}

	// It is possible that an earlier call to update the status has also set
	// APIEndpoints to nil (if the APIEndpoints map was not nil but was empty,
	// saving the status unfortunately re-initializes it as nil)
	if instance.Status.APIEndpoints == nil {
		instance.Status.APIEndpoints = map[string]string{}
	}

	// Mirror internal GlanceAPI status' APIEndpoints and ReadyCount to this parent CR
	if glanceAPI.Status.APIEndpoints != nil {
		instance.Status.APIEndpoints = glanceAPI.Status.APIEndpoints
	}
	instance.Status.GlanceAPIInternalReadyCount = glanceAPI.Status.ReadyCount

	// Get internal GlanceAPI's condition status for comparison with external below
	internalAPICondition := glanceAPI.Status.Conditions.Mirror(glancev1.GlanceAPIReadyCondition)

	// deploy external glance-api

	// Regardless of what the user may have set in GlanceAPIExternal.EndpointType,
	// we force "external" here
	instance.Spec.GlanceAPIExternal.APIType = glancev1.APIExternal
	glanceAPI, op, err = r.apiDeploymentCreateOrUpdate(instance, instance.Spec.GlanceAPIExternal, helper)
	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			glancev1.GlanceAPIReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			glancev1.GlanceAPIReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}
	if op != controllerutil.OperationResultNone {
		r.Log.Info(fmt.Sprintf("Deployment %s successfully reconciled - operation: %s", instance.Name, string(op)))
	}

	// Mirror external GlanceAPI status' APIEndpoints and ReadyCount to this parent CR
	if glanceAPI.Status.APIEndpoints != nil {
		instance.Status.APIEndpoints[string(endpoint.EndpointPublic)] = glanceAPI.Status.APIEndpoints[string(endpoint.EndpointPublic)]
	}
	instance.Status.GlanceAPIExternalReadyCount = glanceAPI.Status.ReadyCount

	// Get external GlanceAPI's condition status and compare it against priority of internal GlanceAPI's condition
	externalAPICondition := glanceAPI.Status.Conditions.Mirror(glancev1.GlanceAPIReadyCondition)
	apiCondition := condition.GetHigherPrioCondition(internalAPICondition, externalAPICondition).DeepCopy()
	if apiCondition != nil {
		instance.Status.Conditions.Set(apiCondition)
	}

	r.Log.Info(fmt.Sprintf("Reconciled Service '%s' successfully", instance.Name))
	return ctrl.Result{}, nil
}

func (r *GlanceReconciler) apiDeploymentCreateOrUpdate(instance *glancev1.Glance, apiInstance glancev1.GlanceAPISpec, helper *helper.Helper) (*glancev1.GlanceAPI, controllerutil.OperationResult, error) {
	deployment := &glancev1.GlanceAPI{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", instance.Name, apiInstance.APIType),
			Namespace: instance.Namespace,
		},
	}

	op, err := controllerutil.CreateOrUpdate(context.TODO(), r.Client, deployment, func() error {
		deployment.Spec = apiInstance
		// Add in transfers from umbrella Glance (this instance) spec
		// TODO: Add logic to determine when to set/overwrite, etc
		deployment.Spec.ContainerImage = instance.Spec.ContainerImage
		deployment.Spec.CustomServiceConfig = instance.Spec.CustomServiceConfig
		deployment.Spec.DatabaseHostname = instance.Status.DatabaseHostname
		deployment.Spec.DatabaseUser = instance.Spec.DatabaseUser
		deployment.Spec.DefaultConfigOverwrite = instance.Spec.DefaultConfigOverwrite
		deployment.Spec.NodeSelector = instance.Spec.NodeSelector
		// TODO: This could be superfluous if backed by Ceph?
		deployment.Spec.Pvc = glance.ServiceName
		deployment.Spec.Secret = instance.Spec.Secret
		deployment.Spec.ExtraMounts = instance.Spec.ExtraMounts

		err := controllerutil.SetControllerReference(instance, deployment, r.Scheme)
		if err != nil {
			return err
		}

		// Add a finalizer to prevent user from manually removing child GlanceAPI
		controllerutil.AddFinalizer(deployment, helper.GetFinalizer())

		return nil
	})

	return deployment, op, err
}

// generateServiceConfigMaps - create create configmaps which hold scripts and service configuration (*used for DBSync only*)
// TODO add DefaultConfigOverwrite
func (r *GlanceReconciler) generateServiceConfigMaps(
	ctx context.Context,
	h *helper.Helper,
	instance *glancev1.Glance,
	envVars *map[string]env.Setter,
) error {
	// TODO: Since these CMs are only used for the DBSync, is there further pruning that
	//       could be done here?

	//
	// create Configmap/Secret required for glance input
	// - %-scripts configmap holding scripts to e.g. bootstrap the service
	// - %-config configmap holding minimal glance config required to get the service up, user can add additional files to be added to the service
	// - parameters which has passwords gets added from the ospSecret via the init container
	//

	cmLabels := labels.GetLabels(instance, labels.GetGroupLabel(glance.ServiceName), map[string]string{})

	// customData hold any customization for the service.
	// custom.conf is going to /etc/<service>/<service>.conf.d
	// all other files get placed into /etc/<service> to allow overwrite of e.g. logging.conf or policy.json
	// TODO: make sure custom.conf can not be overwritten
	customData := map[string]string{common.CustomServiceConfigFileName: instance.Spec.CustomServiceConfig}
	for key, data := range instance.Spec.DefaultConfigOverwrite {
		customData[key] = data
	}

	cms := []util.Template{
		// ScriptsConfigMap
		{
			Name:               fmt.Sprintf("%s-scripts", instance.Name),
			Namespace:          instance.Namespace,
			Type:               util.TemplateTypeScripts,
			InstanceType:       instance.Kind,
			AdditionalTemplate: map[string]string{"common.sh": "/common/common.sh"},
			Labels:             cmLabels,
		},
		// ConfigMap
		{
			Name:         fmt.Sprintf("%s-config-data", instance.Name),
			Namespace:    instance.Namespace,
			Type:         util.TemplateTypeConfig,
			InstanceType: instance.Kind,
			CustomData:   customData,
			Labels:       cmLabels,
		},
	}
	err := configmap.EnsureConfigMaps(ctx, h, instance, cms, envVars)

	if err != nil {
		return nil
	}

	return nil
}
