// Code generated by go-swagger; DO NOT EDIT.

package models

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"context"

	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/swag"
)

// DbWirelessDevice db wireless device
//
// swagger:model db_wireless_device
type DbWirelessDevice struct {

	// unique identifier of the unregistered device
	WirelessDeviceIdentifier string `json:"wireless_device_identifier,omitempty"`

	// name of the unregistered device
	WirelessDeviceName string `json:"wireless_device_name,omitempty"`
}

// Validate validates this db wireless device
func (m *DbWirelessDevice) Validate(formats strfmt.Registry) error {
	return nil
}

// ContextValidate validates this db wireless device based on context it is used
func (m *DbWirelessDevice) ContextValidate(ctx context.Context, formats strfmt.Registry) error {
	return nil
}

// MarshalBinary interface implementation
func (m *DbWirelessDevice) MarshalBinary() ([]byte, error) {
	if m == nil {
		return nil, nil
	}
	return swag.WriteJSON(m)
}

// UnmarshalBinary interface implementation
func (m *DbWirelessDevice) UnmarshalBinary(b []byte) error {
	var res DbWirelessDevice
	if err := swag.ReadJSON(b, &res); err != nil {
		return err
	}
	*m = res
	return nil
}
