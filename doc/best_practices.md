# Best practices

## Large-scale deployment

To run galera cluster at large-scale, it is important to assign galera pods to nodes with desired resources, such as SSD and high performance CPU.

### Deploy nodes with local SSD

Galera clusters can use any provided storage class but it is **highly** recommended to use local storage using [local static provisioner](https://github.com/kubernetes-sigs/sig-storage-local-static-provisioner) for performances and be cost effective. RAID are not mandatory and you can use simple disk to reduce cost. Galera Operator brings you fail-over and you shall remove it from infrastructure requirements. Galera Operator is designed to provide high availability through software.

About disk, you can dedicated a disk for high performance instead of partitioning the disk and sharing the iops. Also, be aware of your disk controller constraints when designing your Kubernetes cluster.

### Assign to nodes with desired resources

Kubernetes nodes can be attached with labels. Users can [assign pods to nodes with given labels](http://kubernetes.io/docs/user-guide/node-selection/). Similarly for the galera-operator, users can specify `Node Selector` in the pod field, parts of the spec field, to select nodes for galera pods. For example, users can label a set of nodes with SSD with label `disk="SSD"`. To assign galera pods to these nodes, users can specify `"disk"="SSD"` node selector in the cluster spec.

### Assign to dedicated node

Even with container isolation, not all resources are isolated. Thus, performance interference can still affect galera clusters' performance unexpectedly. Depending of the workload, you can dedicate nodes for each galera pod to achieve predictable performance.

Kubernetes node can be [tainted](https://github.com/kubernetes/community/blob/master/contributors/design-proposals/scheduling/taint-toleration-dedicated.md) with keys. Together with node selector feature, users can create nodes that are dedicated for only running galera clusters. This is the **suggested way** to run high performance galera clusters.

Use kubectl to taint the node with kubectl taint nodes galera dedicated. Than only galera pods, which tolerate this taint will be assigned to the nodes.
