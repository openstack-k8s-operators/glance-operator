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

// Package controllers implements the glance-operator Kubernetes controllers.
package controllers

import (
	"context"
	"errors"
	"fmt"
	"time"

	networkv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"k8s.io/apimachinery/pkg/types"

	glancev1 "github.com/openstack-k8s-operators/glance-operator/api/v1beta1"
	"github.com/openstack-k8s-operators/glance-operator/pkg/glance"
	"github.com/openstack-k8s-operators/lib-common/modules/common"
	"github.com/openstack-k8s-operators/lib-common/modules/common/env"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	nad "github.com/openstack-k8s-operators/lib-common/modules/common/networkattachment"
	"github.com/openstack-k8s-operators/lib-common/modules/common/secret"
	"github.com/openstack-k8s-operators/lib-common/modules/common/service"
	"github.com/openstack-k8s-operators/lib-common/modules/common/util"
	corev1 "k8s.io/api/core/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// Common static errors for glance controllers
var (
	ErrNetworkAttachmentConfig = errors.New("not all pods have interfaces with ips as configured in NetworkAttachments")
)

// fields to index to reconcile when change
const (
	passwordSecretField        = ".spec.secret"
	caBundleSecretNameField    = ".spec.tls.caBundleSecretName" // #nosec G101
	tlsAPIInternalField        = ".spec.tls.api.internal.secretName"
	tlsAPIPublicField          = ".spec.tls.api.public.secretName"
	topologyField              = ".spec.topologyRef.Name"
	notificationBusSecretField = ".spec.notificationBusSecret"
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
		topologyField,
		notificationBusSecretField,
	}
)

type conditionUpdater interface {
	Set(c *condition.Condition)
	MarkTrue(t condition.Type, messageFormat string, messageArgs ...any)
}

// verifyServiceSecret - ensures that the Secret object exists and the expected
// fields are in the Secret. It also sets a hash of the values of the expected
// fields passed as input.
func verifyServiceSecret(
	ctx context.Context,
	secretName types.NamespacedName,
	expectedFields []string,
	reader client.Reader,
	conditionUpdater conditionUpdater,
	requeueTimeout time.Duration,
	envVars *map[string]env.Setter,
) (ctrl.Result, error) {

	hash, res, err := secret.VerifySecret(ctx, secretName, expectedFields, reader, requeueTimeout)
	if err != nil {
		conditionUpdater.Set(condition.FalseCondition(
			condition.InputReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			condition.InputReadyErrorMessage,
			err.Error()))
		return res, err
	} else if (res != ctrl.Result{}) {
		// Since the service secret should have been manually created by the user and referenced in the spec,
		// we treat this as a warning because it means that the service will not be able to start.
		log.FromContext(ctx).Info(fmt.Sprintf("OpenStack secret %s not found", secretName))
		conditionUpdater.Set(condition.FalseCondition(
			condition.InputReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			condition.InputReadyWaitingMessage))
		return res, nil
	}
	(*envVars)[secretName.Name] = env.SetValue(hash)
	return ctrl.Result{}, nil
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
	nadList := []networkv1.NetworkAttachmentDefinition{}
	for _, netAtt := range nAttach {
		nad, err := nad.GetNADWithName(ctx, helper, netAtt, helper.GetBeforeObject().GetNamespace())
		if err != nil {
			if k8s_errors.IsNotFound(err) {
				// Since the net-attach-def CR should have been manually created by the user and referenced in the spec,
				// we treat this as a warning because it means that the service will not be able to start.
				helper.GetLogger().Info(fmt.Sprintf("network-attachment-definition %s not found", netAtt))
				conditionUpdater.Set(condition.FalseCondition(
					condition.NetworkAttachmentsReadyCondition,
					condition.ErrorReason,
					condition.SeverityWarning,
					condition.NetworkAttachmentsReadyWaitingMessage,
					netAtt))
				return serviceAnnotations, glance.ResultRequeue, nil
			}
			conditionUpdater.Set(condition.FalseCondition(
				condition.NetworkAttachmentsReadyCondition,
				condition.ErrorReason,
				condition.SeverityWarning,
				condition.NetworkAttachmentsReadyErrorMessage,
				err.Error()))
			return serviceAnnotations, ctrl.Result{}, err
		}

		if nad != nil {
			nadList = append(nadList, *nad)
		}
	}
	// Create NetworkAnnotations
	serviceAnnotations, err = nad.EnsureNetworksAnnotation(nadList)
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
	templateParameters map[string]any,
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
			Name:         glance.ServiceName + "-scripts",
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
) (ctrl.Result, string, error) {

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
			condition.CreateServiceReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			condition.CreateServiceReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, endpointName, err
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
			condition.CreateServiceReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			condition.CreateServiceReadyErrorMessage,
			err.Error()))

		return ctrlResult, endpointName, err
	} else if (ctrlResult != ctrl.Result{}) {
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.CreateServiceReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			condition.CreateServiceReadyRunningMessage))
		return ctrlResult, endpointName, nil
	}

	return ctrlResult, endpointName, nil
}

// GetPvcListWithLabel -
func GetPvcListWithLabel(
	ctx context.Context,
	h *helper.Helper,
	namespace string,
	labelSelectorMap map[string]string,
) (*corev1.PersistentVolumeClaimList, error) {

	labelSelectorString := labels.Set(labelSelectorMap).String()
	pvcList, err := h.GetKClient().CoreV1().PersistentVolumeClaims(namespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelectorString})

	if err != nil {
		err = fmt.Errorf("error listing PVC for labels: %v - %w", labelSelectorMap, err)
		return nil, err
	}
	return pvcList, nil
}

// GetServiceLabels -
func GetServiceLabels(
	instance *glancev1.GlanceAPI,
) map[string]string {

	// Generate serviceLabels that will be passed to all the Service related resource
	// By doing this we can `oc get` all the resources associated to Glance making
	// queries by label
	return map[string]string{
		common.AppSelector:       glance.ServiceName,
		common.ComponentSelector: glance.Component,
		glance.GlanceAPIName:     fmt.Sprintf("%s-%s-%s", glance.ServiceName, instance.APIName(), instance.Spec.APIType),
		common.OwnerSelector:     instance.Name,
	}
}
