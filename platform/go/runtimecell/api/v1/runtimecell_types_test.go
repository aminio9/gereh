package v1

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestAddToSchemeRegistersRuntimeCell(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme() error = %v", err)
	}

	kinds, unversioned, err := scheme.ObjectKinds(&RuntimeCell{})
	if err != nil {
		t.Fatalf("ObjectKinds() error = %v", err)
	}

	if unversioned {
		t.Fatal("RuntimeCell unexpectedly registered as unversioned")
	}

	want := SchemeGroupVersion.WithKind("RuntimeCell")
	found := false

	for _, kind := range kinds {
		if kind == want {
			found = true
			break
		}
	}

	if !found {
		t.Fatalf(
			"RuntimeCell GVK not registered; kinds = %v, want %v",
			kinds,
			want,
		)
	}
}

func TestRuntimeCellDeepCopyDoesNotAliasMutableState(t *testing.T) {
	t.Parallel()

	original := &RuntimeCell{
		ObjectMeta: metav1.ObjectMeta{
			Name: "example",
			Labels: map[string]string{
				"gereh.ai/test": "original",
			},
		},
		Status: RuntimeCellStatus{
			Conditions: []metav1.Condition{
				{
					Type:   RuntimeCellConditionReady,
					Status: metav1.ConditionFalse,
					Reason: "Pending",
				},
			},
		},
	}

	clone := original.DeepCopy()

	clone.Labels["gereh.ai/test"] = "changed"
	clone.Status.Conditions[0].Reason = "Changed"

	if original.Labels["gereh.ai/test"] != "original" {
		t.Fatal("DeepCopy() aliased ObjectMeta labels")
	}

	if original.Status.Conditions[0].Reason != "Pending" {
		t.Fatal("DeepCopy() aliased status conditions")
	}
}

func TestRuntimeKindValues(t *testing.T) {
	t.Parallel()

	tests := map[string]RuntimeKind{
		"openclaw": RuntimeKindOpenClaw,
		"hermes":   RuntimeKindHermes,
	}

	for want, value := range tests {
		if string(value) != want {
			t.Fatalf(
				"runtime kind %q serialized as %q",
				want,
				value,
			)
		}
	}
}
