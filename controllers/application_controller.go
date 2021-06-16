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
	"encoding/json"
	"fmt"

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
	kgridclientset "github.com/replicatedhq/kgrid/pkg/client/kgridclientset/typed/kgrid/v1alpha1"
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
//+kubebuilder:rbac:groups="",namespace=kgrid-system,resources=secrets,verbs=get;list;watch;create;update;patch;delete

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
		return ctrl.Result{}, errors.Wrap(err, "failed to get application instance")
	}

	version, err := findVersionForApp(ctx, instance.Namespace, instance)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to find version to test application with")
	}

	err = createAppTests(ctx, instance.Namespace, instance, version, logger)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to get application instance")
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ApplicationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kgridv1alpha1.Application{}).
		Complete(r)
}

func getTestID(cluster string, version string, channelID string, channelSequence uint) string {
	m := map[string]interface{}{
		"cluster":         cluster,
		"version":         version,
		"channelID":       channelID,
		"channelSequence": channelSequence,
	}
	s, _ := json.Marshal(m)
	return fmt.Sprintf("%x", md5.Sum(s))
}

func getPodName(testID string) string {
	return fmt.Sprintf("test-%s", testID)
}

func createAppTests(ctx context.Context, namespace string, app *kgridv1alpha1.Application, version string, logger logr.Logger) error {
	channelID := ""
	channelSequence := uint(0)
	appClusters := []string{}

	if app.Spec.KOTS != nil {
		channelID = app.Spec.KOTS.ChannelID
		channelSequence = app.Spec.KOTS.ChannelSequence
		if version == "" {
			version = app.Spec.KOTS.Version
		}
		appClusters = app.Spec.KOTS.Clusters
	} else {
		return errors.New("no supported applications found")
	}

	grids, err := listGrids(ctx, namespace)
	if err != nil {
		return errors.Wrap(err, "failed to get grids")
	}

	cfg, err := config.GetRESTConfig()
	if err != nil {
		return errors.Wrap(err, "failed to get config")
	}

	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return errors.Wrap(err, "failed to create k8s client")
	}

	foundCluster := false
	for _, grid := range grids.Items {
		for _, gridCluster := range grid.Spec.Clusters {
			for _, appCluster := range appClusters {
				if gridCluster.Name != appCluster {
					continue
				}

				foundCluster = true

				testID := getTestID(gridCluster.Name, version, channelID, channelSequence)
				_, err := clientset.CoreV1().Pods(app.Namespace).Get(ctx, getPodName(testID), metav1.GetOptions{})
				if err == nil {
					continue
				}
				if !kuberneteserrors.IsNotFound(err) {
					return errors.Wrap(err, "failed to check if test exists")
				}

				configSpec, err := getKotsTestConfigMap(ctx, testID, &gridCluster, app, version)
				if err != nil {
					return errors.Wrap(err, "failed to build test configmap")
				}

				_, err = clientset.CoreV1().ConfigMaps(app.Namespace).Create(ctx, configSpec, metav1.CreateOptions{})
				if err != nil && !kuberneteserrors.IsAlreadyExists(err) {
					return errors.Wrap(err, "failed to create test config")
				}

				podSpec := getTestPodSpec(testID, &gridCluster, app)
				_, err = clientset.CoreV1().Pods(app.Namespace).Create(ctx, podSpec, metav1.CreateOptions{})
				if err != nil {
					return errors.Wrap(err, "failed to create test")
				}
			}
		}
	}

	if !foundCluster {
		logger.Info("no cluster found for app %s", app.Name)
		return nil
	}

	return nil
}

func getTestPodSpec(testID string, gridCluster *kgridv1alpha1.Cluster, app *kgridv1alpha1.Application) *corev1.Pod {
	// TODO: pass test ID to grid pod/command
	podSpec := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: getPodName(testID),
		},
		Spec: corev1.PodSpec{
			Affinity: &corev1.Affinity{
				NodeAffinity: defaultKgridNodeAffinity(),
			},
			RestartPolicy: corev1.RestartPolicyNever,
			Containers: []corev1.Container{
				{
					Image:           fmt.Sprintf("%s:%s", kgridImageName(), buildversion.ImageTag()),
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
								Name: getPodName(testID),
							},
						},
					},
				},
			},
		},
	}

	return podSpec
}

