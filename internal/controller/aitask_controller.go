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

package controller

import (
	"context"
	"fmt"
	"math/rand"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	scottyv1alpha1 "github.com/xtruder/scotty/api/v1alpha1"
)

// AITaskReconciler reconciles a AITask object
type AITaskReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=scotty.x-truder.net,resources=aitasks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=scotty.x-truder.net,resources=aitasks/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=scotty.x-truder.net,resources=aitasks/finalizers,verbs=update
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles;rolebindings;clusterroles;clusterrolebindings,verbs=get;list;watch;create;update;patch;delete;escalate;bind

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the AITask object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/reconcile
func (r *AITaskReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// Fetch the AITask resource
	var aiTask scottyv1alpha1.AITask
	if err := r.Get(ctx, req.NamespacedName, &aiTask); err != nil {
		if errors.IsNotFound(err) {
			// AITask was deleted, cleanup handled by garbage collection
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to fetch AITask")
		return ctrl.Result{}, err
	}

	// Initialize status if empty
	if aiTask.Status.Phase == "" {
		aiTask.Status.Phase = scottyv1alpha1.TaskPhasePending
		aiTask.Status.ExecutionCount = 0
		aiTask.Status.PrinterPrompt = r.truncatePrompt(aiTask.Spec.Prompt)
		if err := r.Status().Update(ctx, &aiTask); err != nil {
			log.Error(err, "Failed to update AITask status")
			return ctrl.Result{}, err
		}
	}

	// Ensure PrinterPrompt is up to date
	expectedPrinterPrompt := r.truncatePrompt(aiTask.Spec.Prompt)
	if aiTask.Status.PrinterPrompt != expectedPrinterPrompt {
		aiTask.Status.PrinterPrompt = expectedPrinterPrompt
		if err := r.Status().Update(ctx, &aiTask); err != nil {
			log.Error(err, "Failed to update AITask PrinterPrompt")
			return ctrl.Result{}, err
		}
	}

	// Handle deletion by cleaning up cluster resources
	finalizerName := "scotty.x-truder.net/cluster-rbac-cleanup"
	if aiTask.DeletionTimestamp != nil {
		if controllerutil.ContainsFinalizer(&aiTask, finalizerName) {
			log.Info("Cleaning up cluster RBAC resources")
			if err := r.cleanupClusterRBAC(ctx, &aiTask); err != nil {
				log.Error(err, "Failed to cleanup cluster RBAC resources")
				return ctrl.Result{}, err
			}
			controllerutil.RemoveFinalizer(&aiTask, finalizerName)
			if err := r.Update(ctx, &aiTask); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// Add finalizer if not present and we have cluster permissions
	if aiTask.Spec.Permissions != nil && len(aiTask.Spec.Permissions.ClusterRules) > 0 {
		if !controllerutil.ContainsFinalizer(&aiTask, finalizerName) {
			controllerutil.AddFinalizer(&aiTask, finalizerName)
			if err := r.Update(ctx, &aiTask); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
	}

	// Check if execution job exists
	if aiTask.Status.ExecutionJob != "" {
		var job batchv1.Job
		jobName := types.NamespacedName{
			Namespace: aiTask.Namespace,
			Name:      aiTask.Status.ExecutionJob,
		}

		err := r.Get(ctx, jobName, &job)
		if err != nil && !errors.IsNotFound(err) {
			log.Error(err, "Failed to fetch execution job")
			return ctrl.Result{}, err
		}

		if errors.IsNotFound(err) {
			// Job was deleted, reset status
			log.Info("Execution job not found, resetting status")
			aiTask.Status.ExecutionJob = ""
			aiTask.Status.Phase = scottyv1alpha1.TaskPhasePending
			if err := r.Status().Update(ctx, &aiTask); err != nil {
				return ctrl.Result{}, err
			}
			// No requeue needed since we watch Jobs
			return ctrl.Result{}, nil
		}

		// Monitor job status
		if job.Status.Active > 0 {
			if aiTask.Status.Phase != scottyv1alpha1.TaskPhaseRunning {
				log.Info("Job is running, updating task status")
				aiTask.Status.Phase = scottyv1alpha1.TaskPhaseRunning
				now := metav1.Now()
				aiTask.Status.StartTime = &now
				if err := r.Status().Update(ctx, &aiTask); err != nil {
					return ctrl.Result{}, err
				}
			}
			// No requeue needed since we watch Jobs
			return ctrl.Result{}, nil
		}

		if job.Status.Succeeded > 0 {
			log.Info("Job completed successfully")
			aiTask.Status.Phase = scottyv1alpha1.TaskPhaseCompleted
			now := metav1.Now()
			aiTask.Status.CompletionTime = &now
			aiTask.Status.LastExecution = &now
			aiTask.Status.ExecutionCount++
			if err := r.Status().Update(ctx, &aiTask); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}

		if job.Status.Failed > 0 {
			log.Info("Job failed")
			aiTask.Status.Phase = scottyv1alpha1.TaskPhaseFailed
			now := metav1.Now()
			aiTask.Status.CompletionTime = &now
			aiTask.Status.LastExecution = &now
			aiTask.Status.ExecutionCount++
			if err := r.Status().Update(ctx, &aiTask); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}

		// Job is still pending - no requeue needed since we watch Jobs
		return ctrl.Result{}, nil
	}

	// No execution job exists and task is pending, create one
	if aiTask.Status.Phase == scottyv1alpha1.TaskPhasePending {
		log.Info("Creating RBAC resources for AITask")
		serviceAccount, err := r.createServiceAccount(ctx, &aiTask)
		if err != nil {
			log.Error(err, "Failed to create ServiceAccount")
			return ctrl.Result{}, err
		}

		if err := r.createRBACResources(ctx, &aiTask, serviceAccount); err != nil {
			log.Error(err, "Failed to create RBAC resources")
			return ctrl.Result{}, err
		}

		log.Info("Creating execution job for AITask")
		job, err := r.createExecutionJob(ctx, &aiTask, serviceAccount)
		if err != nil {
			log.Error(err, "Failed to create execution job")
			return ctrl.Result{}, err
		}

		aiTask.Status.ExecutionJob = job.Name
		if err := r.Status().Update(ctx, &aiTask); err != nil {
			log.Error(err, "Failed to update AITask status")
			return ctrl.Result{}, err
		}
	}

	// No requeue needed since we watch owned resources
	return ctrl.Result{}, nil
}

// generateRandomSuffix generates a random alphanumeric suffix
func generateRandomSuffix(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

// createServiceAccount creates a dedicated ServiceAccount for the AITask
func (r *AITaskReconciler) createServiceAccount(ctx context.Context, aiTask *scottyv1alpha1.AITask) (*corev1.ServiceAccount, error) {
	serviceAccountName := fmt.Sprintf("aitask-%s", aiTask.Name)

	// Check if ServiceAccount already exists
	var existingSA corev1.ServiceAccount
	err := r.Get(ctx, types.NamespacedName{Namespace: aiTask.Namespace, Name: serviceAccountName}, &existingSA)
	if err == nil {
		return &existingSA, nil
	}
	if !errors.IsNotFound(err) {
		return nil, fmt.Errorf("failed to check existing ServiceAccount: %w", err)
	}

	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceAccountName,
			Namespace: aiTask.Namespace,
			Labels: map[string]string{
				"app":                      "scotty",
				"scotty.x-truder.net/task": aiTask.Name,
			},
		},
	}

	// Set owner reference for garbage collection
	if err := controllerutil.SetControllerReference(aiTask, serviceAccount, r.Scheme); err != nil {
		return nil, fmt.Errorf("failed to set owner reference: %w", err)
	}

	if err := r.Create(ctx, serviceAccount); err != nil {
		return nil, fmt.Errorf("failed to create ServiceAccount: %w", err)
	}

	return serviceAccount, nil
}

// createRBACResources creates Role/ClusterRole and RoleBinding/ClusterRoleBinding for the AITask
func (r *AITaskReconciler) createRBACResources(ctx context.Context, aiTask *scottyv1alpha1.AITask, serviceAccount *corev1.ServiceAccount) error {
	if aiTask.Spec.Permissions == nil {
		return nil // No permissions specified
	}

	// Create namespaced RBAC resources
	if len(aiTask.Spec.Permissions.NamespacedRules) > 0 {
		if err := r.createNamespacedRBAC(ctx, aiTask, serviceAccount); err != nil {
			return fmt.Errorf("failed to create namespaced RBAC: %w", err)
		}
	}

	// Create cluster-wide RBAC resources
	if len(aiTask.Spec.Permissions.ClusterRules) > 0 {
		if err := r.createClusterRBAC(ctx, aiTask, serviceAccount); err != nil {
			return fmt.Errorf("failed to create cluster RBAC: %w", err)
		}
	}

	return nil
}

// createNamespacedRBAC creates Role and RoleBinding for namespaced permissions
func (r *AITaskReconciler) createNamespacedRBAC(ctx context.Context, aiTask *scottyv1alpha1.AITask, serviceAccount *corev1.ServiceAccount) error {
	roleName := fmt.Sprintf("aitask-%s", aiTask.Name)

	// Create Role
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      roleName,
			Namespace: aiTask.Namespace,
			Labels: map[string]string{
				"app":                      "scotty",
				"scotty.x-truder.net/task": aiTask.Name,
			},
		},
		Rules: aiTask.Spec.Permissions.NamespacedRules,
	}

	if err := controllerutil.SetControllerReference(aiTask, role, r.Scheme); err != nil {
		return fmt.Errorf("failed to set owner reference on Role: %w", err)
	}

	if err := r.Create(ctx, role); err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create Role: %w", err)
	}

	// Create RoleBinding
	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      roleName,
			Namespace: aiTask.Namespace,
			Labels: map[string]string{
				"app":                      "scotty",
				"scotty.x-truder.net/task": aiTask.Name,
			},
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      serviceAccount.Name,
				Namespace: serviceAccount.Namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "Role",
			Name:     roleName,
			APIGroup: "rbac.authorization.k8s.io",
		},
	}

	if err := controllerutil.SetControllerReference(aiTask, roleBinding, r.Scheme); err != nil {
		return fmt.Errorf("failed to set owner reference on RoleBinding: %w", err)
	}

	if err := r.Create(ctx, roleBinding); err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create RoleBinding: %w", err)
	}

	return nil
}

