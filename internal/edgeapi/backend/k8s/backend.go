package k8s

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/project-flotta/flotta-operator/api/v1alpha1"
	"github.com/project-flotta/flotta-operator/internal/common/labels"
	backendapi "github.com/project-flotta/flotta-operator/internal/edgeapi/backend"
	"github.com/project-flotta/flotta-operator/internal/edgeapi/hardware"
	"github.com/project-flotta/flotta-operator/models"
	"github.com/project-flotta/flotta-operator/pkg/mtls"
)

const (
	AuthzKey mtls.RequestAuthKey = "APIAuthzkey"
)

type backend struct {
	logger           *zap.SugaredLogger
	repository       RepositoryFacade
	assembler        *ConfigurationAssembler
	initialNamespace string
	heartbeatHandler *SynchronousHandler
}

func NewBackend(repository RepositoryFacade, assembler *ConfigurationAssembler,
	logger *zap.SugaredLogger, initialNamespace string, recorder record.EventRecorder) backendapi.EdgeDeviceBackend {
	return &backend{repository: repository,
		assembler:        assembler,
		logger:           logger,
		initialNamespace: initialNamespace,
		heartbeatHandler: NewSynchronousHandler(repository, recorder, logger)}
}

func (b *backend) GetRegistrationStatus(ctx context.Context, name, namespace string) (backendapi.RegistrationStatus, error) {
	edgeDevice, err := b.repository.GetEdgeDevice(ctx, name, namespace)
	if err != nil {
		if errors.IsNotFound(err) {
			return backendapi.Unregistered, nil
		}
		return backendapi.Unknown, err
	}

	if edgeDevice.DeletionTimestamp != nil {
		return backendapi.Unregistered, nil
	}

	return backendapi.Registered, nil
}

func (b *backend) GetConfiguration(ctx context.Context, name, namespace string) (*models.DeviceConfigurationMessage, error) {
	logger := b.logger.With("DeviceID", name, "Namespace", namespace)
	edgeDevice, err := b.repository.GetEdgeDevice(ctx, name, namespace)
	if err != nil {
		return nil, err
	}

	return b.assembler.GetDeviceConfiguration(ctx, edgeDevice, logger)
}

func (b *backend) Enrol(ctx context.Context, name, namespace string, enrolmentInfo *models.EnrolmentInfo) (bool, error) {
	_, err := b.repository.GetEdgeDevice(ctx, name, namespace)
	if err == nil {
		// Device is already created.
		return true, nil
	}

	edsr, err := b.repository.GetEdgeDeviceSignedRequest(ctx, name, b.initialNamespace)
	if err == nil {
		// Is already created, but not approved
		if edsr.Spec.TargetNamespace != namespace {
			_, err = b.repository.GetEdgeDevice(ctx, name, edsr.Spec.TargetNamespace)
			if err == nil {
				// Device is already created.
				return true, nil
			}
		}
		return false, nil
	}

	edsr = &v1alpha1.EdgeDeviceSignedRequest{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: b.initialNamespace,
		},
		Spec: v1alpha1.EdgeDeviceSignedRequestSpec{
			TargetNamespace: namespace,
			Approved:        false,
			Features: &v1alpha1.Features{
				Hardware: hardware.MapHardware(enrolmentInfo.Features.Hardware),
			},
		},
	}

	return false, b.repository.CreateEdgeDeviceSignedRequest(ctx, edsr)
}

func (b *backend) GetEdgeDevice(ctx context.Context, name, namespace string) (*v1alpha1.EdgeDevice, error) {
	return b.repository.GetEdgeDevice(ctx, name, namespace)
}

func (b *backend) GetTargetNamespace(ctx context.Context, name, identityNamespace string, matchesCertificate bool) (string, error) {
	logger := b.logger.With("DeviceID", name)
	namespace := identityNamespace
	if identityNamespace == b.initialNamespace && !matchesCertificate {
		// check if it's a valid device, shouldn't match
		esdr, err := b.repository.GetEdgeDeviceSignedRequest(ctx, name, b.initialNamespace)
		if err != nil {
			return "", err
		}
		if esdr.Spec.TargetNamespace != "" {
			namespace = esdr.Spec.TargetNamespace
		}
	}
	dvc, err := b.repository.GetEdgeDevice(ctx, name, namespace)
	if err != nil {
		if errors.IsNotFound(err) {
			return "", backendapi.NewNotApproved(err)
		}
		return "", err
	}

	if dvc == nil {
		return "", fmt.Errorf("device not found")
	}

	isInit := false
	if dvc.ObjectMeta.Labels[v1alpha1.EdgeDeviceSignedRequestLabelName] == v1alpha1.EdgeDeviceSignedRequestLabelValue {
		isInit = true
	}

	// the first time that tries to register should be able to use register certificate.
	if !isInit && !matchesCertificate {
		authKeyVal, _ := ctx.Value(AuthzKey).(mtls.RequestAuthVal)
		logger.With("certcn", authKeyVal.CommonName).Debug("Device tries to re-register with an invalid certificate")
		// At this moment, the registration certificate it's no longer valid,
		// because the CR is already created, and need to be a device
		// certificate.
		return "", fmt.Errorf("forbidden")
	}

	if isInit {
		logger.Info("EdgeDevice registered correctly for first time")
	} else {
		logger.Info("EdgeDevice renew registration correctly")
	}
	return namespace, nil
}

