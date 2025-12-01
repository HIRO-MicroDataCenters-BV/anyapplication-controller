#!/bin/bash

usage() {
    echo "Usage: $0 {create|delete}"
    exit 1
}

main () {
  case "$1" in
      create-config)

          shift
          create_config "$@"
          ;;
      delete-config)
          delete_config
          ;;
      *)
          echo "Error: Invalid command '$1'"
          usage
          ;;
  esac
}

create_config () {    
    validate()

    if [ $# -ne 2 ]; then
        usage
    fi
    local config_file=${1?Please specify config yaml}
    local target_dir=${2?Please specify target_dir} 

    extract_yaml_list "${config_file}" '.clusters[].name' clusters
    extract_yaml_list "${config_file}" '.clusters[].mesh-endpoint' mesh_endpoints
    extract_yaml_list "${config_file}" '.clusters[].placement-endpoint' placement_endpoints

    extract_yaml_value "${config_file}" '.tags.anyapplication-controller' anyapplication_tag
    extract_yaml_value "${config_file}" '.tags.mesh-controller' mesh_tag
    extract_yaml_value "${config_file}" '.tags.placement-controller' placement_tag

    extract_yaml_value "${config_file}" '.orchestrationlib.enabled' orchestrationlib_enabled
    extract_yaml_value "${config_file}" '.orchestrationlib.base-url' orchestrationlib_url

    echo "Creating configuration in ${target_dir}"
    echo " - Clusters:  ${clusters[@]}"
    echo " - Mesh endpoints: ${mesh_endpoints[@]}"
    echo " - Placement endpoints: ${placement_endpoints[@]}"
    echo " - anyapplication controller version: ${anyapplication_tag}"
    echo " - mesh controller           version: ${mesh_tag}"
    echo " - placement controller      version: ${placement_tag}"

    for i in "${!clusters[@]}"; do
        local cluster="${clusters[$i]}"
        local mesh_ip="${mesh_endpoints[$i]}"
        local placement_ip="${placement_endpoints[$i]}"
    
        generate_keys "${cluster}" "${target_dir}"

        generate_anyapplication_values "${cluster}" "${target_dir}" "${anyapplication_tag}"
        generate_mesh_values "${cluster}" "${target_dir}" "${mesh_tag}" "${mesh_ip}"
        generate_placement_values "${cluster}" "${target_dir}" "${placement_tag}" "${placement_ip}"
    done
}

validate() {

  if ! command -v openssl &> /dev/null; then
    echo "openssl is not installed. Please install it before running this script. (e.g brew install openssl)"
    exit 1
  fi

  if ! command -v yq &> /dev/null; then
    echo "yq is not installed. Please install it before running this script. (e.g. brew install yq)"
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

  echo -n $(xxd -plain -cols 32 -s -32 ${target_dir}/${cluster}/private.hex) > "${target_dir}/${cluster}/private.base64.txt"
  echo -n $(xxd -plain -cols 32 -s -32 ${target_dir}/${cluster}/public.hex) > "${target_dir}/${cluster}/public.base64.txt"
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
    chmod u+x ${target_dir}/${cluster}/*.sh
}

generate_mesh_values() {
    local cluster=$1
    local target_dir=$2
    local tag=$3
    local mesh_endpoint=$4

    # global clusters
    # global mesh_endpoints

    echo "generate_mesh_values clusters ${clusters[@]}"

    private_key=$(cat "${target_dir}/${cluster}/private.base64.txt")
    public_key=$(cat "${target_dir}/${cluster}/public.base64.txt")

    cat > "${target_dir}/${cluster}/mesh-values.yaml" <<EOF
image:
    tag: "$tag"

secretKey: "${private_key}"
# pub: ${public_key}

configuration:
  mesh:
    zone: "${cluster}"

  nodes:
$(for i in "${!clusters[@]}"; do
    local cluster_i="${clusters[$i]}"
    local cluster_i_pub_key=$(cat "${target_dir}/${cluster_i}/public.base64.txt")
    local cluster_i_mesh_endpoint="${mesh_endpoints[$i]}"
    echo "    # ${cluster_i}:"
    echo "    - public_key: ${cluster_i_pub_key}"
    echo "      endpoints:"
    echo "        - ${cluster_i_mesh_endpoint}"
    echo ""
done)  
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

    chmod u+x ${target_dir}/${cluster}/*.sh
}

generate_placement_values() {
    local cluster=$1
    local target_dir=$2
    local tag=$3
    local load_balancer_ip=$4

    # global clusters
    # global placement_endpoints

    cat > "${target_dir}/${cluster}/placement-values.yaml" <<EOF
image:
    tag: "${tag}"

service:
  type: LoadBalancer
  loadBalancerIP: ${load_balancer_ip}

settings:
  k8s:
    incluster: true
    context: "${cluster}"
    timeout_seconds: 3600

  placement:
    namespace: default
    available_zones:
$(for i in "${!clusters[@]}"; do
    local cluster_i="${clusters[$i]}"
    echo "      - ${cluster_i}"
done)

    current_zone: "${cluster}"

    static_controller_endpoints:
$(for i in "${!placement_endpoints[@]}"; do
    local placement_endpoint_ip_i="${placement_endpoints[$i]}"
    local cluster_i="${clusters[$i]}"
    echo "      ${cluster_i}: http://${placement_endpoint_ip_i}:8000"
done)

    application_controller_endpoint: http://anyapp-anyapplication.default.svc.cluster.local:9000

$(gen_orchestration_lib_section)

  metrics:
    static_metrics:
      - metric: cost
        value_per_unit:
          cpu: 3.0
          memory: 2.0
          storage: 0.000000001
          gpu: 3.0
          ephemeral-storage: 0.000000001
        weight:
          cpu: 1.0
          memory: 1.0
          storage: 1.0
          gpu: 1.0
          ephemeral-storage: 1.0
        method: weighted_average
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

    chmod u+x ${target_dir}/${cluster}/*.sh
}

gen_orchestration_lib_section() {
    if [[ $orchestrationlib_enabled == true ]]; then
        echo "  orchestrationlib:"
        echo "    enabled: true"
        echo "    base_url: ${orchestrationlib_url}"
    else
        echo "  orchestrationlib:"
        echo "    enabled: false"
        echo "    base_url: localhost"
    fi
}

extract_yaml_list() {
    local yaml_file="$1"
    local yaml_query="$2"
    local array_name="$3"

    # We'll accumulate items here
    local item
    local items=()

    # Read YAML values line-by-line for macOS/Linux portability
    while IFS= read -r item; do
        items+=("$item")
    done < <(yq -r "$yaml_query" "$yaml_file")

    # Export array back to caller via indirect reference
    eval "$array_name=(\"\${items[@]}\")"
}

extract_yaml_value() {
    local yaml_file="$1"
    local yaml_query="$2"
    local var_name="$3"

    # Extract the field
    local value
    value=$(yq -r "$yaml_query" "$yaml_file")

    # Assign via indirect reference
    printf -v "$var_name" "%s" "$value"
}

main "$@"