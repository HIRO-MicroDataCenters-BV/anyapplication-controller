# Architecture

## Summary

A detailed view of decentralized control plane outlining major functions is provided below:

![Components](/components.png)

The DCP performs three functions:

- Application management - responsible for local and global application management making sure that application is running in the current zone
- Mesh management - responsible for peer state synchronisation across clusters making sure that application state is synchronised
- Optimal decision making - responsible for optimal execution of workload and decision making

The optimization/decision part is extensible and multiple factors can be taken into account. At the moment the placement controller considers only the classical request/limits for applications pods.

## Details

Detailed documentation regarding each component is available via Deep Wiki:

AnyApplication Controller [DeepWiki](https://deepwiki.com/HIRO-MicroDataCenters-BV/anyapplication-controller)

Placement Controller [DeepWiki](https://deepwiki.com/HIRO-MicroDataCenters-BV/placement-controller)

Mesh Controller [DeepWiki](https://deepwiki.com/HIRO-MicroDataCenters-BV/mesh-controller)