func (b *backend) Register(ctx context.Context, name, namespace string, registrationInfo *models.RegistrationInfo) error {
	logger := b.logger.With("DeviceID", name, "Namespace", namespace)
	dvc, err := b.repository.GetEdgeDevice(ctx, name, namespace)
	deviceCopy := dvc.DeepCopy()
	for key, val := range hardware.MapLabels(registrationInfo.Hardware) {
		deviceCopy.ObjectMeta.Labels[key] = val
	}
	delete(deviceCopy.Labels, v1alpha1.EdgeDeviceSignedRequestLabelName)

	err = b.repository.PatchEdgeDevice(ctx, dvc, deviceCopy)
	if err != nil {
		logger.With("err", err).Error("cannot update edgedevice")
		return err
	}

	err = b.updateDeviceStatus(ctx, dvc, func(device *v1alpha1.EdgeDevice) {
		device.Status.Hardware = hardware.MapHardware(registrationInfo.Hardware)
	})
	return err
}

func (b *backend) UpdateStatus(ctx context.Context, name, namespace string, heartbeat *models.Heartbeat) (bool, error) {
	return b.heartbeatHandler.Process(ctx, name, namespace, heartbeat)
}

func (b *backend) GetPlaybookExecutions(ctx context.Context, deviceID, namespace string) ([]*models.PlaybookExecution, error) {
	logger := b.logger.With("DeviceID", deviceID, "Namespace", namespace)
	response := []*models.PlaybookExecution{}
	edgeDevice, err := b.repository.GetEdgeDevice(ctx, deviceID, namespace)
	if err != nil {
		return nil, err
	}

	for labelName, labelValue := range edgeDevice.Labels {
		if labels.IsEdgeConfigLabel(labelName) {
			playbookExecution, err := b.repository.GetPlaybookExecution(ctx, fmt.Sprintf("%s-%s", deviceID, labelValue), namespace)
			if err != nil {
				logger.Error(err, "cannot get playbook execution ", "playbook execution name ", fmt.Sprintf("%s-%s", deviceID, labelValue))
				return nil, err
			}
			response = append(response, &models.PlaybookExecution{Name: playbookExecution.Name, AnsiblePlaybookString: string(playbookExecution.Spec.Playbook.Content)})
		}
	}
	return response, nil
}

func (b *backend) updateDeviceStatus(ctx context.Context, device *v1alpha1.EdgeDevice, updateFunc func(d *v1alpha1.EdgeDevice)) error {
	patch := client.MergeFrom(device.DeepCopy())
	updateFunc(device)
	err := b.repository.PatchEdgeDeviceStatus(ctx, device, &patch)
	if err == nil {
		return nil
	}

	// retry patching the edge device status
	for i := 1; i < 4; i++ {
		time.Sleep(time.Duration(i*50) * time.Millisecond)
		device2, err := b.repository.GetEdgeDevice(ctx, device.Name, device.Namespace)
		if err != nil {
			continue
		}
		patch = client.MergeFrom(device2.DeepCopy())
		updateFunc(device2)
		err = b.repository.PatchEdgeDeviceStatus(ctx, device2, &patch)
		if err == nil {
			return nil
		}
	}
	return err
}

