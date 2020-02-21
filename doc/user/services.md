# galera client service

For every galera cluster created, the galera operator will create several services in the same namespace

```bash
$ kubectl create -f ./example-manifests/galera-cluster/40-galera.yaml
$ kubectl get services
NAME                      TYPE        CLUSTER-IP     EXTERNAL-IP   PORT(S)                               AGE
gal-hl-svc                ClusterIP   None           <none>        3306/TCP,4444/TCP,4568/TCP,4567/TCP   18h
gal-monitor-svc           ClusterIP   10.3.32.191    <none>        9104/TCP                              18h
gal-reader-svc            ClusterIP   10.3.178.182   <none>        3306/TCP                              18h
gal-writer-bkp-svc        ClusterIP   10.3.107.25    <none>        3306/TCP                              18h
gal-writer-svc            ClusterIP   10.3.154.36    <none>        3306/TCP                              18h
```

Services created are also visible in the galera resource status

```bash
$ kubectl get galeras.sql.databases -o yaml
...
  status:
    collisionCount: 0
    conditions:
    - lastTransitionTime: "2020-01-28T21:50:15Z"
      message: ""
      reason: Cluster ready
      status: "True"
      type: Ready
    currentReplicas: 3
    currentRevision: gal-59889f974c
    headlessService: gal-hl-svc
    members:
      backup: gal-t2h97
      backupWriter: gal-zkkzv
      ready:
      - gal-z49fb
      - gal-zkkzv
      - gal-t2h97
      unready: null
      writer: gal-z49fb
    nextReplicas: 3
    nextRevision: gal-59889f974c
    observedGeneration: 1
    phase: Running
    podDisruptionBudgetName: gal-pdb
    replicas: 3
    serviceMonitor: gal-monitor-svc
    serviceReader: gal-reader-svc
    serviceWriter: gal-writer-svc
    serviceWriterBackup: gal-writer-bkp-svc
```

Up to six services are created, an headless service **headlessService** is used for communication between galera cluster members. A monitoring service **serviceMonitor** will only be created if metric fields are given. It is used to facilitate integration with monitoring tools by providing a service with the port defined in the metric field.

Other services are used to access galera cluster.
Galera cluster is a multi masters cluster but it is not recommended to use all nodes to write. So to provide a clean way, **serviceWriter** is the service used to write data in the cluster, a backup exists with the service **serviceWriterBackup** an it have to be only used if the first service is failed. To read datas an other service is provided, it is **serviceReader**. At last a service called **serviceSpecial** is used to access the special node if this node is deployed.

# Accessing the services from outisde the cluster

To access the galera cluster from outside the Kubernetes cluster, several solutions are possible (NodePort, LoadBalancer, metalLB) but are out of the scope of this operator. With the specific design of the mysql client, you have to keep in mind that some solutions are not possible like ingress or proxy not designed for supporting this kind of flow.

If you decide to implement new services, refer to roles.md to pick the right node.
