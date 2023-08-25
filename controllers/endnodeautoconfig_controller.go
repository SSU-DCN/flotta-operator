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
	"regexp"
	"strings"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/project-flotta/flotta-operator/api/v1alpha1"
	managementv1alpha1 "github.com/project-flotta/flotta-operator/api/v1alpha1"
	"github.com/project-flotta/flotta-operator/internal/common/repository/edgedevice"
	"github.com/project-flotta/flotta-operator/internal/common/repository/edgeworkload"
	"github.com/project-flotta/flotta-operator/internal/common/repository/endnodeautoconfig"
	"github.com/project-flotta/flotta-operator/internal/common/utils"
)

const EndNodeDeviceReferenceFinalizer = "end-node-device-reference-finalizer"

// EndNodeAutoConfigReconciler reconciles a EndNodeAutoConfig object
type EndNodeAutoConfigReconciler struct {
	client.Client
	Scheme                      *runtime.Scheme
	EdgeWorkloadRepository      edgeworkload.Repository
	EndNodeAutoConfigRepository endnodeautoconfig.Repository
	EdgeDeviceRepository        edgedevice.Repository
	Concurrency                 uint
	ExecuteConcurrent           func(context.Context, uint, ConcurrentFunc, []managementv1alpha1.EdgeDevice) []error
	MaxConcurrentReconciles     int
}

//+kubebuilder:rbac:groups=management.project-flotta.io,resources=endnodeautoconfigs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=management.project-flotta.io,resources=endnodeautoconfigs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=management.project-flotta.io,resources=endnodeautoconfigs/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the EndNodeAutoConfig object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.2/pkg/reconcile
func (r *EndNodeAutoConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling", "endNodeAutoConfig", req)

	// your logic here
	enac, err := r.EndNodeAutoConfigRepository.Read(ctx, req.Name, req.Namespace)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Error(err, "endNodeAutoConfig", " ERROR OCCURRED NOT FOUND")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "endNodeAutoConfig", " ERROR OCCURRED NOT FOUND, REQUEWE")
		return ctrl.Result{Requeue: true}, err
	}

	logger.Info("endNodeAutoConfig", "success GOIG DOQN")

	if enac.DeletionTimestamp == nil && !utils.HasFinalizer(&enac.ObjectMeta, EndNodeDeviceReferenceFinalizer) {
		// WorkloadCopy := enac.DeepCopy()
		// WorkloadCopy.Finalizers = []string{YggdrasilDeviceReferenceFinalizer}
		// err := r.EndNodeAutoConfigRepository.Patch(ctx, enac, enac)
		// if err != nil {
		// 	return ctrl.Result{Requeue: true}, err
		// }
		// return ctrl.Result{Requeue: true}, nil
		logger.Error(err, "endNodeAutoConfig", " CHECK success DELETION")
	}

	if enac.DeletionTimestamp != nil {
		return ctrl.Result{}, nil
	}

	logger.Info("CREATE PLUGIN", "endNodeAutoConfig", len(enac.Spec.Configuration.DevicePlugin.Containers))
	if len(enac.Spec.Configuration.DevicePlugin.Containers) == 0 {
		fmt.Printf("PLUGIN LENGTH IS 0")
		return ctrl.Result{}, err
	}

	input := enac.Spec.Configuration.Connection + enac.Spec.Configuration.Protocol + enac.Spec.Device

	// Remove special characters
	reg := regexp.MustCompile("[^a-zA-Z0-9]+")
	cleanedText := reg.ReplaceAllString(input, "")

	// Remove white spaces
	cleanedText = strings.ReplaceAll(cleanedText, " ", "")

	// Convert to lowercase
	cleanedText = "plugin-" + strings.ToLower(cleanedText)

	err = r.createPluginForDevices(ctx, enac.Spec.Device, cleanedText, enac.Namespace, enac.Spec.Configuration.DevicePlugin)
	if err != nil {
		if errors.IsAlreadyExists(err) {
			logger.Error(err, "Erropr: error: ")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{Requeue: true}, err
	}

	return ctrl.Result{}, nil
}

func (r *EndNodeAutoConfigReconciler) createPluginForDevices(ctx context.Context, device_name, plugin_name, namespace string, devicePlugin v1alpha1.DevicePlugin) error {
	fmt.Printf("CREATE PLUGIN")
	logger := log.FromContext(ctx)
	logger.Info("PLUGIN endNodeAutoConfig")

	deviceConfigPlugins := []v1.Container{}
	for _, item := range devicePlugin.Containers {
		container := v1.Container{
			Name:  item.Name,
			Image: item.Image,
		}
		deviceConfigPlugins = append(deviceConfigPlugins, container)
	}

	edgeWorkloadDevicePlugin := &v1alpha1.EdgeWorkload{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      plugin_name,
			Namespace: namespace,
		},
		Spec: v1alpha1.EdgeWorkloadSpec{
			Device: device_name,
			Type:   v1alpha1.PodWorkloadType,
			Pod: v1alpha1.Pod{
				Spec: v1.PodSpec{
					Containers: append([]v1.Container{}, deviceConfigPlugins...),
				},
			},
		},
	}

	workload, err := r.EdgeWorkloadRepository.Read(ctx, plugin_name, namespace)

	if err == nil {
		// The EdgeWorkload already exists, no need to recreate
		fmt.Print("The EdgeWorkload already exists, no need to recreate")
		logger.Info("PLUGIN1 The EdgeWorkload already exists, no need to recreate success 2")
		return nil
	} else if errors.IsNotFound(err) {
		// An error occurred while trying to retrieve the EdgeWorkload
		workloadCreateError := r.EdgeDeviceRepository.CreateEdgeWorkload(ctx, edgeWorkloadDevicePlugin)
		if workloadCreateError != nil {
			fmt.Printf("Error occured creating device plugin: %s", workloadCreateError.Error())
			logger.Error(workloadCreateError, "PLUGIN1 The EdgeWorkload already exists, no need to recreate success 3")

			return workloadCreateError
		}
		return workloadCreateError
	} else if workload.Name == "" {

		logger.Info("PLUGIN31ww check returned name success 5")
		workloadCreateError := r.EdgeDeviceRepository.CreateEdgeWorkload(ctx, edgeWorkloadDevicePlugin)
		if workloadCreateError != nil {
			fmt.Printf("Error occured creating device plugin: %s", workloadCreateError.Error())
			logger.Error(workloadCreateError, "PLUGIN1 The EdgeWorkload already exists, no need to recreate success 3")

			return workloadCreateError
		}
		return workloadCreateError

	}

	logger.Info("PLUGIN1 No statme,memeyts make sense success 4")

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *EndNodeAutoConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&managementv1alpha1.EndNodeAutoConfig{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: r.MaxConcurrentReconciles}).
		Complete(r)
}
