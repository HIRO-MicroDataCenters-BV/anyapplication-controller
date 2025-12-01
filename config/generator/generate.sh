#!/bin/bash

usage() {
    echo "Usage: $0 {create|delete}"
    exit 1
}

main () {
  case "$1" in
      create-dpc-config)

          shift
          create_dcp_config "$@"
          ;;
      delete-dcp-config)
          delete_config
          ;;
      *)
          echo "Error: Invalid command '$1'"
          usage
          ;;
  esac
}

create_dcp_config () {    
    validate()

    if [ $# -ne 2 ]; then
        usage
    fi

    local clusters_arg=${1?Please specify comma separated list of clusters}
    local target_dir=${2?Please specify target_dir} 

    IFS=',' read -ra CLUSTERS <<< "$clusters_arg"
    echo "Creating configuration for clusters ${CLUSTERS[@]} in ${target_dir}"

    for i in "${!CLUSTERS[@]}"; do
        cluster="${CLUSTERS[$i]}"
        generate_keys $cluster $target_dir
        generate_anyapplication_values $cluster $target_dir 0.0.0
        generate_mesh_values $cluster $target_dir 0.0.0
        generate_placement_values $cluster $target_dir 0.0.0
    done
}

validate() {

  if ! command -v openssl &> /dev/null; then
    echo "openssl is not installed. Please install it before running this script."
    exit 1
  fi

  mkdir -p target
}

generate_keys() {
  local cluster=$1
  local target_dir=$2

  echo "Generating keys for ${cluster} in ${target_dir}"

  mkdir -p "${target_dir}/${cluster}"

  openssl genpkey -algorithm ed25519 -outform der -out "${target_dir}/${cluster}/private.hex"
  openssl pkey -in "${target_dir}/${cluster}/private.hex" -inform DER -pubout -outform DER -out "${target_dir}/${cluster}/public.hex"

  echo -n $(xxd -plain -cols 32 -s -32 ${target_dir}/${cluster}/private.hex) > ${target_dir}/${cluster}/private.base64.txt
  echo -n $(xxd -plain -cols 32 -s -32 ${target_dir}/${cluster}/public.hex) > ${target_dir}/${cluster}/public.base64.txt
}

generate_anyapplication_values() {
    local cluster=$1
    local target_dir=$2
    local tag=$3
    local zone=$cluster

    cat > "${target_dir}/${cluster}/anyapplication-values.yaml" <<EOF
image:
    tag: "$tag"
configuration:
    runtime:
        zone: $zone
EOF

    cat > "${target_dir}/${cluster}/anyapplication-install.sh" <<EOF
helm repo update     
helm install --kube-context "${cluster}" anyapp anyapp-repo/anyapplication \\
    --version "${tag}" \\
    --values "${target_dir}/${cluster}/anyapplication-values.yaml"
EOF

    cat > "${target_dir}/${cluster}/anyapplication-uninstall.sh" <<EOF
helm uninstall --kube-context "${cluster}" anyapp
EOF
    chmod a+x "${target_dir}/${cluster}/*.sh"
}

generate_mesh_values() {
    local cluster=$1
    local target_dir=$2
    local tag=$3
    local zone=$cluster

    cat > "${target_dir}/${cluster}/mesh-values.yaml" <<EOF
image:
    tag: "$tag"
EOF

    cat > "${target_dir}/${cluster}/mesh-install.sh" <<EOF
helm repo update     
helm install --kube-context "${cluster}" mesh mesh-repo/mesh-controller \\
    --version "${tag}" \\
    --values "${target_dir}/${cluster}/mesh-values.yaml"
EOF

    cat > "${target_dir}/${cluster}/mesh-uninstall.sh" <<EOF
helm uninstall --kube-context "${cluster}" mesh
EOF

    chmod a+x "${target_dir}/${cluster}/*.sh"
}

generate_placement_values() {
    local cluster=$1
    local target_dir=$2
    local tag=$3
    local zone=$cluster

    cat > "${target_dir}/${cluster}/placement-values.yaml" <<EOF
image:
    tag: "$tag"
EOF

    cat > "${target_dir}/${cluster}/placement-install.sh" <<EOF
helm repo update     
helm install --kube-context "${cluster}" mesh mesh-repo/mesh-controller \\
    --version "${tag}" \\
    --values "${target_dir}/${cluster}/mesh-values.yaml"
EOF

    cat > "${target_dir}/${cluster}/placement-uninstall.sh" <<EOF
helm uninstall --kube-context "${cluster}" placement
EOF

    chmod a+x "${target_dir}/${cluster}/*.sh"
}


main "$@"

