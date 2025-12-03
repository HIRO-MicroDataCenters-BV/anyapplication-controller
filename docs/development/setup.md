# Development

## **Dependencies**

Requirements:

- Docker
- [Kind cluster](https://kind.sigs.k8s.io/)

## Install Kind clusters

  The command below will create two clusters, installs calico plugin and configures BGP peering between them.

  ```sh  
    ./config/generator/env/clusters.sh create
  ```

## Run local controllers from repo

    TBD


## Run containerized versions via helm charts

1. Generate test cluster configuration using script [here](https://github.com/HIRO-MicroDataCenters-BV/anyapplication-controller/tree/main/config/generator)

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

## Remove Kind clusters

 The command below will remove two kind clusters.

  ```sh  
    ./config/generator/env/clusters.sh delete
  ```

