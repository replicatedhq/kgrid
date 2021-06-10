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
	"crypto/md5"
	"fmt"
	"os"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	kuberneteserrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	kgridv1alpha1 "github.com/replicatedhq/kgrid/apis/kgrid/v1alpha1"
	"github.com/replicatedhq/kgrid/pkg/buildversion"
	"github.com/replicatedhq/kgrid/pkg/config"
	gridtypes "github.com/replicatedhq/kgrid/pkg/kgrid/grid/types"
)

// ApplicationReconciler reconciles a Application object
type ApplicationReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=kgrid.replicated.com,namespace=kgrid-system,resources=applications,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kgrid.replicated.com,namespace=kgrid-system,resources=applications/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kgrid.replicated.com,namespace=kgrid-system,resources=applications/finalizers,verbs=update
//+kubebuilder:rbac:groups="",namespace=kgrid-system,resources=pods,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",namespace=kgrid-system,resources=configmaps,verbs=get;list;watch;create;update;patch;delete

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
	return r.createJob(ctx, req)
}

func (r *ApplicationReconciler) createJob(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
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

	appTestNamePart := ""
	appClusters := []string{}

	if instance.Spec.KOTS != nil {
		appTestNamePart = fmt.Sprintf("%s-%d", instance.Spec.KOTS.ChannelID, instance.Spec.KOTS.ChannelSequence)
		appClusters = instance.Spec.KOTS.Clusters
	} else {
		return ctrl.Result{}, errors.New("no supported applications found")
	}

	grids, err := listGrids(ctx, instance.Namespace)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to get grids")
	}

	cfg, err := config.GetRESTConfig()
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to get config")
	}

	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to create grid client")
	}

	foundCluster := false
	for _, grid := range grids.Items {
		for _, gridCluster := range grid.Spec.Clusters {
			for _, appCluster := range appClusters {
				if gridCluster.Name != appCluster {
					continue
				}

				foundCluster = true

				testName := getDeterministicTestName(gridCluster.Name, appTestNamePart)
				_, err := clientset.CoreV1().Pods(instance.Namespace).Get(ctx, testName, metav1.GetOptions{})
				if err == nil {
					continue
				}
				if !kuberneteserrors.IsNotFound(err) {
					return ctrl.Result{}, errors.Wrap(err, "failed to check if test exists")
				}

				configSpec, err := getKotsTestConfigMap(testName, &gridCluster, instance)
				if err != nil {
					return ctrl.Result{}, errors.Wrap(err, "failed to build test configmap")
				}

				_, err = clientset.CoreV1().ConfigMaps(instance.Namespace).Create(ctx, configSpec, metav1.CreateOptions{})
				if err != nil && !kuberneteserrors.IsAlreadyExists(err) {
					return ctrl.Result{}, errors.Wrap(err, "failed to create test config")
				}

				podSpec := getTestPodSpec(testName, &gridCluster, instance)
				_, err = clientset.CoreV1().Pods(instance.Namespace).Create(ctx, podSpec, metav1.CreateOptions{})
				if err != nil {
					return ctrl.Result{}, errors.Wrap(err, "failed to create test")
				}
			}
		}
	}

	if !foundCluster {
		logger.Info("no cluster found for app %s", instance.Name)
		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ApplicationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kgridv1alpha1.Application{}).
		Complete(r)
}

func getDeterministicTestName(appClusters string, appNamePart string) string {
	return fmt.Sprintf("test-%x", md5.Sum([]byte(fmt.Sprintf("%s-%s", appClusters, appNamePart))))
}

func getTestPodSpec(testName string, gridCluster *kgridv1alpha1.Cluster, app *kgridv1alpha1.Application) *corev1.Pod {
	podSpec := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: testName,
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			Containers: []corev1.Container{
				{
					Image:           fmt.Sprintf("replicated/kgrid:%s", buildversion.ImageTag()),
					ImagePullPolicy: corev1.PullAlways,
					Name:            "grid",
					Command:         []string{"kgrid"},
					Args: []string{
						"create",
						"--from-yaml",
						"/kgrid-specs/grid.yaml",
						"--app",
						"/kgrid-specs/app.yaml",
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "kgrid-specs",
							MountPath: "/kgrid-specs",
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "kgrid-specs",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: testName,
							},
						},
					},
				},
			},
		},
	}

	return podSpec
}

func getKotsTestConfigMap(testName string, gridCluster *kgridv1alpha1.Cluster, app *kgridv1alpha1.Application) (*corev1.ConfigMap, error) {
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testName,
			Namespace: os.Getenv("POD_NAMESPACE"),
			Labels:    map[string]string{},
		},
		Data: map[string]string{},
	}

	gridYaml, err := yaml.Marshal(getGridSpecForTest(gridCluster))
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal gird spec")
	}
	configMap.Data["grid.yaml"] = string(gridYaml)

	appYaml, err := yaml.Marshal(getAppSpecForTest(app))
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal app spec")
	}
	configMap.Data["app.yaml"] = string(appYaml)

	return configMap, nil
}

func getGridSpecForTest(gridCluster *kgridv1alpha1.Cluster) *gridtypes.Grid {
	g := &gridtypes.Grid{
		ObjectMeta: metav1.ObjectMeta{
			Name: gridCluster.Name,
		},
		Spec: gridtypes.GridSpec{},
	}

	clusterSpec := &gridtypes.ClusterSpec{}
	if gridCluster.EKS != nil {
		clusterSpec.EKS = &gridtypes.EKSSpec{}
		if gridCluster.EKS.Create {
			clusterSpec.EKS.NewCluster = &gridtypes.EKSNewClusterSpec{
				Description: "",
				Version:     gridCluster.EKS.Version,
				AccessKeyID: gridtypes.ValueOrValueFrom{
					Value: gridCluster.EKS.AaccessKeyID.String(),
				},
				SecretAccessKey: gridtypes.ValueOrValueFrom{
					Value: gridCluster.EKS.SecretAccessKey.String(),
				},
				Region: gridCluster.EKS.Region,
			}
		} else {
			clusterSpec.EKS.ExistingCluster = &gridtypes.EKSExistingClusterSpec{
				AccessKeyID: gridtypes.ValueOrValueFrom{
					Value: gridCluster.EKS.AaccessKeyID.String(),
				},
				SecretAccessKey: gridtypes.ValueOrValueFrom{
					Value: gridCluster.EKS.SecretAccessKey.String(),
				},
				ClusterName: gridCluster.Name,
				Region:      gridCluster.EKS.Region,
			}
		}
	}

	g.Spec.Clusters = []*gridtypes.ClusterSpec{clusterSpec}

	return g
}

func getAppSpecForTest(app *kgridv1alpha1.Application) *gridtypes.Application {
	if app.Spec.KOTS == nil {
		return nil // TODO: ++++ error
	}
	a := &gridtypes.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name: app.Name,
		},
		Spec: gridtypes.ApplicationSpec{
			KOTSApplicationSpec: &gridtypes.KOTSApplicationSpec{
				Version:        app.Spec.KOTS.Version,
				App:            app.Spec.KOTS.AppSlug,
				LicenseID:      app.Spec.KOTS.LicenseID,
				SkipPreflights: &app.Spec.KOTS.SkipPreflights,
				Namespace:      app.Spec.KOTS.Namespace,
				ConfigValues:   app.Spec.KOTS.ConfigValues.Spec.DeepCopy(),
			},
		},
	}
	return a
}
