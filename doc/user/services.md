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

```

Headless service (the service terminating with hl-svc suffix) is used for communication between galera cluster members.

Monitor service (terminating with monito-svc suffix) will only be created if metric fields are given. It is used to facilitate integration with monitoring tools by providing a service with the port defined in the metric field.

There are three services used to access galera cluster:

    1: service



