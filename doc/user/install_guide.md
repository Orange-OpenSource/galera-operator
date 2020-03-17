# Installation guide

Galera operator is made of two stateless controllers packaged in the same binary.

## Create new galera resources

Kubernetes provides a logical view through **Resources**. Resources are mainly declarative, the requested state is described through the **Spec** field. The current state of a Resource is described in the **Status** field. Resources are managed with **Controllers**. Controllers are designed to reconcile and take actions to bring the observed state to the requested state.

Galera operator is a set of new controllers in charge of new resources. To add new resources to an existing Kubernetes cluster, it uses Custom Resource Definition (CRD)

```bash
$ kubectl apply -f ./example-manifests/galera-crd/galera-crd.yaml
$ kubectl apply -f ./example-manifests/galera-crd/galerabackup-crd.yaml
$ kubectl apply -f ./example-manifests/galera-crd/upgrade-config-crd.yaml
$ kubectl apply -f ./example-manifests/galera-crd/upgrade-rule-crd.yaml
...
$ kubectl api-resources
NAME                              SHORTNAMES   APIGROUP                       NAMESPACED   KIND
...
priorityclasses                   pc           scheduling.k8s.io              false        PriorityClass
galerabackups                     gb           sql.databases                  true         GaleraBackup
galeras                           gl           sql.databases                  true         Galera
upgradeconfigs                    gc           sql.databases                  true         UpgradeConfig
upgraderules                      gr           sql.databases                  true         UpgradeRule
...
```

The four new resources are **Namespaced**, meaning the resources must be attached to a specified **Namespace**.

## Install galera operator

Create if needed a namespace for deploying galera operator:

```bash
$ kubectl create -f  ./example-manifests/galera-operator/00-namespace.yaml
```

### Set up RBAC

It is recommanded to use RBAC for Kubernetes clusters, but it is not mandatory. To work, controllers managing galera resources need to acces Kubernetes API to create, patch, ... different resources. This is done by creating a **Service Account**, defining **Role** (or **ClusterRole**) and binding its together thought a **RoleBinding** (or **ClusterRoleBinding**)

Galera operator needs a **Service Account** to access the Kubernetes API

```bash
$ kubectl create -f ./example-manifests/galera-operator/10-operator-sa.yaml
```

### Galera operator scope

Galera operator can be deployed to only manage galera clusters created in the same namespace. It is possible to deploy a galera operator with special option to manage clusterwide galera clusters.

Galera operator implements subresources, meaning the status and the spec can have different rights. In the chosen design, Galera Operator can read the spec but cannot write or modify it. Only customers of the API can write spec. On the other side, status is written by Galera Controller and you should not give the right to customers to modify the status field, customers should just read the status.

### 1. Managing galera clusters in the same namespace

Create **Role** and use **RoleBinding** to bind the service account with the role:

```bash
$ kubectl create -f ./example-manifests/galera-operator/20-role.yaml
$ kubectl create -f ./example-manifests/galera-operator/30-role-binding.yaml
```

Create **ClusterRole** and use **ClusterRoleBinding** to bind the service account with the role:

```bash
$ kubectl create -f ./example-manifests/galera-operator/40-cluster-role.yaml
$ kubectl create -f ./example-manifests/galera-operator/60-cluster-role-binding.yaml
```

Galera operator is deployed using a **Deployment**

```bash
$ kubectl create -f ./example-manifests/galera-operator/70-operator-deployment.yaml
```


### 2. Managing Clusterwide galera clusters

Create **ClusterRole** and use **ClusterRoleBing** to bind it with the service account:

```bash
$ kubectl create -f ./example-manifests/galera-operator/50-cluster-role-clusterwide.yaml
$ kubectl create -f ./example-manifests/galera-operator/60-cluster-role-binding.yaml
```

To manage galera clusters in all namespaces, galera operator have to run with `-cluster-wide` arg option. You can edit the operator-deploymnet.yaml file and modify it.

```bash
$ kubectl create -f ./example-manifests/galera-operator/70-operator-deployment.yaml
```

## Uninstall galera operator

Note that the galera clusters managed by galera operator will **NOT** be deleted even if the operator is uninstalled.

This is an intentional design to prevent accidental operator failure from killing all the galera clusters.

To delete all clusters, delete all cluster Custom Resource objects before uninstalling the operator.

Clean up galera operator:

```bash
$ kubectl delete -f  ./example-manifests/galera-operator/
```

## Installation using Helm

Today, no Helm chart is available to deploy galera operator.
