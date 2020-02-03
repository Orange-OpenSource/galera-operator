# Resource Labels

Galera operator creates galera clusters using several Kubernetes resources like pods, persistentVolumeClaims, services, ...

To manage galera cluster, some labels are added to managed resources and must not be overwritten. Labels are key/value parameters, these is the keys used by Galera Operator:

    galera.v1beta2.sql.databases/galera-name
	galera.v1beta2.sql.databases/galera-namespace
	galera.v1beta2.sql.databases/scope
    galera.v1beta2.sql.databases/controller-revision-hash
	galera.v1beta2.sql.databases/state
