package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

const (
	// RuntimeCellFinalizer is added by the Phase-23 Runtime Cell Manager before
	// it creates external/runtime resources.
	RuntimeCellFinalizer = "runtime.gereh.ai/runtime-cell"

	// RuntimeCellConditionReady indicates whether the entire cell is ready.
	RuntimeCellConditionReady = "Ready"

	// RuntimeCellConditionInfrastructureReady indicates whether the Kubernetes
	// infrastructure required by the cell is ready.
	RuntimeCellConditionInfrastructureReady = "InfrastructureReady"

	// RuntimeCellConditionRuntimeReady indicates whether the OpenClaw/Hermes
	// runtime itself is healthy.
	RuntimeCellConditionRuntimeReady = "RuntimeReady"
)

// RuntimeKind identifies the runtime implementation hosted by the cell.
//
// +kubebuilder:validation:Enum=openclaw;hermes
type RuntimeKind string

const (
	// RuntimeKindOpenClaw selects an OpenClaw runtime cell.
	RuntimeKindOpenClaw RuntimeKind = "openclaw"

	// RuntimeKindHermes selects a Hermes runtime cell.
	RuntimeKindHermes RuntimeKind = "hermes"
)

// IsolationTier defines the security/resource isolation boundary.
//
// +kubebuilder:validation:Enum=standard;dedicated
type IsolationTier string

const (
	// IsolationTierStandard uses the normal tenant-isolated runtime topology.
	IsolationTierStandard IsolationTier = "standard"

	// IsolationTierDedicated requests a stronger dedicated runtime topology.
	IsolationTierDedicated IsolationTier = "dedicated"
)

// RuntimeCellPhase is a concise summary of reconciliation state.
//
// Conditions remain the authoritative detailed health representation.
//
// +kubebuilder:validation:Enum=Pending;Provisioning;Ready;Degraded;Failed;Deleting
type RuntimeCellPhase string

const (
	// RuntimeCellPhasePending means reconciliation has not completed.
	RuntimeCellPhasePending RuntimeCellPhase = "Pending"

	// RuntimeCellPhaseProvisioning means infrastructure/runtime creation is in progress.
	RuntimeCellPhaseProvisioning RuntimeCellPhase = "Provisioning"

	// RuntimeCellPhaseReady means the desired runtime cell is healthy.
	RuntimeCellPhaseReady RuntimeCellPhase = "Ready"

	// RuntimeCellPhaseDegraded means the cell exists but is not fully healthy.
	RuntimeCellPhaseDegraded RuntimeCellPhase = "Degraded"

	// RuntimeCellPhaseFailed means reconciliation reached a terminal failure.
	RuntimeCellPhaseFailed RuntimeCellPhase = "Failed"

	// RuntimeCellPhaseDeleting means cleanup is in progress.
	RuntimeCellPhaseDeleting RuntimeCellPhase = "Deleting"
)

// RuntimeCellSpec contains durable desired state.
//
// Infrastructure-specific implementation details such as VM size, cloud
// provider, node selectors, image pull credentials, Secrets, or provider API
// keys must never be added here.
type RuntimeCellSpec struct {
	// TenantID identifies the Gereh tenant that owns this runtime cell.
	//
	// It is opaque to Kubernetes. Runtime Manager is responsible for verifying
	// that the authenticated control-plane request is authorized for it.
	//
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=128
	// +kubebuilder:validation:Pattern=`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="tenantId is immutable"
	TenantID string `json:"tenantId"`

	// OperationID is the control-plane idempotency identity.
	//
	// Repeated EnsureTenantRuntime calls using the same operation ID must
	// converge on the same RuntimeCell.
	//
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=128
	// +kubebuilder:validation:Pattern=`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="operationId is immutable"
	OperationID string `json:"operationId"`

	// Runtime identifies which agent runtime this cell hosts.
	//
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="runtime is immutable"
	Runtime RuntimeKind `json:"runtime"`

	// RuntimeVersion is the approved logical runtime release.
	//
	// Runtime Manager maps this value to a digest-pinned image. It is mutable
	// because runtime upgrades reconcile the existing cell to a newer approved
	// version.
	//
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=128
	// +kubebuilder:validation:Pattern=`^[A-Za-z0-9][A-Za-z0-9._+-]{0,127}$`
	RuntimeVersion string `json:"version"`

	// IsolationTier selects the tenant isolation topology.
	//
	// A tier change creates a different security topology and therefore
	// requires a new RuntimeCell instead of mutating an existing cell.
	//
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="isolationTier is immutable"
	IsolationTier IsolationTier `json:"isolationTier"`

	// StorageClass is the Kubernetes StorageClass selected by trusted regional
	// placement policy.
	//
	// It is a Kubernetes abstraction, not an Arvan/RKE2/AWS-specific volume
	// implementation.
	//
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="storageClass is immutable"
	StorageClass string `json:"storageClass"`

	// Region is the Gereh logical execution region.
	//
	// Examples may include ir-thr-1 or another Gereh-defined placement region.
	// This is not a cloud-provider resource ID.
	//
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern=`^[a-z0-9][a-z0-9-]{0,62}$`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="region is immutable"
	Region string `json:"region"`
}

// RuntimeCellStatus contains controller-owned observed state.
type RuntimeCellStatus struct {
	// ObservedGeneration is the metadata.generation last successfully observed
	// by Runtime Manager.
	//
	// +kubebuilder:validation:Minimum=0
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Phase is a concise human-readable state summary.
	Phase RuntimeCellPhase `json:"phase,omitempty"`

	// ObservedRuntimeVersion is the runtime release currently running.
	//
	// +kubebuilder:validation:MaxLength=128
	ObservedRuntimeVersion string `json:"observedRuntimeVersion,omitempty"`

	// FailureCode is a safe machine-readable failure identifier.
	//
	// It must never contain credentials, provider responses, prompts, tool
	// output, URLs with secrets, or arbitrary stack traces.
	//
	// +kubebuilder:validation:MaxLength=128
	// +kubebuilder:validation:Pattern=`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`
	FailureCode string `json:"failureCode,omitempty"`

	// Conditions expose detailed Kubernetes-style readiness information.
	//
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// RuntimeCell represents desired and observed state for one tenant runtime.
//
// RuntimeCell objects are stored in the trusted runtime-system namespace.
// Tenant workloads themselves must not receive write permission to this API.
//
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=runtimecells,scope=Namespaced,shortName=rcell,categories=gereh
// +kubebuilder:printcolumn:name="Runtime",type=string,JSONPath=".spec.runtime"
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Tenant",type=string,JSONPath=".spec.tenantId",priority=1
// +kubebuilder:printcolumn:name="Region",type=string,JSONPath=".spec.region",priority=1
// +kubebuilder:printcolumn:name="Tier",type=string,JSONPath=".spec.isolationTier",priority=1
// +kubebuilder:printcolumn:name="Version",type=string,JSONPath=".spec.version",priority=1
// +kubebuilder:selectablefield:JSONPath=".spec.tenantId"
// +kubebuilder:selectablefield:JSONPath=".spec.operationId"
// +kubebuilder:selectablefield:JSONPath=".spec.runtime"
// +kubebuilder:selectablefield:JSONPath=".status.phase"
type RuntimeCell struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RuntimeCellSpec   `json:"spec"`
	Status RuntimeCellStatus `json:"status,omitempty"`
}

// RuntimeCellList contains a list of RuntimeCells.
//
// +kubebuilder:object:root=true
type RuntimeCellList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []RuntimeCell `json:"items"`
}
