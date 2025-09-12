---
marp: true
theme: uncover
style: |
  section {
    font-size: 32px;
    background-color: #FFF;
  }

  section.align-center {
    text-align: center;
  }

  .columns {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 0.5rem;
  }
  .left {
    text-align: left;
  }
  .right {
    text-align: left;
  }

  div.twocols {
    margin-top: 35px;
    display: flex;
    gap: 40px;
  }
  div.twocols > * {
    flex: 1;
    margin-top: 0;
    padding-top: 0;
  }
  div.twocols p:first-child,
  div.twocols h1:first-child,
  div.twocols h2:first-child,
  div.twocols ul:first-child,
  div.twocols ul li:first-child,
  div.twocols ul li p:first-child {
    margin-top: 0 !important;
  }
  div.twocols p.break {
    display: none; /* Not needed with flexbox */
  }

paginate: true
---

# Patterns for building
# `Kubernetes Operators`

#### From Basics to your custom cluster Automation

</br>

👨🏻‍💻 Jaka Hudoklin / 𝕏 [@offlinehacker](https://x.com/offlinehacker) / 👷🏼 [Raven.io](https://raven.io)

<sub>@DevOpsDays 2025</sub>

---

## What is a Kubernetes Operator?

##### **Operator** extends Kubernetes with application-specific logic

- **Managing Custom Resources**
  Defines and controls custom kubernetes objects using CRDs
- **Encoding Domain Logic**
  Implements custom logic atop kubernetes (backup, upgrade, scaling strategies)
- **Automating Complex Workflows**
  Handles orchestration of complex workflows to manage application lifecycle

- **Examples**: ArgoCD, cert-manager

---

### Key Concepts
- **Custom Resource (CR)**: Domain-specific objects that extend Kubernetes API
- **Custom Resource Definition (CRD)**: Schema that defines the structure of custom resources
- **Controller**: Control loop that watches for changes and reconciles desired vs actual state
- **Operator Pattern**: Combines CRDs + Controllers + Domain Knowledge

---

## Why Operators?

<style scoped>
p, h3, h4, h5, h6, li {
  text-align: left;
}
</style>

<div class="twocols">
<div>

**Traditional Deployment:**
```bash
kubectl apply -f deployment.yaml
kubectl apply -f service.yaml
kubectl apply -f configmap.yaml
# Manual scaling, updates, backups...
```

</div>
<div>

**Operator Approach:**
```yaml
apiVersion: database.example.com/v1
kind: PostgreSQL
metadata:
  name: my-database
spec:
  version: "13"
  replicas: 3
  backup:
    schedule: "0 2 * * *"
```

</div>
</div>

- **Automation**: Codifies operational knowledge
- **Self-healing**: Automatically recovers from failures
- **Day-2 Operations**: Handles updates, scaling, backups
- **Declarative**: Describe desired state, not procedures

---

## Operator Architecture

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   kubectl apply │--->│  Kubernetes API  │--->│ Custom Resource │
│   my-app.yaml   │    │     Server       │    │   (Stored in    │
└─────────────────┘    └──────────────────┘    │     etcd)       │
                                ^              └─────────────────┘
                                │                        │
                                │ Update Status          │ Watch Events
                                │                        v
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│  Managed Apps   │<---│   Controller     │<---│  Controller     │
│ (Deployments,   │    │    Actions       │    │    Manager      │
│  Services, PVs) │    │                  │    │   (Watches)     │
└─────────────────┘    └──────────────────┘    └─────────────────┘
```

---

<style scoped>p, h2 { text-align: left; }</style>

![bg left:33%](scotty/beam-me-up-scotty.jpg)

## Introducing Scotty

Operator that executes AI agents to perform various tasks in kubernetes cluster.

---

## Here is Idea

<div class="columns">
<div class="left">

```yaml
apiVersion: scotty.x-truder.net/v1alpha1
kind: AITask
metadata:
  name: pod-failure-analyzer
spec:
  agent: "claudecode"
  prompt: "Analyze the cluster resource usage and automatically optimize usage"
  
  # Event-based triggers
  triggers:
    - eventType: "pod-oom-killed"
      selector:
        matchLabels:
          tier: "backend"
  
  # AI agent API key from Kubernetes secret for example your ANTHROPIC_API_KEY
  apiKeySecretRef:
    name: "anthropic-api-key"
    key: "api-key"
  
  # AI agent permissions in target cluster  
  permissions:
    # Cluster-wide permissions (creates ClusterRole + ClusterRoleBinding)
    clusterRules:
      - apiGroups: [""]
        resources: ["pods", "events"]
        verbs: ["get", "list", "describe"]
      - apiGroups: ["apps"]
        resources: ["deployments", "replicasets"]
        verbs: ["get", "list", "update", "patch"]
      - apiGroups: ["autoscaling"]
        resources: ["horizontalpodautoscalers"]
        verbs: ["get", "list", "create", "update", "patch"]
  
  timeout: "15m"
  retryPolicy: "OnFailure"
```

</div>
<div class="right">

- `AITask` custom resource
  - Choose a task that you want your AI agent to perform
  - Define triggers for event-based execution
  - Define RBAC permissions given to agent
- AI agent runs as Kubernetes Job until it completes given task

</div>
</div>

---

```
┌─────────────────┐    ┌──────────────────┐    ┌──────────────────┐
│   kubectl apply │--->│  Kubernetes API  │<---│   Agent Job      │
│   ai-task.yaml  │    │     Server       │    │ (claude/openai)  │
└─────────────────┘    │  1. Register CRD │    └──────────────────┘
                       │  2. Store CR     │               ^
                       └──────────────────┘               │
                                 ^     │                  │
                          watch  │     │ events           │
                         events  │     v                  │
                                 │                        │
                       ┌──────────────────┐               │
                       │ Scotty Controller│---------------┘
                       │ 1. Create Job    │
                       │ 2. Monitor       │
                       └──────────────────┘
```

---

![bg right:66%](scotty/star-trek-enterprise.jpg)

# Let's build it!

---

## Kubebuilder

<div class="columns">
<div class="left">

**Framework that eases the process of building operators** by providing a set of tools and best practices

- Generates CRDs, RBAC YAMLs directly from Go code
- Testing harness for unit and end-to-end testing
- Integrated health checks, metrics and logging

</div>

<div class="right">

Scaffolds project structure:

```
scotty/
├── api/v1alpha1/             # CRD definitions
│   ├── aitask_types.go
│   └── groupversion_info.go
├── internal/controller/      # Controller logic
│   └── aitask_controller.go
├── cmd/                      # Main application
│   └── main.go
├── config/                   # Kubernetes manifests
│   ├── crd/
│   ├── rbac/
│   └── manager/
├── test/                     # Testing
│   ├── e2e/                  # End-to-end tests
│   └── utils/                # Test utilities
├── Dockerfile
└── Makefile
```

</div>
</div>

<sub>Kubebuilder docs is excelent: https://book.kubebuilder.io/</sub>

---

<style scoped>h5 { text-align: left; }</style>

## Controller runtime

##### Set of libraries for building operators

- Client for interacting with Kubernetes API with caching
- Controller manager with leader election
  - Implements event driven reconciliation loop
  - Supports watching for changes on arbitrary objects
- Allows you to define schema using Go struct tags
- Mocked kubernetes API server for unit testing `envtest`
- Utility stuff for metrics, health checks, logging, etc.

</br>

<sub>https://github.com/kubernetes-sigs/controller-runtime</sub>

---

## Development Workflow
```bash
# Initialize project
kubebuilder init --domain x-truder.net --repo github.com/xtruder/scotty

# Create API and controller
kubebuilder create api --group scotty --version v1alpha1 --kind AITask

# Generate manifests and install CRDs
make manifests install

# Run operator locally
make run

# Build and deploy
make docker-build docker-push deploy IMG=my-operator:latest
```

---

<style scoped>section { font-size: 24px; }</style>

## Defining schema

<div class="columns">
<div class="left">

**In:** schema described in code

```go
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Agent",type="string",JSONPath=".spec.agent"
// +kubebuilder:printcolumn:name="Started",type="date",JSONPath=".status.startTime"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

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
```

```go
// AITaskSpec defines the desired state of AITask
type AITaskSpec struct {
	// Agent specifies which AI agent to use
	// +kubebuilder:validation:Required
	Agent AgentType `json:"agent"`

	// Image specifies the container image to use for the AI agent
	// +optional
	Image string `json:"image,omitempty"`

	// Prompt contains the task description or prompt for the AI agent
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Prompt string `json:"prompt"`
  
	// ...
}
```

</div>

<div class="right">

**Out:** CRD with schema definition

```yaml
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: aitasks.scotty.x-truder.net
spec:
  group: scotty.x-truder.net
  names:
    kind: AITask
    plural: aitasks
    singular: aitask
  scope: Namespaced
  versions:
    name: v1alpha1
    schema:
      openAPIV3Schema:
        properties:
          apiVersion:
            type: string
          kind:
            type: string
          metadata:
            type: object
          spec:
            properties:
              agent:
                description: Agent specifies which AI agent to use
                enum:
                - claudecode
                - opencode
                type: string
              image:
                description: Image specifies the container image to use for
                  the AI agent
                type: string
              prompt:
                description: Prompt contains the task description or prompt
                  for the AI agent
                minLength: 1
                type: string
```

</div>
</div>

---

## Reconciliation Loop

Main logic for the operator goes here:

```go
func (r *AITaskReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    // 1. Fetch the custom resource
    var aiTask scottyv1alpha1.AITask
    if err := r.Get(ctx, req.NamespacedName, &aiTask); err != nil {
        return ctrl.Result{}, client.IgnoreNotFound(err)
    }
    
    // 2. Compare desired vs actual state
    job := &batchv1.Job{}
    err := r.Get(ctx, types.NamespacedName{Name: aiTask.Name, Namespace: aiTask.Namespace}, job)
    
    // 3. Take actions
    if errors.IsNotFound(err) {
        // Create job
        return r.createJob(ctx, &aiTask)
    }
    
    // 4. Update status
    aiTask.Status.Complete = true
    return ctrl.Result{}, r.Status().Update(ctx, &aiTask)
}
```

---

## Error Handling & Retry Strategy

- **Transient Errors**: Automatic retry with exponential backoff
- **Terminal Errors**: Return error to stop retries
- **Ignored Errors**: `client.IgnoreNotFound()` for deleted resources
- **Manual Requeue**: `ctrl.Result{RequeueAfter: duration}`
  for periodic checks, provides a simple way to do periodic tasks.

---

## Watching events

`controller-runtime` provides easy way to watch for events on arbitrary kubernetes objects that will trigger reconciliation.

```go
return ctrl.NewControllerManagedBy(mgr).
    For(&scottyv1alpha1.AITask{}).          // Watch primary resource
    Owns(&batchv1.Job{}).                   // Watch owned resources
    Watches(&corev1.Pod{}, ...).            // Watch related resources
    Named("aitask").
    OptionsWith(controller.Options{
      // Prevent concurrent reconciliations causing race conditions
      MaxConcurrentReconciles: 1,
    }).
    Complete(r)
```

---

## Resource Lifecycle Management
- **Owner References**
  Automatic garbage collection when parent gets deleted
- **Finalizers**
  Finalizer will prevent object deletion until it's removed, which enables custom cleanup logic.
- **Status Conditions**
  Track resource state with standard Kubernetes conditions
- **Events**
  Emit events for resource state changes. Allows for historical tracking of object lifecycle, which helps with debugging

---

## Unit Testing

<div class="columns">
<div class="left">

```go
It("should initialize status to Pending on first reconcile", func() {
  By("Reconciling the created resource")
  controllerReconciler := &AITaskReconciler{
    Client:                 k8sClient,
    Scheme:                 k8sClient.Scheme(),
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
})
```

```bash
make test
```

</div>
<div class="right">

- BDD style testing with ginkgo
- Great for testing whether reconciliation loop works as expect
- Provides **real kubernetes apiserver** without the need to run whole kubernetes cluster
- Allows **fast iteration** especially when coding with help of AI

</div>
</div>

---

## End-to-end testing

<div class="columns">
<div class="left">

```go
BeforeAll(func() {
  By("creating manager namespace")
  cmd = exec.Command("kubectl", "create", "ns", namespace)
  _, err := utils.Run(cmd)
  Expect(err).NotTo(HaveOccurred(), "Failed to create namespace")

  By("installing CRDs")
  cmd = exec.Command("make", "install")
  _, err = utils.Run(cmd)
  Expect(err).NotTo(HaveOccurred(), "Failed to install CRDs")

  By("creating API key secret")
  apiKey := os.Getenv("ANTHROPIC_API_KEY")
  cmd = exec.Command("kubectl", "create", "secret", "generic",
    "anthropic-api-key",
    "--from-literal=api-key="+apiKey, "-n", namespace)
  _, err = utils.Run(cmd)
  Expect(err).NotTo(HaveOccurred(), "Failed to create API key secret")

  By("deploying the controller-manager")
  cmd = exec.Command("make", "deploy", fmt.Sprintf("IMG=%s", projectImage))
  _, err = utils.Run(cmd)
  Expect(err).NotTo(HaveOccurred(), "Failed to deploy the controller-manager")
})
```

```bash
make test-e2e
```

</div>
<div class="right">

- Runs kubernetes cluster in a container using [kind](https://kind.sigs.k8s.io/)
- Deploys CRDs and operator to the cluster
- Allows us to test scenarios in a real kubernetes cluster

</div>
</div>

---

![bg left:50%](star-trek-explosion.webp)

## Demo time

```bash
# Deploy Scotty
make deploy

# Create AITask
kubectl apply -f examples/resource-analysis-task.yaml

# Watch progress
kubectl get aitasks -n scotty-system -w
```

---

![bg right:50% w:400](link.svg)

## Questions & Discussion

**Thank you for your attention!**

𝕏 [@offlinehacker](https://x.com/offlinehacker)
[github.com/xtruder/scotty](https://github.com/xtruder/scotty)
