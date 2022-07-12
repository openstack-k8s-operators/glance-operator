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

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/go-logr/logr"
	routev1 "github.com/openshift/api/route/v1"
	glancev1beta1 "github.com/openstack-k8s-operators/glance-operator/api/v1beta1"
	"github.com/openstack-k8s-operators/glance-operator/pkg/glance"
	keystonev1beta1 "github.com/openstack-k8s-operators/keystone-operator/api/v1beta1"
	keystone "github.com/openstack-k8s-operators/keystone-operator/pkg/external"
	"github.com/openstack-k8s-operators/lib-common/pkg/common"
	"github.com/openstack-k8s-operators/lib-common/pkg/condition"
	database "github.com/openstack-k8s-operators/lib-common/pkg/database"
	"github.com/openstack-k8s-operators/lib-common/pkg/helper"
	mariadbv1beta1 "github.com/openstack-k8s-operators/mariadb-operator/api/v1beta1"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetClient -
func (r *GlanceAPIReconciler) GetClient() client.Client {
	return r.Client
}

// GetKClient -
func (r *GlanceAPIReconciler) GetKClient() kubernetes.Interface {
	return r.Kclient
}

// GetLogger -
func (r *GlanceAPIReconciler) GetLogger() logr.Logger {
	return r.Log
}

// GetScheme -
func (r *GlanceAPIReconciler) GetScheme() *runtime.Scheme {
	return r.Scheme
}

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
// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;create;update;delete;watch
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete;
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete;
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete;
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete;
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete;
// +kubebuilder:rbac:groups=route.openshift.io,resources=routes,verbs=get;list;watch;create;update;patch;delete;
// +kubebuilder:rbac:groups=mariadb.openstack.org,resources=mariadbdatabases,verbs=get;list;watch;create;update;patch;delete;
// +kubebuilder:rbac:groups=keystone.openstack.org,resources=keystoneapis,verbs=get;list;watch;
// +kubebuilder:rbac:groups=keystone.openstack.org,resources=keystoneservices,verbs=get;list;watch;create;update;patch;delete;

// Reconcile reconcile Glance API requests
func (r *GlanceAPIReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	// Fetch the GlanceAPI instance
	instance := &glancev1beta1.GlanceAPI{}
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

	//
	// initialize status
	//
	if instance.Status.Conditions == nil {
		instance.Status.Conditions = condition.List{}
	}
	if instance.Status.Hash == nil {
		instance.Status.Hash = map[string]string{}
	}
	if instance.Status.APIEndpoints == nil {
		instance.Status.APIEndpoints = map[string]string{}
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
		if err := helper.SetAfter(instance); err != nil {
			common.LogErrorForObject(r, err, "Set after and calc patch/diff", instance)
		}

		if changed := helper.GetChanges()["status"]; changed {
			patch := client.MergeFrom(helper.GetBeforeObject())

			if err := r.Status().Patch(ctx, instance, patch); err != nil && !k8s_errors.IsNotFound(err) {
				common.LogErrorForObject(r, err, "Update status", instance)
			}
		}
	}()

	// Handle service delete
	if !instance.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, instance, helper)
	}

	// Handle non-deleted clusters
	return r.reconcileNormal(ctx, instance, helper)
}

