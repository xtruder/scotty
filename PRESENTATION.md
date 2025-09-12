---
marp: true
theme: uncover
style: |
  section {
    font-size: 34px;
    background-color: #ccc;
  }

  section.align-center {
    text-align: center;
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

# Patterns for building Kubernetes Operators

#### From Basics to AI-Powered Automation

Jaka Hudoklin / [@offlinehacker](https://x.com/offlinehacker) / [Raven.io](https://raven.io)

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

### Benefits
- **Automation**: Codifies operational knowledge
- **Self-healing**: Automatically recovers from failures
- **Day-2 Operations**: Handles updates, scaling, backups
- **Declarative**: Describe desired state, not procedures

---

## Slide 3: Operator Architecture

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   kubectl apply │──▶│  Kubernetes API │───▶│ Custom Resource │
│   my-app.yaml   │    │     Server       │    │   (Stored in    │
└─────────────────┘    └──────────────────┘    │     etcd)       │
                                ▲              └─────────────────┘
                                │                        │
                                │ Update Status          │ Watch Events
                                │                        ▼
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│  Managed Apps   │◀───│   Controller     │◀───│  Controller     │
│ (Deployments,   │    │    Actions       │    │    Manager      │
│  Services, PVs) │    │                  │    │   (Watches)     │
└─────────────────┘    └──────────────────┘    └─────────────────┘
```

## Slide 4: Controller Pattern Deep Dive

### Reconciliation Loop
```go
func (r *AppReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    // 1. Fetch the custom resource
    var app MyApp
    if err := r.Get(ctx, req.NamespacedName, &app); err != nil {
        return ctrl.Result{}, client.IgnoreNotFound(err)
    }
    
    // 2. Compare desired vs actual state
    deployment := &appsv1.Deployment{}
    err := r.Get(ctx, types.NamespacedName{Name: app.Name, Namespace: app.Namespace}, deployment)
    
    // 3. Take corrective actions
    if errors.IsNotFound(err) {
        // Create deployment
        return r.createDeployment(ctx, &app)
    }
    
    // 4. Update status
    app.Status.Ready = true
    return ctrl.Result{RequeueAfter: time.Minute * 5}, r.Status().Update(ctx, &app)
}
```

## Slide 5: Kubebuilder - Operator SDK

### Project Structure
```
my-operator/
├── api/v1/                    # CRD definitions
│   ├── myapp_types.go
│   └── groupversion_info.go
├── controllers/               # Controller logic
│   └── myapp_controller.go
├── config/                    # Kubernetes manifests
│   ├── crd/
│   ├── rbac/
│   └── manager/
├── Dockerfile
├── Makefile
└── main.go
```

### Development Workflow
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

## Slide 6: CRD Schema Design & Validation

### OpenAPI v3 Schema
Kubebuilder generates OpenAPI schemas from Go struct tags:
```go
type EventType string

const (
    EventTypePodFailed    EventType = "pod-failed"
    EventTypePodOOMKilled EventType = "pod-oom-killed"
    // ...
)

type AITaskSpec struct {
    // +kubebuilder:validation:Enum=claudecode;opencode
    // +kubebuilder:validation:Required
    Agent string `json:"agent"`
    
    // +kubebuilder:validation:MinLength=1
    Prompt string `json:"prompt"`
    
    // +kubebuilder:default="15m"
    Timeout *metav1.Duration `json:"timeout,omitempty"`
    
    Triggers []AITaskTrigger `json:"triggers,omitempty"`
}
```

### Validation Features
- **Enums**: Strong typing with predefined values
- **Required/Optional**: Field validation with `+kubebuilder:validation:Required`
- **String Constraints**: MinLength, MaxLength, Pattern (regex)
- **Numeric Constraints**: Minimum, Maximum, MultipleOf
- **Default Values**: `+kubebuilder:default="value"`
- **Array Validation**: MinItems, MaxItems, UniqueItems

### Merge Strategies
```go
// +listType=map
// +listMapKey=type
Conditions []metav1.Condition `json:"conditions,omitempty"`

// +listType=atomic  - replace entire array
// +listType=set     - treat as set of scalars
// +listType=map     - merge by key field
```

### Generated CRD Validation
```yaml
spec:
  properties:
    agent:
      enum: ["claudecode", "opencode"]
      type: string
    triggers:
      items:
        properties:
          eventType:
            enum: ["pod-failed", "pod-oom-killed", "deployment-scaled"]
            type: string
```

## Slide 7: Real-World Example - Scotty AI Operator

### Problem Statement
- Teams need AI assistance for Kubernetes operations
- Manual AI agent execution is cumbersome
- Need controlled, auditable AI operations with proper RBAC

### Solution: Scotty Operator
```yaml
apiVersion: scotty.x-truder.net/v1alpha1
kind: AITas
metadata:
  name: optimize-resources
spec:
  agent: claudecode
  prompt: "Analyze pod resource usage and suggest optimizationsL
  permissions:
    - apiGroups: [""]
      resources: ["pods", "nodes"]
      verbs: ["get", "list"]
  timeout: "30m"
```

## Slide 8: Scotty Architecture

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   kubectl apply │───▶│    AITask CRD    │───▶│   AITask CR     │
│   ai-task.yaml  │    │                  │    │  (with prompt)  │
└─────────────────┘    └──────────────────┘    └─────────────────┘
                                                         │
                                                         ▼
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   Agent Pod     │◀───│  Scotty Controller│◀───│   Watch Events  │
│ (claude/openai) │    │                  │    │                 │
│  + K8s RBAC     │    │ 1. Create Pod    │    └─────────────────┘
│  + Task Prompt  │    │ 2. Monitor       │              
└─────────────────┘    │ 3. Collect Result│              
         │              └──────────────────┘              
         ▼                       ▲                        
┌─────────────────┐              │                        
│  Task Result    │──────────────┘                        
│ (Analysis/Fix)  │                                       
└─────────────────┘                                       
```

## Slide 9: Reconciliation Loop Deep Dive

### Core Reconciliation Pattern
```go
func (r *MyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    // 1. Fetch the resource
    var resource MyResource
    if err := r.Get(ctx, req.NamespacedName, &resource); err != nil {
        return ctrl.Result{}, client.IgnoreNotFound(err)  // Resource deleted
    }
    
    // 2. Determine current vs desired state
    actualState := r.getCurrentState(ctx, &resource)
    desiredState := resource.Spec
    
    // 3. Take corrective actions
    if !reflect.DeepEqual(actualState, desiredState) {
        if err := r.reconcileState(ctx, &resource); err != nil {
            return ctrl.Result{}, err  // Retry automatically
        }
    }
    
    // 4. Update status
    resource.Status = actualState
    return ctrl.Result{RequeueAfter: time.Minute}, r.Status().Update(ctx, &resource)
}
```

### Error Handling & Retry Strategy
- **Transient Errors**: Automatic retry with exponential backoff
- **Terminal Errors**: Return error to stop retries
- **Ignored Errors**: `client.IgnoreNotFound()` for deleted resources
- **Manual Requeue**: `ctrl.Result{RequeueAfter: duration}` for periodic checks

### Reconciliation Triggers
```go
return ctrl.NewControllerManagedBy(mgr).
    For(&MyResource{}).          // Watch primary resource
    Owns(&batchv1.Job{}).        // Watch owned resources
    Watches(&corev1.Pod{}, ...). // Watch related resources
    Complete(r)
```

### Resource Lifecycle Management
- **Owner References**: Automatic garbage collection when parent deleted
- **Finalizers**: Custom cleanup logic before resource deletion
- **Status Conditions**: Track resource state with standard Kubernetes conditions

## Slide 10: Benefits of the Operator Pattern

### For Platform Teams
- **Standardization**: Consistent deployment and management patterns
- **Governance**: Built-in RBAC and policy enforcement
- **Observability**: Native Kubernetes monitoring and logging
- **GitOps Integration**: Declarative configuration in version control

### For Development Teams
- **Self-Service**: Deploy complex applications with simple YAML
- **Consistency**: Same experience across environments
- **Automation**: No manual intervention required
- **Reliability**: Built-in error handling and recovery

## Slide 11: Operator Maturity Levels

### Level 1: Basic Install
- Automated installation via Kubernetes API
- Basic configuration management

### Level 2: Seamless Upgrades
- Patch and minor version updates
- Configuration changes without downtime

### Level 3: Full Lifecycle
- Application lifecycle management
- Backup, restore, and failure recovery

### Level 4: Deep Insights
- Metrics, alerting, and log processing
- Workload analysis and tuning

### Level 5: Auto Pilot
- Horizontal/vertical scaling
- Abnormality detection and correction

## Slide 12: Best Practices

### Design Principles
- **Declarative APIs**: Use spec/status pattern
- **Idempotent Operations**: Safe to retry reconciliation
- **Event-driven**: React to resource changes, not polling
- **Graceful Degradation**: Handle partial failures

### Development Tips
- Start with kubebuilder scaffolding
- Use controller-runtime testing utilities
- Implement proper RBAC from the start
- Add comprehensive observability
- Plan for upgrade scenarios

### Operational Considerations
- Monitor operator health and performance
- Implement proper backup/restore procedures
- Version your CRDs carefully
- Document operational procedures

## Slide 13: Demo Time - Scotty in Action

### What We'll Show
1. **Setup**: Deploy Scotty operator to cluster
2. **Create Task**: Apply AITask resource for cluster analysis
3. **Execution**: Watch operator create and manage AI agent pod
4. **Results**: Examine task results and recommendations
5. **Cleanup**: Observe automatic resource cleanup

### Live Demo Commands
```bash
# Deploy Scotty
make deploy IMG=scotty:latest

# Create AITask
kubectl apply -f examples/resource-analysis-task.yaml

# Watch progress
kubectl get aitasks -w
kubectl logs -f deployment/scotty-controller-manager

# Get results
kubectl get aitask resource-analysis -o yaml
```

## Slide 14: Next Steps & Resources

### Extending Scotty
- **Trigger System**: Cron schedules, webhooks, resource events
- **Agent Marketplace**: Support for custom AI agents
- **Result Storage**: Persist analysis results and recommendations
- **Integration**: Connect with monitoring, alerting, and GitOps tools

### Learning Resources
- **Kubebuilder Book**: https://book.kubebuilder.io/
- **Operator SDK**: https://sdk.operatorframework.io/
- **CNCF Operator White Paper**: Operator pattern best practices
- **Controller Runtime**: https://github.com/kubernetes-sigs/controller-runtime

### Community
- **Operator Framework**: OperatorHub.io for discovering operators
- **SIG API Machinery**: Kubernetes API extension discussions
- **KubeCon Talks**: Latest patterns and practices

## Questions & Discussion

**Key Takeaways:**
- Operators codify operational knowledge into Kubernetes-native applications
- The pattern enables true automation beyond simple deployments
- Kubebuilder provides excellent tooling for operator development
- Scotty demonstrates how operators can enable new capabilities like AI-powered cluster management

**Thank you for your attention!**
