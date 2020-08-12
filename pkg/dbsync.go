package glance

import (
	glancev1beta1 "github.com/openstack-k8s-operators/glance-operator/api/v1beta1"
	util "github.com/openstack-k8s-operators/lib-common/pkg/util"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// DbSyncJob func
func DbSyncJob(cr *glancev1beta1.GlanceAPI, scheme *runtime.Scheme) *batchv1.Job {

	runAsUser := int64(0)

	labels := map[string]string{
		"app": "glance-api",
	}
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name + "-db-sync",
			Namespace: cr.Namespace,
			Labels:    labels,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy:      "OnFailure",
					ServiceAccountName: "glance",
					Containers: []corev1.Container{
						{
							Name:  "glance-db-sync",
							Image: cr.Spec.ContainerImage,
							//Command: []string{"/bin/sleep", "7000"},
							SecurityContext: &corev1.SecurityContext{
								RunAsUser: &runAsUser,
							},
							Env: []corev1.EnvVar{
								{
									Name:  "KOLLA_CONFIG_STRATEGY",
									Value: "COPY_ALWAYS",
								},
								{
									Name:  "KOLLA_BOOTSTRAP",
									Value: "TRUE",
								},
							},
							VolumeMounts: getDbVolumeMounts(),
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
	job.Spec.Template.Spec.Volumes = getVolumes(cr.Name, cr.Spec.Secrets)
	controllerutil.SetControllerReference(cr, job, scheme)
	return job
}
