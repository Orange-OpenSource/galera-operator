# Galera cluster Spec examples

## my.cnf

A configuration file must be provided through a **ConfigMap** :

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: galeraconf
  namespace: galera
data:
  my.cnf: |
    [mysqld]

    user=mysql
    max_connections = 400
    max_user_connections=200

    bind-address=0.0.0.0

    # Connection and Thread variables

    #port                           = 3306
    #socket                         = /var/run/mysqld/mysql.sock          # Use mysqld.sock on Ubuntu, conflicts with AppArmor otherwise
    datadir                         = /var/lib/mysql

    # InnoDB variables

    #innodb_strict_mode             = ON
    #innodb_file_format_check       = 1
    #innodb_file_format             = Barracuda                           # For dynamic and compressed InnoDB tables
    innodb_buffer_pool_size         = 128M                                # Go up to 80% of your available RAM
    #innodb_buffer_pool_instances   = 8                                   # Bigger if huge InnoDB Buffer Pool or high concurrency


    # Galera specific MySQL parameter

    default_storage_engine         = InnoDB                            # Galera only works with InnoDB
    innodb_flush_log_at_trx_commit = 0                                 # Durability is achieved by committing to the Group
    innodb_autoinc_lock_mode       = 2                                 # For parallel applying
    innodb_doublewrite             = 1						           # (the default) when using Galera provider of version >= 2.0.
    binlog_format                  = row                               # Galera only works with RBR
    #query_cache_type               = 0                                 # Use QC with Galera only in a Master/Slave set-up
    #query_cache_size               = 0									# only for versions prior to 5.5.40-galera, 10.0.14-galera and 10.1.2

    # WSREP parameter

    wsrep_on                       = on                                  # Only MariaDB >= 10.1
    wsrep_provider                 = /usr/lib/libgalera_smm.so    		# Location of Galera Plugin on Ubuntu
    # wsrep_provider                 = /usr/lib64/galera-3/libgalera_smm.so   # Location of Galera Plugin on CentOS 7
    wsrep_provider_options         = "gcache.size=300M; gcache.page_size=300M"                 # Depends on you workload, WS kept for IST

    wsrep_cluster_name             = "Cluster Name"          		     # Same Cluster name for all nodes
    wsrep_cluster_address          = "gcomm://192.168.0.2,192.168.0.3"   # The addresses of cluster nodes to connect to when starting up

    wsrep_node_name                = "Node A"                            # Unique node name
    wsrep_node_address             = 192.168.0.1                         # Our address where replication is done
    # wsrep_node_incoming_address    = 10.0.0.1                            # Our external interface where application comes from
    # wsrep_sync_wait                = 1                                   # If you need realy full-synchronous replication (Galera 3.6 and newer)
    # wsrep_slave_threads            = 16                                  # 4 - 8 per core, not more than wsrep_cert_deps_distance

    wsrep_sst_method               = mariabackup                         # SST method (initial full sync): mysqldump, rsync, rsync_wan, xtrabackup-v2
    wsrep_sst_auth                 = "sst:secret"                        # Username/password for sst user
    # wsrep_sst_receive_address      = 192.168.0.1                         # Our address where to receive SST

    #[xtrabackup]
    #user=sst2
    #password=$SST_PASSWORD

    [mysql_safe]
    log-error=/var/log/mysqld.log
    pid-file=/var/run/mysqld/mysqld.pid
```

Theses values are modified or added by Galera Operator:

    wsrep_cluster_name
    wsrep_cluster_address
    wsrep_node_name
    wsrep_node_address
    wsrep_on
    wsrep_sst_auth
    datadir


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