// createClusterRBAC creates ClusterRole and ClusterRoleBinding for cluster-wide permissions
func (r *AITaskReconciler) createClusterRBAC(ctx context.Context, aiTask *scottyv1alpha1.AITask, serviceAccount *corev1.ServiceAccount) error {
	clusterRoleName := fmt.Sprintf("aitask-%s-%s", aiTask.Namespace, aiTask.Name)

	// Create ClusterRole
	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterRoleName,
			Labels: map[string]string{
				"app":                           "scotty",
				"scotty.x-truder.net/task":      aiTask.Name,
				"scotty.x-truder.net/namespace": aiTask.Namespace,
			},
		},
		Rules: aiTask.Spec.Permissions.ClusterRules,
	}

	// Note: Cannot set owner reference on cluster-scoped resources from namespace-scoped owners
	// Instead, we'll use labels to track ownership and handle cleanup manually

	if err := r.Create(ctx, clusterRole); err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create ClusterRole: %w", err)
	}

	// Create ClusterRoleBinding
	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterRoleName,
			Labels: map[string]string{
				"app":                           "scotty",
				"scotty.x-truder.net/task":      aiTask.Name,
				"scotty.x-truder.net/namespace": aiTask.Namespace,
			},
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      serviceAccount.Name,
				Namespace: serviceAccount.Namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "ClusterRole",
			Name:     clusterRoleName,
			APIGroup: "rbac.authorization.k8s.io",
		},
	}

	// Note: Cannot set owner reference on cluster-scoped resources from namespace-scoped owners
	// Instead, we'll use labels to track ownership and handle cleanup manually

	if err := r.Create(ctx, clusterRoleBinding); err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create ClusterRoleBinding: %w", err)
	}

	return nil
}

