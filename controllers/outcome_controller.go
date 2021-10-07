/*
Copyright 2021.

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

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	kuberneteserrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	kgridv1alpha1 "github.com/replicatedhq/kgrid/apis/kgrid/v1alpha1"
	kgridclientset "github.com/replicatedhq/kgrid/pkg/client/kgridclientset/typed/kgrid/v1alpha1"
	"github.com/replicatedhq/kgrid/pkg/config"
)

// OutcomeReconciler reconciles a Outcome object
type OutcomeReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=kgrid.replicated.com,namespace=kgrid-system,resources=outcomes,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kgrid.replicated.com,namespace=kgrid-system,resources=outcomes/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kgrid.replicated.com,namespace=kgrid-system,resources=outcomes/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Outcome object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.8.3/pkg/reconcile
func (r *OutcomeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// This only runs on create events to get the statuses of any pods that might have completed
	// before this was created. The test pods controller will update this Outcome with future changes.
	instance := &kgridv1alpha1.Outcome{}
	err := r.Get(context.Background(), req.NamespacedName, instance)
	if err != nil {
		if kuberneteserrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, errors.Wrap(err, "failed to get outcome instance")
	}

	cfg, err := config.GetRESTConfig()
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to get config")
	}
	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to create client")
	}

	// TODO this could become inefficient if there get to be a lot of test pods
	pods, err := clientset.CoreV1().Pods(instance.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=", TestPodLabelKey),
	})
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to list test pods")
	}

	for _, pod := range pods.Items {
		podTestID := pod.Labels[TestPodLabelKey]

		for i, test := range instance.Tests {
			if test.ID != podTestID {
				continue
			}

			instance.Tests[i].Result = getTestResultFromPod(&pod)
		}
	}

	_, err = updateOutcome(ctx, instance)
	if err != nil {
		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *OutcomeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	isCreate := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return true
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return false
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&kgridv1alpha1.Outcome{}).
		WithEventFilter(isCreate).
		Complete(r)
}

func listOutcomes(ctx context.Context, namespace string) (*kgridv1alpha1.OutcomeList, error) {
	cfg, err := config.GetRESTConfig()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get config")
	}

	clientset, err := kgridclientset.NewForConfig(cfg)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create app client")
	}

	outcomes, err := clientset.Outcomes(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "failed to list apps")
	}

	return outcomes, nil
}

func createOutcome(ctx context.Context, outcome *kgridv1alpha1.Outcome) error {
	cfg, err := config.GetRESTConfig()
	if err != nil {
		return errors.Wrap(err, "failed to get config")
	}

	clientset, err := kgridclientset.NewForConfig(cfg)
	if err != nil {
		return errors.Wrap(err, "failed to create app client")
	}

	_, err = clientset.Outcomes(outcome.Namespace).Create(ctx, outcome, metav1.CreateOptions{})
	if err != nil {
		if kuberneteserrors.IsAlreadyExists(err) {
			return nil
		}

		return errors.Wrap(err, "failed to create outcome")
	}

	return nil
}

func updateOutcome(ctx context.Context, outcome *kgridv1alpha1.Outcome) (*kgridv1alpha1.Outcome, error) {
	cfg, err := config.GetRESTConfig()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get config")
	}

	clientset, err := kgridclientset.NewForConfig(cfg)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create app client")
	}

	outcome, err = clientset.Outcomes(outcome.Namespace).Update(ctx, outcome, metav1.UpdateOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "failed to update outcome")
	}

	return outcome, nil
}
