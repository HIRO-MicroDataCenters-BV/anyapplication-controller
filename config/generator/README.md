# Mesh Configuration Generator

This script (`generate.sh`) automates the generation and deletion of configuration files and installation scripts for deploying **AnyApplication**, **Mesh**, and **Placement** controllers across multiple Kubernetes clusters.

---

## ✨ Features

- Reads cluster and deployment information from a YAML configuration file.
- Generates cryptographic keys for each cluster.
- Prepares Helm values files for each controller and cluster.
- Creates install/uninstall scripts for each cluster and controller.
- Supports orchestration library configuration.

---

## 🚀 Usage

```sh
./generate.sh create-config <config.yaml> <target_dir>
```
- Generates configuration files and scripts in `<target_dir>` based on `<config.yaml>`.

```sh
./generate.sh delete-config <target_dir>
```
- Deletes all generated configuration files and scripts from `<target_dir>`.

---

## 🛠 Requirements

- **openssl**: for key generation  
    _e.g._, `brew install openssl`
- **yq**: for parsing YAML configuration  
    _e.g._, `brew install yq`
- **helm**: for managing Helm charts  
    [https://helm.sh/](https://helm.sh/)

---

## 📄 YAML Configuration Example

```yaml
tags:
    anyapplication-controller: v1.2.3
    mesh-controller: v4.5.6
    placement-controller: v7.8.9

clusters:
    - name: cluster1
        mesh-endpoint: 10.0.0.1
        placement-endpoint: 10.0.0.2
        anyapplication-endpoint: 10.0.0.3

    - name: cluster2
        mesh-endpoint: 10.0.1.1
        placement-endpoint: 10.0.1.2
        anyapplication-endpoint: 10.0.1.3

orchestrationlib:
    enabled: true
    base-url: http://orchestration.example.com
```

---

## 📦 Output Structure

```
<target_dir>/
    cluster1/
        private.base64.txt
        public.base64.txt
        anyapplication-values.yaml
        mesh-values.yaml
        placement-values.yaml
        anyapplication-install.sh
        mesh-install.sh
        placement-install.sh
        anyapplication-uninstall.sh
        mesh-uninstall.sh
        placement-uninstall.sh
    cluster2/
        ...
```

---

## 📝 Notes

- The script adds required Helm repositories automatically.
- All generated install/uninstall scripts are made executable.
- The script must be run from a shell with access to the required tools and permissions to write to `<target_dir>` Generator