// cleanupClusterRBAC removes cluster-scoped RBAC resources
func (r *AITaskReconciler) cleanupClusterRBAC(ctx context.Context, aiTask *scottyv1alpha1.AITask) error {
	clusterRoleName := fmt.Sprintf("aitask-%s-%s", aiTask.Namespace, aiTask.Name)

	// Delete ClusterRole
	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterRoleName,
		},
	}
	if err := r.Delete(ctx, clusterRole); err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete ClusterRole: %w", err)
	}

	// Delete ClusterRoleBinding
	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterRoleName,
		},
	}
	if err := r.Delete(ctx, clusterRoleBinding); err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete ClusterRoleBinding: %w", err)
	}

	return nil
}

// createExecutionJob creates a Kubernetes Job to run the AI agent
func (r *AITaskReconciler) createExecutionJob(ctx context.Context, aiTask *scottyv1alpha1.AITask, serviceAccount *corev1.ServiceAccount) (*batchv1.Job, error) {
	// Generate unique job name including agent name and random suffix
	randomSuffix := generateRandomSuffix(8)
	jobName := fmt.Sprintf("aitask-%s-%s-%s", aiTask.Name, string(aiTask.Spec.Agent), randomSuffix)

	// Determine agent image
	var image string
	switch aiTask.Spec.Agent {
	case scottyv1alpha1.AgentTypeClaudeCode:
		image = "xtruder/scotty-claudecode:latest"
	case scottyv1alpha1.AgentTypeOpenCode:
		image = "xtruder/scotty-opencode:latest"
	default:
		return nil, fmt.Errorf("unsupported agent type: %s", aiTask.Spec.Agent)
	}

	// Parse timeout (default to 15 minutes if not specified)
	timeoutSeconds := int64(900) // 15 minutes
	if aiTask.Spec.Timeout != nil {
		timeoutSeconds = int64(aiTask.Spec.Timeout.Duration.Seconds())
	}

	// Configure backoff limit based on retry policy
	var backoffLimit int32
	switch aiTask.Spec.RetryPolicy {
	case scottyv1alpha1.RetryPolicyNever:
		backoffLimit = 0
	case scottyv1alpha1.RetryPolicyOnFailure, "":
		backoffLimit = 3
	case scottyv1alpha1.RetryPolicyAlways:
		backoffLimit = 6
	default:
		backoffLimit = 3
	}

	// Set ImagePullPolicy (default to IfNotPresent if not specified)
	imagePullPolicy := corev1.PullIfNotPresent
	if aiTask.Spec.ImagePullPolicy != "" {
		imagePullPolicy = aiTask.Spec.ImagePullPolicy
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: aiTask.Namespace,
			Labels: map[string]string{
				"app":                       "scotty",
				"scotty.x-truder.net/task":  aiTask.Name,
				"scotty.x-truder.net/agent": string(aiTask.Spec.Agent),
			},
		},
		Spec: batchv1.JobSpec{
			ActiveDeadlineSeconds: &timeoutSeconds,
			BackoffLimit:          &backoffLimit,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":                       "scotty",
						"scotty.x-truder.net/task":  aiTask.Name,
						"scotty.x-truder.net/agent": string(aiTask.Spec.Agent),
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: serviceAccount.Name,
					RestartPolicy:      corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:            "agent",
							Image:           image,
							ImagePullPolicy: imagePullPolicy,
							Args:            r.buildClaudeArgs(aiTask),
							Env:             r.buildContainerEnv(aiTask),
							SecurityContext: &corev1.SecurityContext{
								RunAsNonRoot:             &[]bool{true}[0],
								AllowPrivilegeEscalation: &[]bool{false}[0],
								Capabilities: &corev1.Capabilities{
									Drop: []corev1.Capability{"ALL"},
								},
								SeccompProfile: &corev1.SeccompProfile{
									Type: corev1.SeccompProfileTypeRuntimeDefault,
								},
							},
						},
					},
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: &[]bool{true}[0],
						RunAsUser:    &[]int64{1000}[0],
						RunAsGroup:   &[]int64{1000}[0],
					},
				},
			},
		},
	}

	// Set owner reference for garbage collection
	if err := controllerutil.SetControllerReference(aiTask, job, r.Scheme); err != nil {
		return nil, fmt.Errorf("failed to set owner reference: %w", err)
	}

	// Create the job
	if err := r.Create(ctx, job); err != nil {
		return nil, fmt.Errorf("failed to create job: %w", err)
	}

	return job, nil
}

