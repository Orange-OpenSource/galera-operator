# Backup a Galera Cluster using Galera Operator

Galera Operator provide a way to backup Galera Cluster using an implemented method to a remote storage. Today, only **mariabackup** method and **S3** storage are available but other solutions can be implemented.

## How to backup a Galera Cluster

First you need to have a Galera Cluster managed by the Galera Operator. GaleraBackup CRD must be deployed on the Kubernetes Cluster.

## Setup S3 secret

Create a Kubernetes secret that contains your S3 credential (change $YOURKEY and $YOURSECRET), this secret will be used by Galera Operator to save Galera backup into S3

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: s3-secret
  namespace: galera
type: opaque
stringData:
  accessKeyId: $YOURKEY
  secretAccessKey: $YOURSECRET
```

Also provide a secret with login and password for a database account to let the operator accesses the Galera Cluster and executes the chosen backup method (mariabackup). You need some permissions on the database account to perform the backup : `RELOAD`, `PROCESS`, `LOCK TABLES` and `REPLICATION CLIENT`. 

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: galera-secret
  namespace: galera
type: opaque
stringData:
  user: $SAVE
  password: $SAVEPWD
```

and on the database container:
```bash
CREATE USER '$SAVE'@'localhost' IDENTIFIED BY '$SAVEPWD';
GRANT RELOAD, PROCESS, LOCK TABLES, REPLICATION CLIENT ON *.* TO 'mariabackup'@'localhost';
```


## Create GaleraBackup CR

The spec indicates the name and namespace of the Galera Cluster to backup (note that is namespace is optional), the backup method and the storage provider.

```yaml
apiVersion: sql.databases/v1beta2
kind: GaleraBackup
metadata:
  name: galera-backup
  namespace: galera
spec:
  galeraName: gal
#  galeraNamespace: galera
  backupMethodType: mariabackup
  mariabackup:
    credentialsBackup:
      name: galera-secret
  storageProviderType: S3
  s3:
#    region:
    endpoint: http://s3.orangedev.fr
    bucket: save
#    forcePathStyle:
    credentialsSecret:
      name: s3-secret
```

## Verify

Check the `status` of the `GaleraBackup` to see if the backup is running and to verify if the backup is completed. Name of the backup is computed.