# Galera cluster Spec examples

## my.cnf


## Three member cluster 

Note: Change $ROOTPASSWORD and $STORAGECLASS for you desired StorageClass or remove this line to use the default storage class is configured

```yaml
spec:
  replicas: 3
  pod:
    credentialsSecret:
      name: galera-secret
    image: sebs42/mariadb:10.4.2-bionic
    mycnfConfigMap:
      name: galeraconf
    env:
      - name: MYSQL_ROOT_PASSWORD
        value: $ROOTPASSWORD
  persistentVolumeClaimSpec:
    accessModes: [ "ReadWriteOnce" ]
    storageClassName: $STORAGECLASS
    resources:
      requests:
        storage: 50Gi
```

## Three member cluster with anti-affinity across nodes

Note: Change CLUSTERNAME to the galera's name. Change $ROOTPASSWORD and $STORAGECLASS for you desired StorageClass or remove this line to use the default storage class is configured

```yaml
spec:
  replicas: 3
  pod:
    credentialsSecret:
      name: galera-secret
    image: sebs42/mariadb:10.4.2-bionic
    mycnfConfigMap:
      name: galeraconf
    env:
      - name: MYSQL_ROOT_PASSWORD
        value: $ROOTPASSWORD
    priorityClassName: high-priority
    affinity:
      podAntiAffinity:
        requiredDuringSchedulingIgnoredDuringExecution:
          - labelSelector:
              matchExpressions:
                - key: galera-cluster
                  operator: In
                  values:
                    - $CLUSTERNAME
            topologyKey: kubernetes.io/hostname
  persistentVolumeClaimSpec:
    accessModes: [ "ReadWriteOnce" ]
    storageClassName: $STORAGECLASS
    resources:
      requests:
        storage: 50Gi
```

For other topology keys, see https://kubernetes.io/docs/concepts/configuration/assign-pod-node/

### Three member cluster with resource requirement

Note: Change $ROOTPASSWORD and $STORAGECLASS for you desired StorageClass or remove this line to use the default storage class is configured

```yaml
spec:
  replicas: 3
  pod:
    credentialsSecret:
      name: galera-secret
    image: sebs42/mariadb:10.4.2-bionic
    mycnfConfigMap:
      name: galeraconf
    resources:
      requests:
        cpu: 500m
        memory: 2Gi
      limits:
        cpu: 1000m
        memory: 4Gi    
    env:
      - name: MYSQL_ROOT_PASSWORD
        value: $ROOTPASSWORD
    priorityClassName: high-priority
  persistentVolumeClaimSpec:
    accessModes: [ "ReadWriteOnce" ]
    storageClassName: $STORAGECLASS
    resources:
      requests:
        storage: 50Gi
```

### Galera cluster with metrics sidecar

```yaml
spec:
  replicas: 3
  pod:
    labels:
      galera-cluster: galera
    credentialsSecret:
      name: galera-secret
    image: sebs42/mariadb:10.4.2-bionic
    mycnfConfigMap: {galeraconf}
    env:
      - name: MYSQL_ROOT_PASSWORD
        valueFrom:
          secretKeyRef:
            name: galera-secret
            key: password
      - name: EXPORTER_PASSWORD
        valueFrom:
          secretKeyRef:
            name: galera-secret
            key: exporter-password
    metric:
      image: prom/mysqld-exporter:v0.11.0
      env:
        - name: DATA_SOURCE_NAME
          value: exporter:${EXPORTER_PASSWORD}@(localhost:3306)
      resources:
        requests:
          cpu: 100m
          memory: 1Gi
        limits:
          cpu: 200m
          memory: 2Gi
      port: 9104
  persistentVolumeClaimSpec:
    accessModes: [ "ReadWriteOnce" ]
    resources:
      requests:
        storage: 90Gi
```

### Galera cluster with custom pod security context

```yaml
spec:
  replicas: 3
  pod:
    labels:
      galera-cluster: galera
    credentialsSecret:
      name: galera-secret
    image: sebs42/mariadb:10.4.5-noroot
    mycnfConfigMap: {galeraconf}
    env:
      - name: MYSQL_ROOT_PASSWORD
        valueFrom:
          secretKeyRef:
            name: galera-secret
            key: password
    securityContect:
      runAsNonRoot: true
      runAsUser: 8000
      fsGroup: 8000
  persistentVolumeClaimSpec:
    accessModes: [ "ReadWriteOnce" ]
    resources:
      requests:
        storage: 90Gi
```
