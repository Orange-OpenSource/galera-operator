# Restoring a Galera Cluster

To restore a Galera Cluster, you use the same Galera Custom Resource. Additional fields need to be see in the `spec` section of the Galera resource with the name of the backup and also information about the storage used.


```yaml
...
spec:
  replicas: 3
  restore:
    name: gal-galera-backup.20200128172501.sql.gz
    storageProviderType: S3
    s3:
      #region:
      endpoint: $YOURENDPOINT
      bucket: save
      #forcePathStyle:
      credentialsSecret:
        name: s3-secret
...
```