// buildContainerEnv builds environment variables for the AI agent container
func (r *AITaskReconciler) buildContainerEnv(aiTask *scottyv1alpha1.AITask) []corev1.EnvVar {
	env := []corev1.EnvVar{
		{
			Name:  "AGENT_TYPE",
			Value: string(aiTask.Spec.Agent),
		},
		{
			Name:  "KUBERNETES_NAMESPACE",
			Value: aiTask.Namespace,
		},
	}

	// Add API key from secret if specified
	if aiTask.Spec.APIKeySecretRef != nil {
		apiKeyEnv := corev1.EnvVar{
			Name: "ANTHROPIC_API_KEY",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: aiTask.Spec.APIKeySecretRef,
			},
		}
		env = append(env, apiKeyEnv)
	}

	return env
}

// buildClaudeArgs builds command line arguments for Claude Code agent
func (r *AITaskReconciler) buildClaudeArgs(aiTask *scottyv1alpha1.AITask) []string {
	// Build RBAC permissions info
	rbacInfo := "RBAC Permissions:\n"

	// Add namespaced permissions
	if aiTask.Spec.Permissions.NamespacedRules != nil {
		rbacInfo += "Namespace " + aiTask.Namespace + ":\n"
		for _, rule := range aiTask.Spec.Permissions.NamespacedRules {
			rbacInfo += fmt.Sprintf("- %v on %v (verbs: %v)\n", rule.APIGroups, rule.Resources, rule.Verbs)
		}
	}

	// Add cluster permissions
	if aiTask.Spec.Permissions.ClusterRules != nil {
		rbacInfo += "Cluster-wide:\n"
		for _, rule := range aiTask.Spec.Permissions.ClusterRules {
			rbacInfo += fmt.Sprintf("- %v on %v (verbs: %v)\n", rule.APIGroups, rule.Resources, rule.Verbs)
		}
	}

	systemPrompt := fmt.Sprintf(`You are an AI agent running as a Kubernetes service operator with goal to complete the task

- Use kubectl commands to inspect and modify cluster resources.
- You SHOULD NOT perform any unnecessary actions, do only what is required for task completion.
- You MUST take into account RBAC permissions given to the agent.
- Consider cluster security, resource constraints, and operational best practices.

Environment:
- Namespace: %s
- Agent: %s

%s
`,
		aiTask.Namespace, string(aiTask.Spec.Agent), rbacInfo)

	args := []string{
		"-p", aiTask.Spec.Prompt,
		"--append-system-prompt", systemPrompt,
		"--output-format", "stream-json",
		"--verbose",
		"--permission-mode", "bypassPermissions",
	}

	return args
}

// truncatePrompt truncates the prompt to 30 characters and appends "..." if needed
func (r *AITaskReconciler) truncatePrompt(prompt string) string {
	if len(prompt) <= 30 {
		return prompt
	}
	return prompt[:30] + "..."
}

// SetupWithManager sets up the controller with the Manager.
func (r *AITaskReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&scottyv1alpha1.AITask{}).
		Owns(&batchv1.Job{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&rbacv1.Role{}).
		Owns(&rbacv1.RoleBinding{}).
		Owns(&rbacv1.ClusterRole{}).
		Owns(&rbacv1.ClusterRoleBinding{}).
		Named("aitask").
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 1, // Prevent concurrent reconciliations causing race conditions
		}).
		Complete(r)
}
