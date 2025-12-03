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

### Run AnyApplication Controller (kind-cluster1)

  Checkout and build project:

  ```sh
    # Check out repository
    git clone https://github.com/HIRO-MicroDataCenters-BV/anyapplication-controller.git
    cd anyapplication-controller/

    # Build project
    make build
  ```

  Run Controller:

  ```sh
    # install CRDS
    kubectl config use-context kind-kind-cluster1
    make install

    # run controller    
    make run-kind-kind-cluster1
  ```

### Run Mesh Controller (kind-cluster1)

  Install [cargo and rust](https://rust-lang.org/tools/install/) and build project:

  ```sh
    # Check out repository
    git clone https://github.com/HIRO-MicroDataCenters-BV/mesh-controller.git
    cd mesh-controller/

    # build project
    cargo build
  ```
  
  Install [openssl](https://github.com/openssl/openssl) and generate private/public keys and configure them:
  
  ```sh
    openssl genpkey -algorithm ed25519 -outform der -out "./private-key1.der"
    openssl pkey -in "./private-key1.der" -inform DER -pubout -outform DER -out "./public-key1.der"

    echo -n $(xxd -plain -cols 32 -s -32 ./private-key1.der) > "./private-key1.hex"
    echo -n $(xxd -plain -cols 32 -s -32 ./public-key1.der) > "./public-key1.hex"
    
  ```

  Note that this has to be done for all peers (clusters) and configured in the corresponding `kind-cluster<X>.yaml`.

  ```yaml

    private_key_path: "./private-key1.hex"
    
    nodes:
    # public keys and endpoints of other known peers except self
    - public_key: "43b2bb39061bc3267e869303268a81734fb8767d3a17ee490813955bd734fd3a"
        endpoints:
        - "localhost:9102"

  ```

  Run controller:
  ```sh
    # install CRDS
    kubectl --context kind-kind-cluster1 apply -f ./charts/mesh-controller/crds/meshpeer.yaml
    
    # run controller    
    cargo run -- -c ./env/kind-cluster1.yaml
  ```

### Run Placement Controller (kind-cluster1)

  Install [uv](https://github.com/astral-sh/uv) and build project

  ```sh
    # Check out repository
    git clone https://github.com/HIRO-MicroDataCenters-BV/placement-controller.git
    cd placement-controller/

    # build project
    uv build
  ```

  Run controller:

  ```sh    
    uv run python -m placement_controller.main --config ./etc/kind-kind-cluster1.yaml
  ```

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