//NEW CODE
// ManageWirelessDevices implements backend.EdgeDeviceBackend.
func (b *backend) HandleWirelessDevices(ctx context.Context, name string, namespace string, wirelessDevices []*models.WirelessDevice) (bool, error) {
	edgeDevice, err := b.repository.GetEdgeDevice(ctx, name, namespace)
	if err != nil {
		return false, fmt.Errorf("failed to get edge device: %w", err)
	}

	oldEdgeDevice := edgeDevice.DeepCopy()
	SpecRegisteredDevices := oldEdgeDevice.Spec.WirelessDevices
	StatusRegisteredDevices := oldEdgeDevice.Status.WirelessDevices
	patch := client.MergeFrom(oldEdgeDevice)

	// matchedDevice := false
	for _, HBwirelessDevice := range wirelessDevices {
		// Check if the wireless device has a matching registered device
		matchedDevice := searchWirelessDevice(SpecRegisteredDevices, HBwirelessDevice.Name, HBwirelessDevice.Identifiers)

		if !matchedDevice {
			// Create a new instance of v1alpha1.WirelessDevices and populate it with the new values
			convertedDevice := &v1alpha1.WirelessDevices{
				Name:         HBwirelessDevice.Name,
				Manufacturer: HBwirelessDevice.Manufacturer,
				Model:        HBwirelessDevice.Model,
				SWVersion:    HBwirelessDevice.SwVersion,
				Identifiers:  HBwirelessDevice.Identifiers,
				Protocol:     HBwirelessDevice.Protocol,
				Connection:   HBwirelessDevice.Connection,
				DeviceType:   HBwirelessDevice.DeviceType,
			}

			// Set specific fields based on the device type
			if strings.ToLower(HBwirelessDevice.DeviceType) == "sensor" {
				convertedDevice.Readings = HBwirelessDevice.Readings
			} else {
				convertedDevice.State = HBwirelessDevice.State
			}

			// Append the new instance to edgeDevice.Spec.WirelessDevices
			edgeDevice.Spec.WirelessDevices = append(edgeDevice.Spec.WirelessDevices, convertedDevice)

			//check if the end node data matches the autoconfig
			hasApplied, err := b.applyWorkloadsFromEndNodeAutoConfig(ctx, edgeDevice, HBwirelessDevice)
			if !hasApplied {
				b.logger.Error("An error occurred apply for EndNodeAutoConfig: %s", err.Error())
			}
		}

		//for the status
		matchedDeviceStatus := searchWirelessDevice(StatusRegisteredDevices, HBwirelessDevice.Name, HBwirelessDevice.Identifiers)

		if !matchedDeviceStatus {
			// Create a new instance of v1alpha1.WirelessDevices and populate it with the new values
			convertedDevice := &v1alpha1.WirelessDevices{
				Name:         HBwirelessDevice.Name,
				Manufacturer: HBwirelessDevice.Manufacturer,
				Model:        HBwirelessDevice.Model,
				SWVersion:    HBwirelessDevice.SwVersion,
				Identifiers:  HBwirelessDevice.Identifiers,
				Protocol:     HBwirelessDevice.Protocol,
				Connection:   HBwirelessDevice.Connection,
				Battery:      HBwirelessDevice.Battery,
				DeviceType:   HBwirelessDevice.DeviceType,
				Availability: HBwirelessDevice.Availability,
				LastSeen:     HBwirelessDevice.LastSeen,
			}

			// Set specific fields based on the device type
			if strings.ToLower(HBwirelessDevice.DeviceType) == "sensor" {
				convertedDevice.Readings = HBwirelessDevice.Readings
			} else {
				convertedDevice.State = HBwirelessDevice.State
			}

			// Append the new instance to edgeDevice.Spec.WirelessDevices
			edgeDevice.Status.WirelessDevices = append(edgeDevice.Status.WirelessDevices, convertedDevice)
		}

	}

	edgeDeviceUP := edgeDevice.DeepCopy()

	if patch == nil {
		fmt.Println("")
	}
	if edgeDeviceUP == nil {
		fmt.Println("")
	}

	fmt.Println("Updated Check")

	// Patch the edgeDevice with the updated or added devices
	// if err := b.repository.PatchEdgeDeviceStatus(ctx, edgeDevice, &patch); err != nil {
	// 	return false, fmt.Errorf("failed to patch edge device status: %w", err)
	// }

	// // Patch the edgeDevice with the updated or added devices
	// if err := b.repository.PatchEdgeDevice(ctx, edgeDevice, edgeDeviceUP); err != nil {
	// 	return false, fmt.Errorf("failed to patch edge device: %w", err)
	// }

	return true, nil
}

func searchWirelessDevice(slice []*v1alpha1.WirelessDevices, targetName, targetIdentifiers string) bool {
	for _, device := range slice {
		if device.Name == targetName && device.Identifiers == targetIdentifiers {
			return true // Found the target WirelessDevice in the slice
		}
	}
	return false // Target WirelessDevice not found in the slice
}

