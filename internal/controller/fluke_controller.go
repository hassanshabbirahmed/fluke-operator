/*
Copyright 2026.

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

package controller

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	flukev1alpha1 "github.com/hassanshabbirahmed/fluke-operator/api/v1alpha1"
)

// FlukeReconciler reconciles a Fluke object
type FlukeReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=fluke.example.com,resources=flukes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=fluke.example.com,resources=flukes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=fluke.example.com,resources=flukes/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete

// Reconcile is Lesson 3's TRIVIAL version: fetch the Fluke, log it, create
// its owned Deployment if missing. Deliberately not doing yet (see
// docs/notes/03-fluke-design.md for the full design): correcting drift if
// the Deployment already differs from spec, the finalizer, and device
// allocation. Those are Lessons 4-6 - added on top of this same shape.
func (r *FlukeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	var fluke flukev1alpha1.Fluke
	if err := r.Get(ctx, req.NamespacedName, &fluke); err != nil {
		if apierrors.IsNotFound(err) {
			// The Fluke was deleted after this reconcile was queued - nothing to do.
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	log.Info("reconciling Fluke", "replicas", fluke.Spec.Replicas, "image", fluke.Spec.Image)

	var dep appsv1.Deployment
	err := r.Get(ctx, req.NamespacedName, &dep)
	if apierrors.IsNotFound(err) {
		dep = buildDeployment(&fluke)
		// Ties the Deployment's lifecycle to the Fluke's: delete the Fluke,
		// Kubernetes garbage-collects the Deployment too. Also what makes
		// this Deployment "owned" for the .Owns() watch below.
		if err := ctrl.SetControllerReference(&fluke, &dep, r.Scheme); err != nil {
			return ctrl.Result{}, err
		}
		log.Info("Deployment missing - creating it", "name", dep.Name)
		if err := r.Create(ctx, &dep); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	} else if err != nil {
		return ctrl.Result{}, err
	}

	log.Info("Deployment already exists - nothing to do yet (drift correction is Lesson 4)", "name", dep.Name)
	return ctrl.Result{}, nil
}

// buildDeployment turns a Fluke's spec into the Deployment the controller
// will create and own. A plain function, not a method: it only reads its
// argument, it doesn't need a Fluke or *FlukeReconciler receiver.
func buildDeployment(fluke *flukev1alpha1.Fluke) appsv1.Deployment {
	labels := map[string]string{
		"app.kubernetes.io/name":     "fluke",
		"app.kubernetes.io/instance": fluke.Name,
	}
	return appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fluke.Name,
			Namespace: fluke.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &fluke.Spec.Replicas,
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "workload",
						Image: fluke.Spec.Image,
					}},
				},
			},
		},
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *FlukeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&flukev1alpha1.Fluke{}).
		// Also reconcile when an owned Deployment changes - needed once
		// Lesson 4 adds drift correction; harmless to wire up now.
		Owns(&appsv1.Deployment{}).
		Named("fluke").
		Complete(r)
}
