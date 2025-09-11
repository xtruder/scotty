/*
Copyright 2025.

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
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EventType represents the type of Kubernetes event that can trigger AITask execution
// +kubebuilder:validation:Enum=pod-created;pod-failed;pod-oom-killed;deployment-scaled;node-not-ready;persistent-volume-full;service-endpoint-unavailable
type EventType string

const (
	EventTypePodCreated                 EventType = "pod-created"
	EventTypePodFailed                  EventType = "pod-failed"
	EventTypePodOOMKilled               EventType = "pod-oom-killed"
	EventTypeDeploymentScaled           EventType = "deployment-scaled"
	EventTypeNodeNotReady               EventType = "node-not-ready"
	EventTypePersistentVolumeFull       EventType = "persistent-volume-full"
	EventTypeServiceEndpointUnavailable EventType = "service-endpoint-unavailable"
)

// AgentType represents the AI agent to use for task execution
// +kubebuilder:validation:Enum=claudecode;opencode
type AgentType string

const (
	AgentTypeClaudeCode AgentType = "claudecode"
	AgentTypeOpenCode   AgentType = "opencode"
)

// RetryPolicy defines how to handle failed task executions
// +kubebuilder:validation:Enum=Never;OnFailure;Always
type RetryPolicy string

const (
	RetryPolicyNever     RetryPolicy = "Never"
	RetryPolicyOnFailure RetryPolicy = "OnFailure"
	RetryPolicyAlways    RetryPolicy = "Always"
)

// TaskPhase represents the current execution phase of an AITask
// +kubebuilder:validation:Enum=Pending;Running;Completed;Failed
type TaskPhase string

const (
	TaskPhasePending   TaskPhase = "Pending"
	TaskPhaseRunning   TaskPhase = "Running"
	TaskPhaseCompleted TaskPhase = "Completed"
	TaskPhaseFailed    TaskPhase = "Failed"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// AITaskTrigger defines event-based triggers for AITask execution
type AITaskTrigger struct {
	// EventType specifies the type of Kubernetes event that triggers execution
	EventType EventType `json:"eventType"`

	// Selector specifies which resources to watch for events
	// +optional
	Selector *metav1.LabelSelector `json:"selector,omitempty"`

	// Namespaces limits the trigger to specific namespaces
	// +optional
	Namespaces []string `json:"namespaces,omitempty"`

	// Debounce prevents trigger spam by setting a minimum interval between executions
	// +optional
	// +kubebuilder:default="5m"
	Debounce *metav1.Duration `json:"debounce,omitempty"`
}

// AITaskPermissions defines RBAC permissions for the AI agent
type AITaskPermissions struct {
	// NamespacedRules defines namespaced RBAC permissions (creates Role + RoleBinding)
	// +optional
	NamespacedRules []rbacv1.PolicyRule `json:"namespacedRules,omitempty"`

	// ClusterRules defines cluster-wide RBAC permissions (creates ClusterRole + ClusterRoleBinding)
	// +optional
	ClusterRules []rbacv1.PolicyRule `json:"clusterRules,omitempty"`
}

// AITaskSpec defines the desired state of AITask
type AITaskSpec struct {
	// Agent specifies which AI agent to use (claudecode, opencode)
	// +kubebuilder:validation:Required
	Agent AgentType `json:"agent"`

	// Prompt contains the task description or prompt for the AI agent
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Prompt string `json:"prompt"`

	// Triggers define the events that will execute this AITask
	// +optional
	Triggers []AITaskTrigger `json:"triggers,omitempty"`

	// Permissions specify the Kubernetes RBAC permissions for the AI agent
	// +optional
	Permissions *AITaskPermissions `json:"permissions,omitempty"`

	// APIKeySecretRef specifies the secret containing the AI agent API key
	// +optional
	APIKeySecretRef *corev1.SecretKeySelector `json:"apiKeySecretRef,omitempty"`

	// Timeout specifies the maximum execution time for the task
	// +optional
	// +kubebuilder:default="15m"
	Timeout *metav1.Duration `json:"timeout,omitempty"`

	// RetryPolicy defines how to handle failed executions
	// +optional
	// +kubebuilder:default="OnFailure"
	RetryPolicy RetryPolicy `json:"retryPolicy,omitempty"`
}

// AITaskTriggerInfo contains information about what triggered the execution
type AITaskTriggerInfo struct {
	// EventType is the type of event that triggered execution
	EventType EventType `json:"eventType"`

	// Resource is the name of the resource that triggered the event
	Resource string `json:"resource"`

	// Namespace is the namespace of the triggering resource
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// Timestamp when the trigger occurred
	Timestamp metav1.Time `json:"timestamp"`
}

// AITaskStatus defines the observed state of AITask.
type AITaskStatus struct {
	// Phase indicates the current execution phase
	// +optional
	Phase TaskPhase `json:"phase,omitempty"`

	// TriggeredBy contains information about what triggered the current execution
	// +optional
	TriggeredBy *AITaskTriggerInfo `json:"triggeredBy,omitempty"`

	// ExecutionJob is the name of the job running the AI agent
	// +optional
	ExecutionJob string `json:"executionJob,omitempty"`

	// StartTime when the current execution started
	// +optional
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// CompletionTime when the execution completed
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`

	// Result contains the output from the AI agent execution
	// +optional
	Result string `json:"result,omitempty"`

	// LastExecution timestamp of the most recent execution
	// +optional
	LastExecution *metav1.Time `json:"lastExecution,omitempty"`

	// ExecutionCount tracks the total number of executions
	// +optional
	ExecutionCount int32 `json:"executionCount,omitempty"`

	// conditions represent the current state of the AITask resource.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// AITask is the Schema for the aitasks API
type AITask struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// spec defines the desired state of AITask
	// +required
	Spec AITaskSpec `json:"spec"`

	// status defines the observed state of AITask
	// +optional
	Status AITaskStatus `json:"status,omitempty,omitzero"`
}

// +kubebuilder:object:root=true

// AITaskList contains a list of AITask
type AITaskList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AITask `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AITask{}, &AITaskList{})
}
