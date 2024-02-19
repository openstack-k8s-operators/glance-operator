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

	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"

	"time"

	glancev1 "github.com/openstack-k8s-operators/glance-operator/api/v1beta1"
	"github.com/openstack-k8s-operators/glance-operator/pkg/glance"
	"github.com/openstack-k8s-operators/lib-common/modules/common/env"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	nad "github.com/openstack-k8s-operators/lib-common/modules/common/networkattachment"
	"github.com/openstack-k8s-operators/lib-common/modules/common/secret"
	"github.com/openstack-k8s-operators/lib-common/modules/common/service"
	"github.com/openstack-k8s-operators/lib-common/modules/common/util"
	corev1 "k8s.io/api/core/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// fields to index to reconcile when change
const (
	passwordSecretField     = ".spec.secret"
	caBundleSecretNameField = ".spec.tls.caBundleSecretName"
	tlsAPIInternalField     = ".spec.tls.api.internal.secretName"
	tlsAPIPublicField       = ".spec.tls.api.public.secretName"
)

var (
	glanceWatchFields = []string{
		passwordSecretField,
	}
	glanceAPIWatchFields = []string{
		passwordSecretField,
		caBundleSecretNameField,
		tlsAPIInternalField,
		tlsAPIPublicField,
	}
)

type conditionUpdater interface {
	Set(c *condition.Condition)
	MarkTrue(t condition.Type, messageFormat string, messageArgs ...interface{})
}

// ensureNAD - common function called in the glance controllers that GetNAD based
// on the string[] passed as input and produces the required Annotation for the
// glanceAPI component
func ensureNAD(
	ctx context.Context,
	conditionUpdater conditionUpdater,
	nAttach []string,
	helper *helper.Helper,
) (map[string]string, ctrl.Result, error) {

	var serviceAnnotations map[string]string
	var err error
	// Iterate over the []networkattachment, get the corresponding NAD and create
	// the required annotation
	for _, netAtt := range nAttach {
		_, err = nad.GetNADWithName(ctx, helper, netAtt, helper.GetBeforeObject().GetNamespace())
		if err != nil {
			if k8s_errors.IsNotFound(err) {
				conditionUpdater.Set(condition.FalseCondition(
					condition.NetworkAttachmentsReadyCondition,
					condition.RequestedReason,
					condition.SeverityInfo,
					condition.NetworkAttachmentsReadyWaitingMessage,
					netAtt))
				return serviceAnnotations, ctrl.Result{RequeueAfter: time.Second * 10}, fmt.Errorf("network-attachment-definition %s not found", netAtt)
			}
			conditionUpdater.Set(condition.FalseCondition(
				condition.NetworkAttachmentsReadyCondition,
				condition.ErrorReason,
				condition.SeverityWarning,
				condition.NetworkAttachmentsReadyErrorMessage,
				err.Error()))
			return serviceAnnotations, ctrl.Result{}, err
		}
	}
	// Create NetworkAnnotations
	serviceAnnotations, err = nad.CreateNetworksAnnotation(helper.GetBeforeObject().GetNamespace(), nAttach)
	if err != nil {
		return serviceAnnotations, ctrl.Result{}, fmt.Errorf("failed create network annotation from %s: %w",
			nAttach, err)
	}
	return serviceAnnotations, ctrl.Result{}, err
}

// GenerateConfigsGeneric -
func GenerateConfigsGeneric(
	ctx context.Context, h *helper.Helper,
	instance client.Object,
	envVars *map[string]env.Setter,
	templateParameters map[string]interface{},
	customData map[string]string,
	cmLabels map[string]string,
	scripts bool,
) error {

	cms := []util.Template{
		// Templates where the GlanceAPI config is stored
		{
			Name:          fmt.Sprintf("%s-config-data", instance.GetName()),
			Namespace:     instance.GetNamespace(),
			Type:          util.TemplateTypeConfig,
			InstanceType:  instance.GetObjectKind().GroupVersionKind().Kind,
			ConfigOptions: templateParameters,
			CustomData:    customData,
			Labels:        cmLabels,
		},
	}
	// TODO: Scripts have no reason to be secrets, should move to configmap
	if scripts {
		cms = append(cms, util.Template{
			Name:         fmt.Sprintf("%s-scripts", instance.GetName()),
			Namespace:    instance.GetNamespace(),
			Type:         util.TemplateTypeScripts,
			InstanceType: instance.GetObjectKind().GroupVersionKind().Kind,
			Labels:       cmLabels,
		})
	}
	return secret.EnsureSecrets(ctx, h, instance, cms, envVars)
}

// GetHeadlessService -
func GetHeadlessService(
	ctx context.Context,
	helper *helper.Helper,
	instance *glancev1.GlanceAPI,
	serviceLabels map[string]string,
) (ctrl.Result, error) {

	endpointName := instance.Name
	// The endpointName for headless services **must** match with:
	// - statefulset.metadata.name
	// - statefulset.spec.servicename
	if instance.Spec.APIType != glancev1.APISingle {
		endpointName = fmt.Sprintf("%s-api", instance.Name)
	}

	// Create the (headless) service
	svc, err := service.NewService(
		service.GenericService(&service.GenericServiceDetails{
			Name:      endpointName,
			Namespace: instance.Namespace,
			Labels:    serviceLabels,
			Selector:  serviceLabels,
			Port: service.GenericServicePort{
				Name:     endpointName,
				Port:     glance.GlanceInternalPort,
				Protocol: corev1.ProtocolTCP,
			},
			ClusterIP: "None",
		}),
		5,
		nil,
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
		service.AnnotationEndpointKey: "headless",
	})
	svc.AddAnnotation(map[string]string{
		service.AnnotationIngressCreateKey: "false",
	})

	// register the service hostname as base domain to build the worker_self_reference_url
	// that will be used for distributed image import across multiple replicas
	instance.Status.Domain = svc.GetServiceHostname()

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

	return ctrlResult, nil
}