// SetupWithManager sets up the controller with the Manager.
func (r *GlanceAPIReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&glancev1beta1.GlanceAPI{}).
		Owns(&mariadbv1beta1.MariaDBDatabase{}).
		Owns(&keystonev1beta1.KeystoneService{}).
		Owns(&batchv1.Job{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.Secret{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&appsv1.Deployment{}).
		Owns(&routev1.Route{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Complete(r)
}

func (r *GlanceAPIReconciler) reconcileDelete(ctx context.Context, instance *glancev1beta1.GlanceAPI, helper *helper.Helper) (ctrl.Result, error) {
	r.Log.Info("Reconciling Service delete")

	// Service is deleted so remove the finalizer.
	controllerutil.RemoveFinalizer(instance, helper.GetFinalizer())
	r.Log.Info("Reconciled Service delete successfully")
	if err := r.Update(ctx, instance); err != nil && !k8s_errors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *GlanceAPIReconciler) reconcileInit(
	ctx context.Context,
	instance *glancev1beta1.GlanceAPI,
	helper *helper.Helper,
	serviceLabels map[string]string,
) (ctrl.Result, error) {
	r.Log.Info("Reconciling Service init")

	// Define a new PVC object
	// TODO: Once conditions added to PVC lib-common logic, handle
	//       the returned condition here
	pvc := common.NewPvc(
		glance.Pvc(instance, serviceLabels),
		5,
	)

	ctrlResult, err := pvc.CreateOrPatch(ctx, helper)

	if err != nil {
		return ctrlResult, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	if pvc.GetPvc().Status.Phase != corev1.ClaimBound {
		r.Log.Info("Waiting for GlanceAPI PVC to bind")
		return ctrl.Result{RequeueAfter: time.Second * 5}, nil
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
	cond, ctrlResult, err := db.CreateOrPatchDB(
		ctx,
		helper,
	)
	instance.Status.Conditions.UpdateCurrentCondition(cond)

	if err != nil {
		return ctrl.Result{}, err
	}
	if (ctrlResult != ctrl.Result{}) {
		r.Log.Info(cond.Message)
		return ctrlResult, nil
	}
	// wait for the DB to be setup
	cond, ctrlResult, err = db.WaitForDBCreated(ctx, helper)
	instance.Status.Conditions.UpdateCurrentCondition(cond)
	if err != nil {
		return ctrlResult, err
	}
	if (ctrlResult != ctrl.Result{}) {
		r.Log.Info(cond.Message)
		return ctrlResult, nil
	}
	// update Status.DatabaseHostname, used to config the service
	instance.Status.DatabaseHostname = db.GetDatabaseHostname()
	// create service DB - end

	//
	// expose the service (create service, route and return the created endpoint URLs)
	//
	var ports = map[common.Endpoint]int32{
		common.EndpointAdmin:    glance.GlanceAdminPort,
		common.EndpointPublic:   glance.GlancePublicPort,
		common.EndpointInternal: glance.GlanceInternalPort,
	}

	apiEndpoints, ctrlResult, err := common.ExposeEndpoints(
		ctx,
		helper,
		glance.ServiceName,
		serviceLabels,
		ports,
	)
	if err != nil {
		return ctrlResult, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	//
	// Update instance status with service endpoint url from route host information
	//
	// TODO: need to support https default here
	if instance.Status.APIEndpoints == nil {
		instance.Status.APIEndpoints = map[string]string{}
	}
	instance.Status.APIEndpoints = apiEndpoints

	// expose service - end

	//
	// create users and endpoints - https://docs.openstack.org/Glance/latest/install/install-rdo.html#configure-user-and-endpoints
	// TODO: rework this
	//
	ospSecret, _, err := common.GetSecret(ctx, helper, instance.Spec.Secret, instance.Namespace)
	if err != nil {
		if k8s_errors.IsNotFound(err) {
			return ctrl.Result{RequeueAfter: time.Second * 10}, fmt.Errorf("OpenStack secret %s not found", instance.Spec.Secret)
		}
		return ctrl.Result{}, err
	}

	GlanceKeystoneService := &keystonev1beta1.KeystoneService{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name,
			Namespace: instance.Namespace,
		},
	}

	_, err = controllerutil.CreateOrPatch(context.TODO(), r.Client, GlanceKeystoneService, func() error {
		GlanceKeystoneService.Spec.Username = instance.Spec.ServiceUser
		GlanceKeystoneService.Spec.Password = string(ospSecret.Data[instance.Spec.PasswordSelectors.Service])
		GlanceKeystoneService.Spec.ServiceType = glance.ServiceType
		GlanceKeystoneService.Spec.ServiceName = glance.ServiceName
		GlanceKeystoneService.Spec.ServiceDescription = glance.ServiceName
		GlanceKeystoneService.Spec.Enabled = true
		// TODO: get from keystone object
		GlanceKeystoneService.Spec.Region = "regionOne"
		GlanceKeystoneService.Spec.AdminURL = apiEndpoints["admin"]
		GlanceKeystoneService.Spec.PublicURL = apiEndpoints["public"]
		GlanceKeystoneService.Spec.InternalURL = apiEndpoints["internal"]

		return nil
	})

	if err != nil {
		return ctrl.Result{}, err
	}

	//
	// run Glance db sync
	//
	dbSyncHash := instance.Status.Hash[glancev1beta1.DbSyncHash]
	jobDef := glance.DbSyncJob(instance, serviceLabels)
	dbSyncjob := common.NewJob(
		jobDef,
		glancev1beta1.DbSyncHash,
		instance.Spec.PreserveJobs,
		5,
		dbSyncHash,
	)
	ctrlResult, err = dbSyncjob.DoJob(
		ctx,
		helper,
	)
	if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}
	if err != nil {
		return ctrl.Result{}, err
	}
	if dbSyncjob.HasChanged() {
		instance.Status.Hash[glancev1beta1.DbSyncHash] = dbSyncjob.GetHash()
		if err := r.Client.Status().Update(ctx, instance); err != nil {
			return ctrl.Result{}, err
		}
		r.Log.Info(fmt.Sprintf("Job %s hash added - %s", jobDef.Name, instance.Status.Hash[glancev1beta1.DbSyncHash]))
	}

	// run Glance db sync - end

	r.Log.Info("Reconciled Service init successfully")
	return ctrl.Result{}, nil
}

func (r *GlanceAPIReconciler) reconcileUpdate(ctx context.Context, instance *glancev1beta1.GlanceAPI, helper *helper.Helper) (ctrl.Result, error) {
	r.Log.Info("Reconciling Service update")

	// TODO: should have minor update tasks if required
	// - delete dbsync hash from status to rerun it?

	r.Log.Info("Reconciled Service update successfully")
	return ctrl.Result{}, nil
}

func (r *GlanceAPIReconciler) reconcileUpgrade(ctx context.Context, instance *glancev1beta1.GlanceAPI, helper *helper.Helper) (ctrl.Result, error) {
	r.Log.Info("Reconciling Service upgrade")

	// TODO: should have major version upgrade tasks
	// -delete dbsync hash from status to rerun it?

	r.Log.Info("Reconciled Service upgrade successfully")
	return ctrl.Result{}, nil
}

func (r *GlanceAPIReconciler) reconcileNormal(ctx context.Context, instance *glancev1beta1.GlanceAPI, helper *helper.Helper) (ctrl.Result, error) {
	r.Log.Info("Reconciling Service")

	// If the service object doesn't have our finalizer, add it.
	controllerutil.AddFinalizer(instance, helper.GetFinalizer())
	// Register the finalizer immediately to avoid orphaning resources on delete
	//if err := patchHelper.Patch(ctx, openStackCluster); err != nil {
	if err := r.Update(ctx, instance); err != nil {
		return ctrl.Result{}, err
	}

	// ConfigMap
	configMapVars := make(map[string]common.EnvSetter)

	//
	// check for required OpenStack secret holding passwords for service/admin user and add hash to the vars map
	//
	ospSecret, hash, err := common.GetSecret(ctx, helper, instance.Spec.Secret, instance.Namespace)
	if err != nil {
		if k8s_errors.IsNotFound(err) {
			return ctrl.Result{RequeueAfter: time.Second * 10}, fmt.Errorf("OpenStack secret %s not found", instance.Spec.Secret)
		}
		return ctrl.Result{}, err
	}
	configMapVars[ospSecret.Name] = common.EnvValue(hash)
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
		return ctrl.Result{}, err
	}

	//
	// create hash over all the different input resources to identify if any those changed
	// and a restart/recreate is required.
	//
	inputHash, err := r.createHashOfInputHashes(ctx, instance, configMapVars)
	if err != nil {
		return ctrl.Result{}, err
	}
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

	// Define a new Deployment object
	depl := common.NewDeployment(
		glance.Deployment(instance, inputHash, serviceLabels),
		5,
	)

	ctrlResult, err = depl.CreateOrPatch(ctx, helper)
	if err != nil {
		return ctrlResult, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}
	// create Deployment - end

	r.Log.Info("Reconciled Service successfully")
	return ctrl.Result{}, nil
}

//
// generateServiceConfigMaps - create create configmaps which hold scripts and service configuration
// TODO add DefaultConfigOverwrite
//
func (r *GlanceAPIReconciler) generateServiceConfigMaps(
	ctx context.Context,
	h *helper.Helper,
	instance *glancev1beta1.GlanceAPI,
	envVars *map[string]common.EnvSetter,
) error {
	//
	// create Configmap/Secret required for glance input
	// - %-scripts configmap holding scripts to e.g. bootstrap the service
	// - %-config configmap holding minimal glance config required to get the service up, user can add additional files to be added to the service
	// - parameters which has passwords gets added from the ospSecret via the init container
	//

	cmLabels := common.GetLabels(instance, common.GetGroupLabel(glance.ServiceName), map[string]string{})

	// customData hold any customization for the service.
	// custom.conf is going to /etc/<service>/<service>.conf.d
	// all other files get placed into /etc/<service> to allow overwrite of e.g. logging.conf or policy.json
	// TODO: make sure custom.conf can not be overwritten
	customData := map[string]string{common.CustomServiceConfigFileName: instance.Spec.CustomServiceConfig}
	for key, data := range instance.Spec.DefaultConfigOverwrite {
		customData[key] = data
	}

	keystoneAPI, err := keystone.GetKeystoneAPI(ctx, h, instance.Namespace, map[string]string{})
	if err != nil {
		return err
	}
	authURL, err := keystoneAPI.GetEndpoint(common.EndpointPublic)
	if err != nil {
		return err
	}
	templateParameters := make(map[string]interface{})
	templateParameters["ServiceUser"] = instance.Spec.ServiceUser
	templateParameters["KeystonePublicURL"] = authURL

	cms := []common.Template{
		// ScriptsConfigMap
		{
			Name:               fmt.Sprintf("%s-scripts", instance.Name),
			Namespace:          instance.Namespace,
			Type:               common.TemplateTypeScripts,
			InstanceType:       instance.Kind,
			AdditionalTemplate: map[string]string{"common.sh": "/common/common.sh"},
			Labels:             cmLabels,
		},
		// ConfigMap
		{
			Name:          fmt.Sprintf("%s-config-data", instance.Name),
			Namespace:     instance.Namespace,
			Type:          common.TemplateTypeConfig,
			InstanceType:  instance.Kind,
			CustomData:    customData,
			ConfigOptions: templateParameters,
			Labels:        cmLabels,
		},
	}
	err = common.EnsureConfigMaps(ctx, r, instance, cms, envVars)
	if err != nil {
		return nil
	}

	return nil
}

//
// createHashOfInputHashes - creates a hash of hashes which gets added to the resources which requires a restart
// if any of the input resources change, like configs, passwords, ...
//
func (r *GlanceAPIReconciler) createHashOfInputHashes(
	ctx context.Context,
	instance *glancev1beta1.GlanceAPI,
	envVars map[string]common.EnvSetter,
) (string, error) {
	mergedMapVars := common.MergeEnvs([]corev1.EnvVar{}, envVars)
	hash, err := common.ObjectHash(mergedMapVars)
	if err != nil {
		return hash, err
	}
	if hashMap, changed := common.SetHash(instance.Status.Hash, common.InputHashName, hash); changed {
		instance.Status.Hash = hashMap
		if err := r.Client.Status().Update(ctx, instance); err != nil {
			return hash, err
		}
		r.Log.Info(fmt.Sprintf("Input maps hash %s - %s", common.InputHashName, hash))
	}
	return hash, nil
}
