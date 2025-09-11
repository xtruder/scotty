# Scotty - AI Kubernetes Operator

Scotty is a Kubernetes operator that executes AI agents to perform various tasks in your cluster using natural language prompts triggered by cluster events.

## Project Overview

### What Scotty Does
- Watches for Kubernetes resource events (pod creation, updates, failures, etc.)
- Triggers AI agents based on configured event patterns
- Executes AI agents in isolated pods with controlled permissions
- Provides automated cluster analysis, optimization, and incident response

### Technology Stack
- **Language**: Go
- **Framework**: Kubebuilder (operator SDK)
- **Dependencies**: 
  - controller-runtime (Kubernetes controller framework)
  - client-go (Kubernetes Go client)
  - apimachinery (Kubernetes API types)
- **Container Runtime**: Kubernetes pods for agent execution
- **AI Agents**: claude-code, opencode (containerized)

## Architecture

### Core Components
- **AITask Custom Resource**: Declarative task definition with triggers and permissions
- **Controller Manager**: Watches resources and manages AITask lifecycle  
- **Event Trigger System**: Monitors cluster events and matches against trigger rules
- **Agent Executor**: Spawns and manages AI agent pods with RBAC
- **Result Collector**: Gathers agent output and updates task status

### Directory Structure
```
scotty/
├── api/v1alpha1/           # CRD definitions and types
│   ├── aitask_types.go     # AITask resource definition
│   └── groupversion_info.go
├── internal/controller/    # Controller logic
│   └── aitask_controller.go
├── cmd/                    # Main application entry point
│   └── main.go
├── config/                 # Kubernetes manifests
│   ├── crd/                # Custom Resource Definitions
│   ├── rbac/               # RBAC permissions
│   ├── manager/            # Controller deployment
│   └── samples/            # Sample AITask resources
├── test/                   # Test suites
│   ├── e2e/                # End-to-end tests
│   └── utils/              # Test utilities
├── Dockerfile              # Controller container image
├── Dockerfile.claudecode   # Claude Code agent container image
└── Makefile                # Build and deployment targets
```

## AITask Custom Resource Design

```yaml
apiVersion: scotty.x-truder.net/v1alpha1
kind: AITask
metadata:
  name: pod-failure-analyzer
spec:
  agent: "claudecode"
  prompt: "Analyze why this pod failed and suggest fixes"
  
  # Event-based triggers
  triggers:
    - eventType: "pod-failed"
      selector:
        matchLabels:
          app: "web-server"
      debounce: "5m"  # Prevent spam
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
    # Namespaced permissions (creates Role + RoleBinding)
    namespacedRules:
      - apiGroups: [""]
        resources: ["pods", "events"]
        verbs: ["get", "list", "describe"]
      - apiGroups: ["apps"]
        resources: ["deployments", "replicasets"]
        verbs: ["get", "list"]
    
    # Cluster-wide permissions (creates ClusterRole + ClusterRoleBinding)
    clusterRules:
      - apiGroups: [""]
        resources: ["nodes"]
        verbs: ["get", "list"]
  
  timeout: "15m"
  retryPolicy: "OnFailure"
  
status:
  phase: "Running"
  triggeredBy:
    eventType: "pod-failed"
    resource: "web-server-abc123"
    timestamp: "2024-01-01T12:00:00Z"
  executionPod: "aitask-pod-failure-analyzer-xyz"
  result: "Pod failed due to OOMKilled. Recommend increasing memory limit to 512Mi"
  lastExecution: "2024-01-01T12:15:00Z"
```

## Event Trigger System

### Supported Event Types
- `pod-created` - New pod creation
- `pod-failed` - Pod failure events  
- `pod-oom-killed` - Out of memory kills
- `deployment-scaled` - Deployment scaling events
- `node-not-ready` - Node health issues
- `persistent-volume-full` - Storage alerts
- `service-endpoint-unavailable` - Service connectivity issues

### Trigger Matching
- **Label Selectors**: Target specific workloads
- **Namespace Filtering**: Scope to specific namespaces
- **Debouncing**: Prevent trigger spam with cooldown periods
- **Rate Limiting**: Control agent execution frequency per trigger type

## Development Commands

```bash
# Project initialization
kubebuilder init --domain x-truder.net --repo github.com/offlinehq/scotty
kubebuilder create api --group scotty --version v1alpha1 --kind AITask

# Local development
make install    # Install CRDs
make run        # Run controller locally

# Build and deploy
make docker-build IMG=scotty:latest
make deploy IMG=scotty:latest

# Testing
make test       # Unit tests
make test-e2e   # End-to-end tests
```

## Security & Permissions

### RBAC Design
- **Controller RBAC**: Watch events, manage pods, update AITask status
- **Agent RBAC**: Minimal permissions per AITask specification
- **Service Account Isolation**: Each AITask gets dedicated ServiceAccount
- **Pod Security**: Non-root containers, read-only root filesystem

### Agent Container Images
- `xtruder/scotty-claudecode:latest` - Claude Code agent container
- `xtruder/scotty-opencode:latest` - OpenAI Code agent container  
- Containers include: agent binary, kubectl, basic troubleshooting tools

### Network Policies
- Agent pods isolated from external network by default
- Kubernetes API access only
- Optional: allowlist for specific external resources

