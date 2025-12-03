# Decentralised Control Plane for Containerised Applications

# Overview

Decentralized Control Plane for kubernetes applications enables application deployment into a mesh of disjoint kubernetes clusters with intelligent orchestration that considers various optimization criteria - such as application energy consumption, dynamic energy prices, data security zones etc.

![Overview](/overview.png)

The main features of DCP are a decentralized autonomy and an intelligent orchestration.

Each DCP zone is an autonomous peer that replicates entire state and its updates to all other peers. Each zone is capable to recover all deployed applications in case of failures or network partitioning. At the core of the mesh system is p2p network of zone controllers, that together ensure that the application is running in the most optimal cluster.

The intelligent orchestration enables zone-local decisions taking into account global state in all zones. The orchestration system is extensible with a variety of decision making algorithms.

# Features

- Application orchestration in a mesh of disjoint kubernetes clusters
- Optimal application placement decision and continuous decision review
- Extensible support for multiple optimization criteria
- ArgoCD compatible GitOps engine
- Containerised applications packaged with Helm