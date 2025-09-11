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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	scottyv1alpha1 "github.com/xtruder/scotty/api/v1alpha1"
)

var _ = Describe("AITask Controller", func() {
	Context("When reconciling an AITask resource", func() {
		const (
			taskName      = "test-aitask"
			taskNamespace = "default"
			timeout       = time.Second * 10
			interval      = time.Millisecond * 250
		)

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      taskName,
			Namespace: taskNamespace,
		}

		BeforeEach(func() {
			By("Creating the custom resource for AITask")
			aiTask := &scottyv1alpha1.AITask{
				ObjectMeta: metav1.ObjectMeta{
					Name:      taskName,
					Namespace: taskNamespace,
				},
				Spec: scottyv1alpha1.AITaskSpec{
					Agent:  scottyv1alpha1.AgentTypeClaudeCode,
					Prompt: "Analyze the cluster and provide recommendations",
					Timeout: &metav1.Duration{
						Duration: 15 * time.Minute,
					},
					RetryPolicy: scottyv1alpha1.RetryPolicyOnFailure,
					Permissions: &scottyv1alpha1.AITaskPermissions{
						NamespacedRules: []rbacv1.PolicyRule{
							{
								APIGroups: []string{""},
								Resources: []string{"pods"},
								Verbs:     []string{"get", "list"},
							},
						},
						// No cluster rules to avoid finalizer issues in tests
					},
					APIKeySecretRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "anthropic-api-key",
						},
						Key: "api-key",
					},
				},
			}
			Expect(k8sClient.Create(ctx, aiTask)).To(Succeed())
		})

		AfterEach(func() {
			By("Cleanup the AITask resource")
			aiTask := &scottyv1alpha1.AITask{}
			err := k8sClient.Get(ctx, typeNamespacedName, aiTask)
			if err == nil {
				Expect(k8sClient.Delete(ctx, aiTask)).To(Succeed())
				// Wait for deletion to complete
				Eventually(func() error {
					return k8sClient.Get(ctx, typeNamespacedName, aiTask)
				}, timeout, interval).Should(HaveOccurred())
			}

			By("Cleanup any remaining resources")

			// Cleanup jobs
			jobList := &batchv1.JobList{}
			err = k8sClient.List(ctx, jobList)
			if err == nil {
				for _, job := range jobList.Items {
					if job.Labels != nil && job.Labels["scotty.x-truder.net/task"] == taskName {
						Expect(k8sClient.Delete(ctx, &job)).To(Succeed())
					}
				}
			}

			// Cleanup ServiceAccounts
			saList := &corev1.ServiceAccountList{}
			err = k8sClient.List(ctx, saList)
			if err == nil {
				for _, sa := range saList.Items {
					if sa.Labels != nil && sa.Labels["scotty.x-truder.net/task"] == taskName {
						Expect(k8sClient.Delete(ctx, &sa)).To(Succeed())
					}
				}
			}

			// Cleanup Roles
			roleList := &rbacv1.RoleList{}
			err = k8sClient.List(ctx, roleList)
			if err == nil {
				for _, role := range roleList.Items {
					if role.Labels != nil && role.Labels["scotty.x-truder.net/task"] == taskName {
						Expect(k8sClient.Delete(ctx, &role)).To(Succeed())
					}
				}
			}

			// Cleanup RoleBindings
			roleBindingList := &rbacv1.RoleBindingList{}
			err = k8sClient.List(ctx, roleBindingList)
			if err == nil {
				for _, rb := range roleBindingList.Items {
					if rb.Labels != nil && rb.Labels["scotty.x-truder.net/task"] == taskName {
						Expect(k8sClient.Delete(ctx, &rb)).To(Succeed())
					}
				}
			}
		})

		It("should initialize status to Pending on first reconcile", func() {
			By("Reconciling the created resource")
			controllerReconciler := &AITaskReconciler{
				Client:                 k8sClient,
				Scheme:                 k8sClient.Scheme(),
				DefaultClaudeCodeImage: "ghcr.io/xtruder/scotty/claudecode:latest",
				DefaultOpenCodeImage:   "xtruder/scotty-opencode:latest",
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Checking that the AITask status is initialized")
			aiTask := &scottyv1alpha1.AITask{}
			Eventually(func() scottyv1alpha1.TaskPhase {
				err := k8sClient.Get(ctx, typeNamespacedName, aiTask)
				if err != nil {
					return ""
				}
				return aiTask.Status.Phase
			}, timeout, interval).Should(Equal(scottyv1alpha1.TaskPhasePending))

			By("Verifying execution count is initialized")
			Expect(aiTask.Status.ExecutionCount).To(Equal(int32(0)))

			By("Verifying PrinterPrompt is set correctly")
			Expect(aiTask.Status.PrinterPrompt).To(Equal("Analyze the cluster and provid..."))
		})

		It("should create a Job when AITask is in Pending phase", func() {
			By("Reconciling the created resource")
			controllerReconciler := &AITaskReconciler{
				Client:                 k8sClient,
				Scheme:                 k8sClient.Scheme(),
				DefaultClaudeCodeImage: "ghcr.io/xtruder/scotty/claudecode:latest",
				DefaultOpenCodeImage:   "xtruder/scotty-opencode:latest",
			}

			// First reconcile initializes status
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			// Second reconcile should create the job
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Checking that a Job was created")
			Eventually(func() bool {
				jobList := &batchv1.JobList{}
				err := k8sClient.List(ctx, jobList)
				if err != nil {
					return false
				}
				for _, job := range jobList.Items {
					if job.Labels != nil &&
						job.Labels["scotty.x-truder.net/task"] == taskName &&
						job.Labels["scotty.x-truder.net/agent"] == "claudecode" {
						return true
					}
				}
				return false
			}, timeout, interval).Should(BeTrue())

			By("Verifying the AITask status contains the job name")
			aiTask := &scottyv1alpha1.AITask{}
			Eventually(func() string {
				err := k8sClient.Get(ctx, typeNamespacedName, aiTask)
				if err != nil {
					return ""
				}
				return aiTask.Status.ExecutionJob
			}, timeout, interval).ShouldNot(BeEmpty())

			By("Verifying the job name includes the agent name")
			Expect(aiTask.Status.ExecutionJob).To(ContainSubstring("claudecode"))
		})

		It("should create RBAC resources for AITask permissions", func() {
			By("Creating and reconciling AITask")
			controllerReconciler := &AITaskReconciler{
				Client:                 k8sClient,
				Scheme:                 k8sClient.Scheme(),
				DefaultClaudeCodeImage: "ghcr.io/xtruder/scotty/claudecode:latest",
				DefaultOpenCodeImage:   "xtruder/scotty-opencode:latest",
			}

			// Initialize and create RBAC resources
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying ServiceAccount was created")
			serviceAccountName := fmt.Sprintf("aitask-%s", taskName)
			var serviceAccount corev1.ServiceAccount
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Namespace: taskNamespace, Name: serviceAccountName}, &serviceAccount)
			}, timeout, interval).Should(Succeed())

			By("Verifying Role was created for namespaced permissions")
			var role rbacv1.Role
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Namespace: taskNamespace, Name: serviceAccountName}, &role)
			}, timeout, interval).Should(Succeed())
			Expect(role.Rules).To(HaveLen(1))
			Expect(role.Rules[0].Resources).To(ContainElement("pods"))

			By("Verifying RoleBinding was created")
			var roleBinding rbacv1.RoleBinding
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Namespace: taskNamespace, Name: serviceAccountName}, &roleBinding)
			}, timeout, interval).Should(Succeed())
			Expect(roleBinding.Subjects).To(HaveLen(1))
			Expect(roleBinding.Subjects[0].Name).To(Equal(serviceAccountName))
		})

		It("should validate Job configuration matches AITask spec", func() {
			By("Creating and reconciling AITask")
			controllerReconciler := &AITaskReconciler{
				Client:                 k8sClient,
				Scheme:                 k8sClient.Scheme(),
				DefaultClaudeCodeImage: "ghcr.io/xtruder/scotty/claudecode:latest",
				DefaultOpenCodeImage:   "xtruder/scotty-opencode:latest",
			}

			// Initialize and create job
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Finding the created Job")
			var createdJob *batchv1.Job
			Eventually(func() bool {
				jobList := &batchv1.JobList{}
				err := k8sClient.List(ctx, jobList)
				if err != nil {
					return false
				}
				for i := range jobList.Items {
					job := &jobList.Items[i]
					if job.Labels != nil && job.Labels["scotty.x-truder.net/task"] == taskName {
						createdJob = job
						return true
					}
				}
				return false
			}, timeout, interval).Should(BeTrue())

			By("Verifying Job configuration")
			Expect(createdJob).NotTo(BeNil())
			Expect(*createdJob.Spec.BackoffLimit).To(Equal(int32(3)))            // OnFailure retry policy
			Expect(*createdJob.Spec.ActiveDeadlineSeconds).To(Equal(int64(900))) // 15 minutes

			By("Verifying Pod template configuration")
			podSpec := createdJob.Spec.Template.Spec
			Expect(podSpec.Containers).To(HaveLen(1))

			container := podSpec.Containers[0]
			Expect(container.Name).To(Equal("agent"))
			Expect(container.Image).To(Equal("ghcr.io/xtruder/scotty/claudecode:latest"))

			By("Verifying command arguments")
			args := container.Args
			Expect(args).To(ContainElement("-p"))
			Expect(args).To(ContainElement("Analyze the cluster and provide recommendations"))
			Expect(args).To(ContainElement("--output-format"))
			Expect(args).To(ContainElement("stream-json"))
			Expect(args).To(ContainElement("--verbose"))
			Expect(args).To(ContainElement("--permission-mode"))
			Expect(args).To(ContainElement("bypassPermissions"))
			Expect(args).To(ContainElement("--append-system-prompt"))

			By("Verifying environment variables")
			envMap := make(map[string]string)
			var anthropicKeyEnv *corev1.EnvVar
			for i, env := range container.Env {
				if env.Name == "ANTHROPIC_API_KEY" {
					anthropicKeyEnv = &container.Env[i]
				} else {
					envMap[env.Name] = env.Value
				}
			}

			Expect(envMap["AGENT_TYPE"]).To(Equal("claudecode"))
			Expect(envMap["KUBERNETES_NAMESPACE"]).To(Equal(taskNamespace))

			// Verify ANTHROPIC_API_KEY comes from secret
			Expect(anthropicKeyEnv).NotTo(BeNil())
			Expect(anthropicKeyEnv.ValueFrom).NotTo(BeNil())
			Expect(anthropicKeyEnv.ValueFrom.SecretKeyRef).NotTo(BeNil())
			Expect(anthropicKeyEnv.ValueFrom.SecretKeyRef.Name).To(Equal("anthropic-api-key"))
			Expect(anthropicKeyEnv.ValueFrom.SecretKeyRef.Key).To(Equal("api-key"))

			By("Verifying ServiceAccount is configured")
			serviceAccountName := fmt.Sprintf("aitask-%s", taskName)
			Expect(podSpec.ServiceAccountName).To(Equal(serviceAccountName))
		})
	})
})
