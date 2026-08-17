package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	// GroupName is the Kubernetes API group containing Gereh runtime resources.
	GroupName = "runtime.gereh.ai"

	// Version is the current stable RuntimeCell API version.
	Version = "v1"
)

var (
	// SchemeGroupVersion identifies runtime.gereh.ai/v1.
	SchemeGroupVersion = schema.GroupVersion{
		Group:   GroupName,
		Version: Version,
	}

	// SchemeBuilder registers RuntimeCell API objects.
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)

	// AddToScheme adds RuntimeCell API objects to a Kubernetes runtime scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)

func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(
		SchemeGroupVersion,
		&RuntimeCell{},
		&RuntimeCellList{},
	)

	metav1.AddToGroupVersion(
		scheme,
		SchemeGroupVersion,
	)

	return nil
}
