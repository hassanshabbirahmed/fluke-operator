/*
Copyright 2026.

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
	"k8s.io/apimachinery/pkg/runtime"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// FlukeSpec defines the desired state of Fluke. This is what a user writes;
// the controller never modifies it. See docs/notes/03-fluke-design.md for
// the design this was validated against.
type FlukeSpec struct {
	// replicas is how many copies of the workload to run.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=1
	Replicas int32 `json:"replicas"`

	// flukesPerReplica is how many fluke devices each replica claims via DRA.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=1
	FlukesPerReplica int32 `json:"flukesPerReplica"`

	// image is the container image each replica runs.
	// +kubebuilder:validation:MinLength=1
	Image string `json:"image"`
}

// FlukePhase summarises FlukeStatus for a human glancing at `kubectl get`.
// The controller derives it every reconcile from ReadyReplicas and
// AllocatedFlukes below — a user never sets it directly.
// +kubebuilder:validation:Enum=Pending;Progressing;Ready;Degraded
type FlukePhase string

const (
	FlukePending     FlukePhase = "Pending"
	FlukeProgressing FlukePhase = "Progressing"
	FlukeReady       FlukePhase = "Ready"
	FlukeDegraded    FlukePhase = "Degraded"
)

// FlukeStatus defines the observed state of Fluke.
type FlukeStatus struct {
	// phase is the controller's one-word summary of the two fields below.
	// +optional
	Phase FlukePhase `json:"phase,omitempty"`

	// readyReplicas is how many replicas the owned Deployment reports Ready.
	// +optional
	ReadyReplicas int32 `json:"readyReplicas"`

	// allocatedFlukes is how many fluke devices are currently bound to this
	// Fluke's replicas.
	// +optional
	AllocatedFlukes int32 `json:"allocatedFlukes"`

	// conditions represent the current state of the Fluke resource.
	// Each condition has a unique type and reflects the status of a specific aspect of the resource.
	//
	// Standard condition types include:
	// - "Available": the resource is fully functional
	// - "Progressing": the resource is being created or updated
	// - "Degraded": the resource failed to reach or maintain its desired state
	//
	// The status of each condition is one of True, False, or Unknown.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Replicas",type=integer,JSONPath=`.spec.replicas`
// +kubebuilder:printcolumn:name="Ready",type=integer,JSONPath=`.status.readyReplicas`
// +kubebuilder:printcolumn:name="Flukes",type=integer,JSONPath=`.status.allocatedFlukes`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Fluke is the Schema for the flukes API
type Fluke struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of Fluke
	// +required
	Spec FlukeSpec `json:"spec"`

	// status defines the observed state of Fluke
	// +optional
	Status FlukeStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// FlukeList contains a list of Fluke
type FlukeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []Fluke `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &Fluke{}, &FlukeList{})
		return nil
	})
}
