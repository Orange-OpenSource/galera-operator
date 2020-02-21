# Upgrade Galera Cluster

The process to upgrade a Galera Cluster is to modify the yaml/json.

To upgrade from a 10.2.12 to a 10.4.2:

```yaml
spec:
  replicas: 3
  pod:
    credentialsSecret:
      name: galera-secret
    image: sebs42/mariadb:10.3.12-bionic
```

Change the image:

```yaml
spec:
  replicas: 3
  pod:
    credentialsSecret:
      name: galera-secret
    image: sebs42/mariadb:10.4.2-bionic
```

Today, upgrade is permitted if the new version in greater than the older, following the [semantic versioning](https://semver.org).

New CRDs (upgradeConfig and upgradeRule) will be used to validate if the provided my.cnf is compliant with the new version.