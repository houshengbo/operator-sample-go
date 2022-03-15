package applicationcontroller

import (
	"context"

	applicationsamplev1alpha1 "github.com/nheidloff/operator-sample-go/operator-application/api/v1alpha1"
	"github.com/nheidloff/operator-sample-go/operator-application/utilities"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func (reconciler *ApplicationReconciler) defineDeployment(application *applicationsamplev1alpha1.Application) *appsv1.Deployment {
	replicas := application.Spec.AmountPods
	labels := map[string]string{labelKey: labelValue}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName,
			Namespace: application.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Image: image,
						Name:  containerName,
						Ports: []corev1.ContainerPort{{
							ContainerPort: port,
						}},
						Env: []corev1.EnvVar{{
							Name: secretGreetingMessageLabel,
							ValueFrom: &v1.EnvVarSource{
								SecretKeyRef: &v1.SecretKeySelector{
									LocalObjectReference: v1.LocalObjectReference{
										Name: secretName,
									},
									Key: secretGreetingMessageLabel,
								},
							}},
						},
						ReadinessProbe: &v1.Probe{
							ProbeHandler: v1.ProbeHandler{
								HTTPGet: &v1.HTTPGetAction{Path: "/q/health/live", Port: intstr.IntOrString{
									IntVal: port,
								}},
							},
							InitialDelaySeconds: 20,
						},
						LivenessProbe: &v1.Probe{
							ProbeHandler: v1.ProbeHandler{
								HTTPGet: &v1.HTTPGetAction{Path: "/q/health/ready", Port: intstr.IntOrString{
									IntVal: port,
								}},
							},
							InitialDelaySeconds: 40,
						},
					}},
				},
			},
		},
	}

	specHashActual := utilities.GetHashForSpec(&deployment.Spec)
	deployment.Labels = utilities.SetHashToLabels(nil, specHashActual)

	ctrl.SetControllerReference(application, deployment, reconciler.Scheme)
	return deployment
}

func (reconciler *ApplicationReconciler) reconcileDeployment(ctx context.Context, application *applicationsamplev1alpha1.Application) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	deployment := &appsv1.Deployment{}
	deploymentDefinition := reconciler.defineDeployment(application)
	err := reconciler.Get(ctx, types.NamespacedName{Name: deploymentName, Namespace: application.Namespace}, deployment)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("Deployment resource " + deploymentName + " not found. Creating or re-creating deployment")
			err = reconciler.Create(ctx, deploymentDefinition)
			if err != nil {
				log.Info("Failed to create deployment resource. Re-running reconcile.")
				return ctrl.Result{}, err
			}
		} else {
			log.Info("Failed to get deployment resource " + deploymentName + ". Re-running reconcile.")
			return ctrl.Result{}, err
		}
	} else {
		specHashTarget := utilities.GetHashForSpec(&deploymentDefinition.Spec)
		specHashActual := utilities.GetHashFromLabels(deployment.Labels)
		// Note: When using the hash, the controller will not revert manual changes of the amount of replicas in the deployment
		if specHashActual != specHashTarget {
			var current int32 = *deployment.Spec.Replicas
			var expected int32 = *deploymentDefinition.Spec.Replicas
			if current != expected {
				deployment.Spec.Replicas = &expected
				deployment.Labels = utilities.SetHashToLabels(deployment.Labels, specHashTarget)
				err = reconciler.Update(ctx, deployment)
				if err != nil {
					log.Info("Failed to update deployment resource. Re-running reconcile.")
					return ctrl.Result{}, err
				}
			}
		}
	}
	return ctrl.Result{}, nil
}
