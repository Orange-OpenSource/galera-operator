// Copyright 2020 Orange SA
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cluster

import (
	"fmt"
	"galera-operator/pkg/utils/constants"
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	apigalera "galera-operator/pkg/apis/apigalera/v1beta2"
)

func TestGetParentNameAndSuffix(t *testing.T) {
	name := "test"
	creds := newSecretForGalera(name)
	config := newConfigMapForGalera(name)


	galera := newGalera(name, creds.Name, config.Name,3)

//	mapCredGalera, err := gc.secretControl.GetGaleraCreds(currentGalera)

	credsMap := make(map[string]string, len(creds.Data))
	for k, v := range creds.Data {
		credsMap[k] = string(v)
	}

	pod := newGaleraPod(
		galera,
		"revision",
		"toto",
		apigalera.RoleWriter,
		apigalera.StateCluster,
		"",
		constants.BootstrapImage,
		constants.BackupImage,
		true,
		credsMap,
	)


}

func TestGetClaimParentNameAndSuffix(t *testing.T) {

}

func TestCreateUniquePodName(t *testing.T) {
	clusterName := "test"
	existing := []string{"foo", "bar"}
	available := []string{}

}

func TestIsPodMemberOf(t *testing.T) {

}

func TestIsClaimMemberOf(t *testing.T) {

}

func TestGetBootstrapAddresses(t *testing.T) {

}

func TestIsRunningAndReady(t *testing.T) {

}

func TestNewPodControllerRef(t *testing.T) {

}

func TestCreateApplyRevision(t *testing.T) {

}

func TestRollingUpdateApplyRevision(t *testing.T) {

}

func TestGetPersistentVolumeClaims(t *testing.T) {

}

func newSecretForGalera(name string) *corev1.Secret {
	strData := map[string]string{"user":"root", "password":"test"}

	return &corev1.Secret{
		TypeMeta:   metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-secret", name),
			Namespace: corev1.NamespaceDefault,
		},
		Type:       corev1.SecretTypeOpaque,
		StringData: strData,
	}
}

func newConfigMapForGalera(name string) *corev1.ConfigMap {
	data := make(map[string]string)

	data["my.cnf"] = `[mysqld]
user=mysql
bind-address=0.0.0.0

# Connection and Thread variables
#port                           = 3306
#socket                         = /var/run/mysqld/mysql.sock          # Use mysqld.sock on Ubuntu, conflicts with AppArmor otherwise
datadir                         = /var/lib/mysql

default_storage_engine         = InnoDB                            # Galera only works with InnoDB
innodb_flush_log_at_trx_commit = 0                                 # Durability is achieved by committing to the Group
innodb_autoinc_lock_mode       = 2                                 # For parallel applying
innodb_doublewrite             = 1						           # (the default) when using Galera provider of version >= 2.0.
binlog_format                  = row	

# WSREP parameter
wsrep_on                       = on                                  # Only MariaDB >= 10.1
wsrep_provider                 = /usr/lib/libgalera_smm.so    		# Location of Galera Plugin on Ubuntu
wsrep_provider_options         = "gcache.size=300M; gcache.page_size=300M"                 # Depends on you workload, WS kept for IST

wsrep_cluster_name             = "Cluster Name"          		     # Same Cluster name for all nodes
wsrep_cluster_address          = "gcomm://192.168.0.2,192.168.0.3"   # The addresses of cluster nodes to connect to when starting up

wsrep_node_name                = "Node A"                            # Unique node name
wsrep_node_address             = 192.168.0.1                         # Our address where replication is done
# wsrep_node_incoming_address    = 10.0.0.1                            # Our external interface where application comes from
# wsrep_sync_wait                = 1                                   # If you need realy full-synchronous replication (Galera 3.6 and newer)
# wsrep_slave_threads            = 16                                  # 4 - 8 per core, not more than wsrep_cert_deps_distance

wsrep_sst_method               = mariabackup                         # SST method (initial full sync): mysqldump, rsync, rsync_wan, xtrabackup-v2
`

	return &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-config", name),
			Namespace: corev1.NamespaceDefault,
		},
		Data: data,
	}
}

func newGalera(galeraName, credsName, configName string, replicas int) *apigalera.Galera {



	credsRef := corev1.LocalObjectReference{Name: credsName}


	configRef := corev1.LocalObjectReference{Name: configName}

	env := []corev1.EnvVar{corev1.EnvVar{Name: "MYSQL_ROOT_PASSWORD", Value: "test"}}

	podTemplate := apigalera.PodTemplate{
		CredentialsSecret: &credsRef,
		Image:             "sebs42/mariadb:10.4.2-bionic",
		Env:               env,
		MycnfConfigMap:    &configRef,
	}

	return &apigalera.Galera{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Galera",
			APIVersion: "sql.databases/v1beta2",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      galeraName,
			Namespace: corev1.NamespaceDefault,
			UID:       types.UID("test"),
			Labels:    map[string]string{"foo": "bar"},
		},
		Spec: apigalera.GaleraSpec{
			Replicas:                  func() *int32 { i:= int32(replicas); return &i }(),
			Pod:                       &podTemplate,
			PersistentVolumeClaimSpec: corev1.PersistentVolumeClaimSpec{},
			Restore:                   nil,
			RevisionHistoryLimit:      func() *int32 { limit := int32(2); return &limit}(),
		},
	}
}