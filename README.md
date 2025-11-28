[![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/HIRO-MicroDataCenters-BV/anyapplication-controller)

# anyapplication-controller

A Kubernetes controller for managing Helm-based applications across multiple zones in distributed cloud platforms (DCP). The controller provides automated deployment, synchronization, and recovery strategies for applications deployed in micro data centers.

## Description

The AnyApplication Controller is a custom Kubernetes operator that extends Kubernetes capabilities to manage applications across multiple zones or micro data centers. It enables:

- **Multi-Zone Deployment**: Deploy Helm charts across multiple zones with configurable placement strategies
- **Automated Synchronization**: Keep applications synchronized across zones with retry and backoff policies
- **Recovery Management**: Automatic recovery with configurable tolerance and retry mechanisms
- **Placement Strategies**: Support for both Local and Global placement strategies
- **Ownership Transfer**: Manage application ownership and state transitions across zones
- **Helm Integration**: Native support for Helm charts with flexible configuration options

## Features

- **Zone-Based Management**: Deploy and manage applications across multiple zones with independent versioning
- **Flexible Placement**: Choose between Local (zone-specific) or Global (multi-zone) placement strategies
- **Self-Healing**: Automated sync policies with prune, self-heal, and retry capabilities
- **State Tracking**: Comprehensive status tracking for deployments, placements, and ownership transfers
- **Helm Support**: Full Helm chart integration with values, parameters, and CRD control


## Core Concepts

### AnyApplication Custom Resource

The `AnyApplication` CRD defines how applications should be deployed across zones. Key specifications include:

- **Source**: Helm chart repository and configuration
- **Zones**: Number of zones where the application should be deployed
- **PlacementStrategy**: Local (single zone) or Global (multi-zone) deployment
- **SyncPolicy**: Automated synchronization with prune, self-heal, and retry options
- **RecoverStrategy**: Tolerance and retry configuration for failure recovery

### Placement Strategies

- **Local**: Application is deployed in a single zone based on zone affinity
- **Global**: Application is deployed across multiple zones for high availability

### Application States

The controller tracks applications through various states:
- `New`: Initial state when application is created
- `Placement`: Application placement is being determined
- `Operational`: Application is running successfully
- `Relocation`: Application is being moved between zones
- `OwnershipTransfer`: Ownership is being transferred
- `Failure`: Application has encountered an error

## Quick Start Example

Here's a simple example deploying an NGINX ingress controller:

```yaml
apiVersion: dcp.hiro.io/v1
kind: AnyApplication
metadata:
  name: nginx-app
  namespace: default
spec:
  source: 
    helm:
      repository: https://helm.nginx.com/stable
      chart: nginx-ingress
      version: 2.0.1
      namespace: default
  zones: 1
  placementStrategy: 
    strategy: Global
  recoverStrategy:
    tolerance: 1
    maxRetries: 3
```

## Getting Started

### Prerequisites
- go version v1.24.0+
- docker version 17.03+.
- kubectl version v1.11.3+.
- Access to a Kubernetes v1.11.3+ cluster.

### To Deploy on the cluster
**Build and push your image to the location specified by `IMG`:**

```sh
make docker-build docker-push IMG=<some-registry>/anyapplication-controller:tag
```

**NOTE:** This image ought to be published in the personal registry you specified.
And it is required to have access to pull the image from the working environment.
Make sure you have the proper permission to the registry if the above commands don’t work.

**Install the CRDs into the cluster:**

```sh
make install
```

**Deploy the Manager to the cluster with the image specified by `IMG`:**

```sh
make deploy IMG=<some-registry>/anyapplication-controller:tag
```

> **NOTE**: If you encounter RBAC errors, you may need to grant yourself cluster-admin
privileges or be logged in as admin.

**Create instances of your solution**
You can apply the samples (examples) from the config/sample:

```sh
kubectl apply -k config/samples/
```

>**NOTE**: Ensure that the samples has default values to test it out.

### To Uninstall
**Delete the instances (CRs) from the cluster:**

```sh
kubectl delete -k config/samples/
```

**Delete the APIs(CRDs) from the cluster:**

```sh
make uninstall
```

**UnDeploy the controller from the cluster:**

```sh
make undeploy
```

## Project Distribution

Following the options to release and provide this solution to the users.

### By providing a bundle with all YAML files

1. Build the installer for the image built and published in the registry:

```sh
make build-installer IMG=<some-registry>/anyapplication-controller:tag
```

**NOTE:** The makefile target mentioned above generates an 'install.yaml'
file in the dist directory. This file contains all the resources built
with Kustomize, which are necessary to install this project without its
dependencies.

2. Using the installer

Users can just run 'kubectl apply -f <URL for YAML BUNDLE>' to install
the project, i.e.:

```sh
kubectl apply -f https://raw.githubusercontent.com/<org>/anyapplication-controller/<tag or branch>/dist/install.yaml
```

### By providing a Helm Chart

1. Build the chart using the optional helm plugin

```sh
kubebuilder edit --plugins=helm/v1-alpha
```

2. See that a chart was generated under 'dist/chart', and users
can obtain this solution from there.

**NOTE:** If you change the project, you need to update the Helm Chart
using the same command above to sync the latest changes. Furthermore,
if you create webhooks, you need to use the above command with
the '--force' flag and manually ensure that any custom configuration
previously added to 'dist/chart/values.yaml' or 'dist/chart/manager/manager.yaml'
is manually re-applied afterwards.

## Advanced Configuration

### Sync Policy

Configure automated synchronization with advanced options:

```yaml
spec:
  syncPolicy:
    automated:
      prune: true        # Delete resources not in source
      selfHeal: true     # Revert manual changes
      allowEmpty: false  # Prevent empty deployments
    syncOptions:
      - CreateNamespace=true
    retry:
      limit: 5
      backoff:
        duration: "5s"
        factor: 2
        maxDuration: "3m"
```

### Multiple Zones

Deploy across multiple zones for high availability:

```yaml
spec:
  zones: 3  # Deploy to 3 zones
  placementStrategy:
    strategy: Global
  recoverStrategy:
    tolerance: 1      # Tolerate 1 zone failure
    maxRetries: 3     # Retry failed deployments 3 times
```

### Helm Parameters

Pass custom parameters to Helm charts:

```yaml
spec:
  source:
    helm:
      repository: https://charts.example.com
      chart: my-app
      version: 1.0.0
      namespace: production
      parameters:
        - name: replicas
          value: "3"
          forceString: true
      skipCrds: false
      values: |
        service:
          type: LoadBalancer
        resources:
          limits:
            memory: "512Mi"
```

## Monitoring

Check the status of your AnyApplication:

```bash
kubectl get anyapplications -n <namespace>
kubectl describe anyapplication <name> -n <namespace>
```

The status section provides detailed information about:
- Ownership and global state
- Zone-specific deployment status
- Condition history with timestamps
- Version tracking per zone

## Architecture

The controller follows the Kubernetes operator pattern:

1. **Reconciliation Loop**: Continuously watches AnyApplication resources
2. **Job Management**: Creates async jobs for deployment, placement, and ownership tasks
3. **Zone Coordination**: Manages application state across multiple zones
4. **Helm Integration**: Generates and applies Helm manifests per zone
5. **Status Updates**: Tracks conditions and versions for each zone

## API Reference

For detailed API documentation, see [docs/api-reference/anyapplication.md](docs/api-reference/anyapplication.md)

## Troubleshooting

### Common Issues

**Application stuck in Placement state**
- Check zone availability and node affinity
- Verify placement strategy configuration

**Sync failures**
- Review retry configuration in syncPolicy
- Check Helm chart repository accessibility
- Examine controller logs: `kubectl logs -n <controller-namespace> deployment/anyapplication-controller`

**Zone version mismatches**
- Verify sync policy is configured correctly
- Check for manual interventions in zones
- Review zone status conditions

## Contributing

We welcome contributions! Here's how to get started:

### Development Setup

1. Clone the repository
2. Install dependencies: `go mod download`
3. Install CRDs: `make install`

### Running Locally

Run the controller locally against a Kubernetes cluster:

```bash
make install  # Install CRDs
make run      # Run controller locally
```

### Testing

- Unit tests: `make test`
- E2E tests: `make test-e2e` (requires Kind cluster)
- Linting: `make lint` or `make lint-fix` for auto-fixes

### Pull Request Process

1. Fork the repository
2. Create a feature branch
3. Make your changes with tests
4. Ensure `make test` and `make lint` pass
5. Submit a pull request with a clear description

### Additional Resources

For detailed information on how to contribute, please refer to our contributing guidelines.

**NOTE:** Run `make help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

## License

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

