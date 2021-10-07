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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kgridv1alpha1 "github.com/replicatedhq/kgrid/apis/kgrid/v1alpha1"
	kgridclientset "github.com/replicatedhq/kgrid/pkg/client/kgridclientset/typed/kgrid/v1alpha1"
	"github.com/replicatedhq/kgrid/pkg/config"
)

// VersionReconciler reconciles a Version object
type VersionReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=kgrid.replicated.com,namespace=kgrid-system,resources=versions,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kgrid.replicated.com,namespace=kgrid-system,resources=versions/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kgrid.replicated.com,namespace=kgrid-system,resources=versions/finalizers,verbs=update

//+kubebuilder:rbac:groups="",namespace=kgrid-system,resources=configmaps,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Version object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.8.3/pkg/reconcile
func (r *VersionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Log.WithValues("version", req.NamespacedName)
	logger.Info("reconciling version")

	instance := &kgridv1alpha1.Version{}
	err := r.Get(context.Background(), req.NamespacedName, instance)
	if err != nil {
		if kuberneteserrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, errors.Wrap(err, "failed to get version instance")
	}

	if instance.Spec.KOTS == nil || instance.Spec.KOTS.Latest == "" {
		return ctrl.Result{}, errors.New("no supported version found")
	}

	apps, err := listApplications(ctx, instance.Namespace)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to get grids")
	}

	tests := []kgridv1alpha1.Test{}

	for _, app := range apps.Items {
		if app.Spec.KOTS == nil || app.Spec.KOTS.Version != "latest" {
			continue
		}

		appTests, err := createAppTests(ctx, app.Namespace, &app, instance.Spec.KOTS.Latest, logger)
		if err != nil {
			return ctrl.Result{}, errors.Wrap(err, "failed to create application test")
		}
		tests = append(tests, appTests...)
	}

	outcomeName := instance.Labels["runId"]
	if outcomeName != "" {
		outcome := &kgridv1alpha1.Outcome{
			ObjectMeta: metav1.ObjectMeta{
				Name:      outcomeName,
				Namespace: instance.Namespace,
			},
			Tests: tests,
		}
		err := createOutcome(ctx, outcome)
		if err != nil {
			return ctrl.Result{}, errors.Wrapf(err, "failed to create Outcome %s", outcomeName)
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *VersionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kgridv1alpha1.Version{}).
		Complete(r)
}

func listVersions(ctx context.Context, namespace string) (*kgridv1alpha1.VersionList, error) {
	cfg, err := config.GetRESTConfig()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get config")
	}

	clientset, err := kgridclientset.NewForConfig(cfg)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create app client")
	}

	versions, err := clientset.Versions(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "failed to list apps")
	}

	return versions, nil
}
