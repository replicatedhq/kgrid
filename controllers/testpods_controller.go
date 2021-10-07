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
	kgridv1alpha1 "github.com/replicatedhq/kgrid/apis/kgrid/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	kuberneteserrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const TestPodLabelKey = "kgrid.replicated.com/test"

// TestPodReconciler reconciles a test pod object
type TestPodReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=kgrid.replicated.com,namespace=kgrid-system,resources=pods,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kgrid.replicated.com,namespace=kgrid-system,resources=pods/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kgrid.replicated.com,namespace=kgrid-system,resources=pods/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the test pod object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.8.3/pkg/reconcile
func (r *TestPodReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Log.WithValues("test pod", req.NamespacedName)
	logger.Info("reconciling test pod")

	pod := &corev1.Pod{}
	err := r.Get(context.Background(), req.NamespacedName, pod)
	if err != nil {
		if kuberneteserrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, errors.Wrap(err, "failed to get test pod")
	}
	testID := pod.Labels[TestPodLabelKey]
	if testID == "" {
		return ctrl.Result{}, nil
	}

	testResult := getTestResultFromPod(pod)
	if testResult == "" {
		return ctrl.Result{}, nil
	}

	results, err := listResults(ctx, pod.Namespace)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to get test pod")
	}
	for _, results := range results.Items {
		updated := false

		for i, test := range results.Tests {
			if test.ID == testID {
				results.Tests[i].Result = testResult
				updated = true
			}
		}

		if !updated {
			continue
		}

		_, err = updateResults(ctx, &results)
		if err != nil {
			return ctrl.Result{}, errors.Wrap(err, "failed to update results")
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *TestPodReconciler) SetupWithManager(mgr ctrl.Manager) error {
	isTestPod, err := predicate.LabelSelectorPredicate(metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{
			{
				Key:      TestPodLabelKey,
				Operator: metav1.LabelSelectorOpExists,
			},
		},
	})
	if err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Pod{}).
		WithEventFilter(isTestPod).
		Complete(r)
}

func getTestResultFromPod(pod *corev1.Pod) kgridv1alpha1.TestResultStatus {
	switch pod.Status.Phase {
	case corev1.PodSucceeded:
		return kgridv1alpha1.TestResultPass
	case corev1.PodPending, corev1.PodRunning:
		return kgridv1alpha1.TestResultPending
	case corev1.PodFailed:
		return kgridv1alpha1.TestResultFail
	case corev1.PodUnknown:
		return kgridv1alpha1.TestResultUnknown
	}

	return kgridv1alpha1.TestResultUnknown
}