func getKotsTestConfigMap(ctx context.Context, testID string, gridCluster *kgridv1alpha1.Cluster, app *kgridv1alpha1.Application, version string) (*corev1.ConfigMap, error) {
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getPodName(testID),
			Namespace: app.Namespace,
			Labels:    map[string]string{},
		},
		Data: map[string]string{},
	}

	gridSpec, err := getGridSpecForTest(ctx, app.Namespace, gridCluster)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build gird spec")
	}

	gridYaml, err := yaml.Marshal(gridSpec)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal gird spec")
	}
	configMap.Data["grid.yaml"] = string(gridYaml)

	appSpec, err := getAppSpecForTest(app, version)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build app spec")
	}

	appYaml, err := yaml.Marshal(appSpec)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal app spec")
	}
	configMap.Data["app.yaml"] = string(appYaml)

	return configMap, nil
}

func getGridSpecForTest(ctx context.Context, namespace string, gridCluster *kgridv1alpha1.Cluster) (*gridtypes.Grid, error) {
	g := &gridtypes.Grid{
		ObjectMeta: metav1.ObjectMeta{
			Name: gridCluster.Name,
		},
		Spec: gridtypes.GridSpec{},
	}

	accessKeyID, err := gridCluster.EKS.AaccessKeyID.String(ctx, namespace)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get access key ID")
	}

	secretAccessKey, err := gridCluster.EKS.SecretAccessKey.String(ctx, namespace)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get secret access key")
	}

	clusterSpec := &gridtypes.ClusterSpec{}
	if gridCluster.EKS != nil {
		clusterSpec.EKS = &gridtypes.EKSSpec{}
		if gridCluster.EKS.Create {
			clusterSpec.EKS.NewCluster = &gridtypes.EKSNewClusterSpec{
				Description: "",
				Version:     gridCluster.EKS.Version,
				AccessKeyID: gridtypes.ValueOrValueFrom{
					Value: accessKeyID,
				},
				SecretAccessKey: gridtypes.ValueOrValueFrom{
					Value: secretAccessKey,
				},
				Region: gridCluster.EKS.Region,
			}
		} else {
			clusterSpec.EKS.ExistingCluster = &gridtypes.EKSExistingClusterSpec{
				AccessKeyID: gridtypes.ValueOrValueFrom{
					Value: accessKeyID,
				},
				SecretAccessKey: gridtypes.ValueOrValueFrom{
					Value: secretAccessKey,
				},
				ClusterName: gridCluster.Name,
				Region:      gridCluster.EKS.Region,
			}
		}
	}

	g.Spec.Clusters = []*gridtypes.ClusterSpec{clusterSpec}

	return g, nil
}

func getAppSpecForTest(app *kgridv1alpha1.Application, version string) (*gridtypes.Application, error) {
	if app.Spec.KOTS == nil {
		return nil, errors.New("KOTS app is required")
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

	if version != "" {
		a.Spec.KOTSApplicationSpec.Version = version
	}

	return a, nil
}

func kgridImageName() string {
	// TODO: Use kustomize and set image name in env variable
	if buildversion.ImageTag() == "v0.0.0" {
		return "localhost:32000/kgrid/kgird"
	}
	return "replicated/kgrid"
}

func defaultKgridNodeAffinity() *corev1.NodeAffinity {
	return &corev1.NodeAffinity{
		RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
			NodeSelectorTerms: []corev1.NodeSelectorTerm{
				{
					MatchExpressions: []corev1.NodeSelectorRequirement{
						{
							Key:      "kubernetes.io/os",
							Operator: corev1.NodeSelectorOpIn,
							Values: []string{
								"linux",
							},
						},
						{
							Key:      "kubernetes.io/arch",
							Operator: corev1.NodeSelectorOpNotIn,
							Values: []string{
								"arm64",
							},
						},
					},
				},
			},
		},
	}
}

func findVersionForApp(ctx context.Context, namespace string, app *kgridv1alpha1.Application) (string, error) {
	if app.Spec.KOTS == nil {
		return "", errors.Errorf("app %s has no supported app type", app.Name)
	}

	if app.Spec.KOTS.Version != "latest" && app.Spec.KOTS.Version != "" {
		return app.Spec.KOTS.Version, nil
	}

	versions, err := listVersions(ctx, namespace)
	if err != nil {
		return "", errors.Wrap(err, "failed to list versions")
	}

	for _, version := range versions.Items {
		if version.Spec.KOTS != nil {
			return version.Spec.KOTS.Latest, nil
		}
	}

	return "", errors.Errorf("no version found for app %s", app.Name)
}

func listApplications(ctx context.Context, namespace string) (*kgridv1alpha1.ApplicationList, error) {
	cfg, err := config.GetRESTConfig()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get config")
	}

	clientset, err := kgridclientset.NewForConfig(cfg)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create app client")
	}

	apps, err := clientset.Applications(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "failed to list apps")
	}

	return apps, nil
}
