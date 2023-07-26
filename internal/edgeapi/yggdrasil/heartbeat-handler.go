package yggdrasil

import (
	"context"
	"fmt"
	"time"

	"github.com/project-flotta/flotta-operator/internal/edgeapi/backend"
	"github.com/project-flotta/flotta-operator/models"
)

//go:generate mockgen -package=yggdrasil -destination=mock_status-updater.go . StatusUpdater
type StatusUpdater interface {
	UpdateStatus(ctx context.Context, name, namespace string, heartbeat *models.Heartbeat) (bool, error)
}

type RetryingDelegatingHandler struct {
	delegate StatusUpdater
}

func NewRetryingDelegatingHandler(delegate StatusUpdater) *RetryingDelegatingHandler {
	return &RetryingDelegatingHandler{delegate: delegate}
}

func (h *RetryingDelegatingHandler) Process(ctx context.Context, name, namespace string, heartbeat *models.Heartbeat) error {
	// retry patching the edge device status

	// fmt.Println(heartbeat.Hardware.WirelessDevices)
	fmt.Printf("%+v\n", heartbeat.Hardware.WirelessDevices)

	for _, item := range heartbeat.Hardware.WirelessDevices {
		fmt.Printf("Name: %s\n", item.Name)
		fmt.Printf("Manufacturer: %s\n", item.Manufacturer)
		fmt.Printf("Model: %s\n", item.Model)
		fmt.Printf("Software Version: %s\n", item.SwVersion)
		fmt.Printf("Identifiers: %s\n", item.Identifiers)
		fmt.Printf("Protocol: %s\n", item.Protocol)
		fmt.Printf("Connection: %s\n", item.Connection)
		fmt.Printf("Battery: %s\n", item.Battery)
		fmt.Printf("Availability: %s\n", item.Availability)
		fmt.Printf("Device Type: %s\n", item.DeviceType)
		fmt.Printf("Last Seen: %s\n", item.LastSeen)
		fmt.Println("--------")
	}

	var err error
	var retry bool
	for i := 1; i < 5; i++ {
		childCtx := context.WithValue(ctx, backend.RetryContextKey, retry)
		retry, err = h.delegate.UpdateStatus(childCtx, name, namespace, heartbeat)
		if err == nil {
			return nil
		}
		if !retry {
			break
		}

		time.Sleep(time.Duration(i*50) * time.Millisecond)
	}
	return err
}
