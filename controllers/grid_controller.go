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

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kgridv1alpha1 "github.com/replicatedhq/kgrid/apis/kgrid/v1alpha1"
	kgridclientset "github.com/replicatedhq/kgrid/pkg/client/kgridclientset/typed/kgrid/v1alpha1"
	"github.com/replicatedhq/kgrid/pkg/config"
)

// GridReconciler reconciles a Grid object
type GridReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=kgrid.replicated.com,resources=grids,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kgrid.replicated.com,resources=grids/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kgrid.replicated.com,resources=grids/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Grid object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.8.3/pkg/reconcile
func (r *GridReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Log.WithValues("grid", req.NamespacedName)

	instance := &kgridv1alpha1.Grid{}
	err := r.Get(context.Background(), req.NamespacedName, instance)
	if err != nil {
		logger.Error(err, "failed to get grid instance")
		return ctrl.Result{}, err
	}

	result, err := r.reconcileGrid(ctx, instance)
	if err != nil {
		logger.Error(err, "failed to reconcile grid")
		return ctrl.Result{}, err
	}

	return result, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *GridReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kgridv1alpha1.Grid{}).
		Complete(r)
}

func (r *GridReconciler) reconcileGrid(ctx context.Context, instance *kgridv1alpha1.Grid) (ctrl.Result, error) {
	// TODO: implement
	return ctrl.Result{}, nil
}

func listGrids(ctx context.Context, namespace string) (*kgridv1alpha1.GridList, error) {
	cfg, err := config.GetRESTConfig()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get config")
	}

	clientset, err := kgridclientset.NewForConfig(cfg)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create grid client")
	}

	grids, err := clientset.Grids(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "failed to list grids")
	}

	return grids, nil
}
