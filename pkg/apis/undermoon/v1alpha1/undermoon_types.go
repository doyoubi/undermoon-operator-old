package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// UndermoonSpec defines the desired state of Undermoon
type UndermoonSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html

	// Each chunk has 2 masters and 2 replicas. This field is used to specify node number of the cluster.
	// +kubebuilder:validation:Minimum=1
	ChunkNumber uint32 `json:"chunkNumber"`
}

// UndermoonStatus defines the observed state of Undermoon
type UndermoonStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Undermoon is the Schema for the undermoons API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=undermoons,scope=Namespaced
type Undermoon struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   UndermoonSpec   `json:"spec,omitempty"`
	Status UndermoonStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// UndermoonList contains a list of Undermoon
type UndermoonList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Undermoon `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Undermoon{}, &UndermoonList{})
}
