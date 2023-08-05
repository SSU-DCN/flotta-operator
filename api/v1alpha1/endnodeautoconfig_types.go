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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// EndNodeAutoConfigSpec defines the desired state of EndNodeAutoConfig
type EndNodeAutoConfigSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// DeviceSelector *metav1.LabelSelector `json:"deviceSelector,omitempty"`
	Device string `json:"device"`

	Configuration Configuration `json:"configuration"`
}

type Configuration struct {
	Protocol     string       `json:"protocol,omitempty"`
	Connection   string       `json:"connection,omitempty"`
	DevicePlugin DevicePlugin `json:"devicePlugin,omitempty"`
	WorkloadSpec WorkloadSpec `json:"workloadSpec,omitempty"`
}

// EndNodeAutoConfigStatus defines the observed state of EndNodeAutoConfig
type EndNodeAutoConfigStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	EndNodeAutoConfigCondition []EndNodeAutoConfigCondition `json:"condition,omitempty"`
}

type EndNodeAutoConfigCondition struct {
	// The absence of a condition should be interpreted the same as Unknown
	Status metav1.ConditionStatus `json:"status" description:"status of the condition, one of True, False, Unknown"`

	// +optional
	Message *string `json:"message,omitempty" description:"human-readable message indicating details about last transition"`
	// +optional
	LastTransitionTime *metav1.Time `json:"lastTransitionTime,omitempty" description:"last time the condition transit from one status to another"`
}

type DevicePlugin struct {
	Containers []Containers `json:"containers,omitempty"`
}

type WorkloadSpec struct {
	Containers []Containers `json:"containers,omitempty"`
}
type Containers struct {
	Name  string `json:"name,omitempty"`
	Image string `json:"image,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// EndNodeAutoConfig is the Schema for the endnodeautoconfigs API
type EndNodeAutoConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   EndNodeAutoConfigSpec   `json:"spec,omitempty"`
	Status EndNodeAutoConfigStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// EndNodeAutoConfigList contains a list of EndNodeAutoConfig
type EndNodeAutoConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []EndNodeAutoConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&EndNodeAutoConfig{}, &EndNodeAutoConfigList{})
}
