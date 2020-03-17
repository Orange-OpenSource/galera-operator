# Design overview

## Galera cluster creation

When a Galera resource is created, Kubernetes store this resource and an `add` event for this resource is created. Galera Operator just check through the Kubernetes mecanism is a Galera resource  is `added`, `removed` or `modified`. For all this events, Galera Operator processes it through the same reconcile loop. First Kubernetes pod is created. Galera Operator checks `pod` and `persisten volume claim` resources the same way. So When the first pod is created, Galera Operator sees a new pod, checks is this pod belong to a Galera cluster and if so, reenter the main reconcile loop. This is how a a Galera Cluster is created,  adding pod by pod Galera nodes to the Galera cluster. 

## Galera membership reconciliation

The reconcile loop have several inputs:

    1. One Galera resource processed
    2. P Pods belonging to the Galera resource
    3. C Persistent Volume Claims belonging to the Galera resource

Reconciliation is done that way:

    1. Create current and next revision, current equal to next revision if the Galera cluster is not upgrading
    2. Check if pods and claims are running and ready, it means, that a pod is synced in the Galera cluster by checking if its state is `PRIMARY`
    3. If pods are not ready or terminating, we wait until they are in a final state : `failed`, `terminated` or `ready`
    4. Create new Pod if needed, using the next revision
    5. If no pod need to be created, check if there are not too much pods and delete them one by one only if there data are already copied to other memeber of the Galera cluster
    6. Delete running pod for upgrade, can also delete pvc if new volume requirments are specified


## Galera cluster upgrade

When a cluster upgrade by changing MariaDB version and keeping the same same volume requirements , additional operations are performed.

    1 Delete a pod and keep the volume claim
    2 Start a new pod, as pod and volume claim are not in the same version, start the pod in a standalone mode, ie the pod is not joining the Galera cluster
    3 Run *MYSQL_UPGRADE* on the standalone pod, patch the volume claim to indicate the new revision used and delete the pod
    4 Start a new pod reusing the previous volume claim, as the next revision is used for the pod and the volume claim, join the Galera cluster

## Galera cluster backup

Pods in a Galera cluster are not the same, by default there is one galera container, an additional metric container can be specified and a persistent volume claim is used to contain data mapped to MariaDB datadir. The backup pod contain an additional persistent volume claim and a backup container. This backup container provide an API used to pilot some operations from Galera Operator.

When a Galera Backup resource is created, the backup controller part of the operator will use the method provided (only mariabackup for the moment) to create a backup copy to the second local persistent volume. It will also prepare this backup. Once it is done, a copy will be sent to the remote storage (only S3 for the moment).

## Galera cluster restoration

Restoration is done using a Galera resource telling where to find a backup to restore.

All pods are restored in parallel, each pod is in standalone state and copy data using a backup container. Once the copy is done, pods are deleted and the cluster is rebuild pod by pod using the existing persistent volume containing the data (they are not deleted).

