# Roles

Nodes are not playing the same role in a Galera Cluser. Differents roles are created through labels and these roles are dynamics, ie Galera Operator can patch pods to change roles played by a pod depending the status of the Galera Cluster.

There are a Writer Role and a WriterBackup Role about which node can bu used to write datas in the Galera Cluser.

There is a Reader role, generally the Writer is not a Reader, execpt if there is only one running node in the cluster.

There is a Special Role, used by Special node. This node can be used to import datas in the new tables. It is designed to be useful when upgrading parts of the client application using Galera Cluster.

There are two states per pod, a node can be standalone or cluster. When the state is cluster, the pod is part of the cluster, on the other side, when a node is standalone, the node is not part of the cluster. State is used to upgrade a node or to restore a Galera Cluster.
