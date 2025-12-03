# Installation

## **Dependencies**

Requirements:

- Kubernetes cluster ≥ 1.32
- Cilium or Calico networking with BGP peering (or exposing services externally via LoadBalancers)

## Install Guide

1. Generate multi cluster configuration using script [here](https://github.com/HIRO-MicroDataCenters-BV/anyapplication-controller/tree/main/config/generator)

  The command below will generate development configuration from ./config-dev.yaml file for two clusters - kind-cluster1 and kind-cluster2.

  ```sh
  ./generate.sh create-config ./config-dev.yaml ./target
  ```

  Note that the script will generate keys for all clusters. New ones are generated on every call.

2. Execute install scripts for all clusters:

  ```sh

  # kind-kind-cluster1
  ./target/kind-kind-cluster1/anyapplication-install.sh
  ./target/kind-kind-cluster1/mesh-install.sh
  ./target/kind-kind-cluster1/placement-install.sh
  
  # kind-kind-cluster2
  ./target/kind-kind-cluster2/anyapplication-install.sh
  ./target/kind-kind-cluster2/mesh-install.sh
  ./target/kind-kind-cluster2/placement-install.sh
  
  ```
## Uninstall Guide

Execute uninstall scripts for all clusters

  ```sh

  # kind-kind-cluster1
  ./target/kind-kind-cluster1/anyapplication-uninstall.sh
  ./target/kind-kind-cluster1/mesh-uninstall.sh
  ./target/kind-kind-cluster1/placement-uninstall.sh
  
  # kind-kind-cluster2
  ./target/kind-kind-cluster2/anyapplication-uninstall.sh
  ./target/kind-kind-cluster2/mesh-uninstall.sh
  ./target/kind-kind-cluster2/placement-uninstall.sh
  
  ```

## Application Deployment

The application is deployed via Kubernetes Resource where the application is referenced by its helm chart.

```json
---
apiVersion: dcp.hiro.io/v1
kind: AnyApplication
metadata:
  name: my-app
  namespace: default
spec:
  source: 
    helm:
      repository: https://helm.app.com/stable
      chart: my-app
      version: 1.0.0
      namespace: default
      values: |-
          # values overrides
  zones: 1
  placementStrategy: 
    strategy: Global
  recoverStrategy:
    tolerance: 1
    maxRetries: 3
```