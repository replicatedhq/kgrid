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
	kuberneteserrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kgridv1alpha1 "github.com/replicatedhq/kgrid/apis/kgrid/v1alpha1"
)

var (
	errNoCluster = errors.New("no matching clusters found")
)

// ApplicationReconciler reconciles a Application object
type ApplicationReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=kgrid.replicated.com,resources=applications,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kgrid.replicated.com,resources=applications/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kgrid.replicated.com,resources=applications/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Application object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.8.3/pkg/reconcile
func (r *ApplicationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Log.WithValues("application", req.NamespacedName)
	logger.Info("reconciling app")

	instance := &kgridv1alpha1.Application{}
	err := r.Get(context.Background(), req.NamespacedName, instance)
	if err != nil {
		if kuberneteserrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		logger.Error(err, "failed to get application instance")
		return ctrl.Result{}, err
	}

	if instance.Spec.KOTS != nil {
		result, err := r.reconcileKotsApplication(ctx, instance.Namespace, instance.Spec.KOTS)
		if err == nil {
			return result, nil
		}

		if errors.Cause(err) == errNoCluster {
			logger.Info(err.Error())
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, errors.Wrap(err, "failed to reconcile KOTS app")
	}

	return ctrl.Result{}, errors.New("no supported applications found")
}

// SetupWithManager sets up the controller with the Manager.
func (r *ApplicationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kgridv1alpha1.Application{}).
		Complete(r)
}

func (r *ApplicationReconciler) reconcileKotsApplication(ctx context.Context, namespace string, kotsApp *kgridv1alpha1.KOTS) (ctrl.Result, error) {
	grids, err := listGrids(ctx, namespace)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to get grids")
	}

	testCreated := false
	for _, grid := range grids.Items {
		for _, gridCluster := range grid.Spec.Clusters {
			for _, appCluster := range kotsApp.Clusters {
				if gridCluster.Name != appCluster {
					continue
				}

				// TODO: start test
				testCreated = true
			}
		}
	}

	if !testCreated {
		return ctrl.Result{}, errNoCluster
	}

	return ctrl.Result{}, nil
}
