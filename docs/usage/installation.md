# Installation

## **Dependencies**

Requirements:

- Kubernetes cluster ≥ 1.32
- Cilium or Calico networking with BGP peering (or exposing services externally via LoadBalancers)

## Install Guide

1. Generate multi cluster configuration using script [here](https://github.com/HIRO-MicroDataCenters-BV/anyapplication-controller/tree/main/config/generator)

2. Execute install scripts for all clusters

    TBD

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