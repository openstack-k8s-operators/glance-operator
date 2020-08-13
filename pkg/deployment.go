package glance

import (
	glancev1beta1 "github.com/openstack-k8s-operators/glance-operator/api/v1beta1"
	util "github.com/openstack-k8s-operators/lib-common/pkg/util"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// Deployment func
func Deployment(cr *glancev1beta1.GlanceAPI, configHash string, scheme *runtime.Scheme) *appsv1.Deployment {
	runAsUser := int64(0)

	labels := map[string]string{
		"app": "glance-api",
	}
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name,
			Namespace: cr.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Replicas: &cr.Spec.Replicas,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: "glance",
					Containers: []corev1.Container{
						{
							Name: "glance-api",
							//Command: []string{"/bin/sleep", "7000"},
							Image: cr.Spec.ContainerImage,
							SecurityContext: &corev1.SecurityContext{
								RunAsUser: &runAsUser,
							},
							Env: []corev1.EnvVar{
								{
									Name:  "KOLLA_CONFIG_STRATEGY",
									Value: "COPY_ALWAYS",
								},
								{
									Name:  "CONFIG_HASH",
									Value: configHash,
								},
							},
							VolumeMounts: getVolumeMounts(),
						},
					},
					InitContainers: []corev1.Container{
						{
							Name:    "glance-secrets",
							Image:   cr.Spec.ContainerImage,
							Command: []string{"/bin/sh", "-c", util.ExecuteTemplateFile("password_init.sh", nil)},
							Env: []corev1.EnvVar{
								{
									Name:  "DatabaseHost",
									Value: cr.Spec.DatabaseHostname,
								},
								// FIXME: wire in transport host (AMQP)
								{
									Name:  "TransportHost",
									Value: "foo.bar",
								},
								{
									Name:  "DatabaseUser",
									Value: cr.Name,
								},
								{
									Name:  "DatabaseSchema",
									Value: cr.Name,
								},
							},
							VolumeMounts: getInitVolumeMounts(),
						},
					},
				},
			},
		},
	}
	deployment.Spec.Template.Spec.Volumes = getVolumes(cr.Name, cr.Spec.Secret)
	controllerutil.SetControllerReference(cr, deployment, scheme)
	return deployment
}
