# Status Conditions and Events

To let users understand what actions are taken by Galera Operator, and to make easier for ops to run the operator and the managed clusters, Status Conditions, Status Phase and Events are used in the standard Kubernetes convention.

Use `kubectl describe <<resource>>` or `kubectl get <<resource>> -o yaml` to access information about a resource. You also can use `kubectl get events` to have access to actions released by Galera Operator.

## Conditions and Phase

### Phase

There are eight different Phases:

- **" "** : uninitialized phase, used to know when it is the first time the Galera resource is seen by the operator
- **Running** : phase of a running Galera Cluster
- **Running&Backuping** : phase when a backup on a running Galera Cluster is in progress
- **Creating** : Galera Cluster that are currently creating, ie not all nodes have been created
- **ScheduleRestore** : first step of restoration : preparing a restoration
- **CopyRestore** : second step of restoration : datas are retrieved from a distant storage solution
- **Restoring** : third step of restoration : building the cluster
- **Failed** : Galera Cluster in failed state are not anymore managed by the Galera Operator, all created resources are not deleted

The phase is used to described the cluster state, so only one phase at a time is possible.

### Conditions

Conditions describe more precisely a Galera Cluster. Conditions can be mixed to describe what are the current operations taken by the operator.

There are five conditions :

- Ready
- Failed
- Scaling
- Upgrading
- Restoring

## Events

The following types of events and their specific instances are common in the lifecycle of a Galera cluster:

- A new pod is created
- A pod is patched
- A pod is deleted
- A dead galera node is replaced
- A new pvc is created
- A pvc is patched
- A pvc is deleted
- A backup failed
- A backup is finnish
- Restoration of a galera cluster is failed