func (b *backend) applyWorkloadsFromEndNodeAutoConfig(ctx context.Context, device *v1alpha1.EdgeDevice, wirelessDevice *models.WirelessDevice) (bool, error) {

	b.logger.Infof("WE ARE IN THE FUNCTION")

	// logger := b.logger.With("DeviceID", device.Name)

	listEndNodeAutoConfig, err := b.repository.ListEndNodeAutoConfigByEdgeDevice(ctx, device.Namespace, device.Name)
	if err != nil {
		return false, err
	}

	if len(listEndNodeAutoConfig) == 0 {
		return false, fmt.Errorf("%s", "No EndNodeAutoConfig resources")
	}

	b.logger.Infof("WE ARE IN THE FUNCTION 2")

	var endNodeAutoConfig *v1alpha1.EndNodeAutoConfig
	// fetch all configs in the namespace
	for _, item := range listEndNodeAutoConfig {
		fmt.Println("HEY ERROR")
		b.logger.Info(item.Spec.Configuration.Connection, "item.Spec.Configuration.Connection")
		b.logger.Info(item.Spec.Configuration.Protocol, "item.Spec.Configuration.Protocol")
		b.logger.Info(wirelessDevice.Connection, "wirelessDevice.Connection")
		b.logger.Info(wirelessDevice.Connection, "wirelessDevice.Protocol")
		if strings.ToLower(item.Spec.Configuration.Protocol) == strings.ToLower(wirelessDevice.Connection) || strings.ToLower(item.Spec.Configuration.Protocol) == strings.ToLower(wirelessDevice.Protocol) {
			// Found the matching EndNodeAutoConfig resource, return it.
			b.logger.Info("MatchedHERE")
			endNodeAutoConfig = item
			break
		}
	}

	if endNodeAutoConfig == nil {
		b.logger.Error("No EndNodeAutoConfig resources")
		return false, fmt.Errorf("%s", "No EndNodeAutoConfig resources")
	}
	b.logger.Infof("WE ARE IN THE FUNCTION 3")

	deviceConfigPlugins := []v1.Container{}
	for _, item := range endNodeAutoConfig.Spec.Configuration.DevicePlugin.Containers {
		container := v1.Container{
			Name:  item.Name,
			Image: item.Image,
		}
		deviceConfigPlugins = append(deviceConfigPlugins, container)
	}

	edgeConfigsWorkloads := []v1.Container{}
	for _, item := range endNodeAutoConfig.Spec.Configuration.WorkloadSpec.Containers {
		container := v1.Container{
			Name:  item.Name,
			Image: item.Image,
		}
		edgeConfigsWorkloads = append(edgeConfigsWorkloads, container)
	}

	edgeWorkloadDevicePlugin := &v1alpha1.EdgeWorkload{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateUniqueName("end-node-device-plugin-", device.Name),
			Namespace: device.Namespace,
		},
		Spec: v1alpha1.EdgeWorkloadSpec{
			Device: device.Name,
			Type:   v1alpha1.PodWorkloadType,
			Pod: v1alpha1.Pod{
				Spec: v1.PodSpec{
					Containers: append([]v1.Container{}, deviceConfigPlugins...),
				},
			},
		},
	}

	edgeWorkloadDeviceWorkloads := &v1alpha1.EdgeWorkload{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateUniqueName("end-node-device-workloads-", device.Name),
			Namespace: device.Namespace,
		},
		Spec: v1alpha1.EdgeWorkloadSpec{
			Device: device.Name,
			Type:   v1alpha1.PodWorkloadType,
			Pod: v1alpha1.Pod{
				Spec: v1.PodSpec{
					Containers: append([]v1.Container{}, edgeConfigsWorkloads...),
				},
			},
		},
	}

	pluginCreateError := b.repository.CreateEdgeWorkload(ctx, edgeWorkloadDevicePlugin)
	workloadCreateError := b.repository.CreateEdgeWorkload(ctx, edgeWorkloadDeviceWorkloads)
	if pluginCreateError != nil || workloadCreateError != nil {
		if pluginCreateError != nil {
			// logger.Errorf("Failed to create endNode Plugin ", pluginCreateError)
			return false, pluginCreateError
		}

		if workloadCreateError != nil {
			// logger.Errorf("Failed to create endNode Workloads ", workloadCreateError)
			return false, workloadCreateError
		}
	}

	return true, nil
}

// generateUniqueName generates a unique name using a unique identifier.
func generateUniqueName(name, device string) string {

	return fmt.Sprintf("%s-%s-%d", name, device, time.Now().UnixNano())
}
