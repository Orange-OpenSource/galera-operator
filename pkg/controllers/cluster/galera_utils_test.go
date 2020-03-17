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
	apigalera "galera-operator/pkg/apis/apigalera/v1beta2"
	"galera-operator/pkg/utils/constants"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	podutil "k8s.io/kubernetes/pkg/api/v1/pod"
	//	"k8s.io/kubernetes/pkg/controller/history"
	//	"reflect"
	"testing"
	//	"k8s.io/apimachinery/pkg/runtime"
	//	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	//	sqlv1beta2 "galera-operator/pkg/apis/apigalera/v1beta2"
)

func TestGetParentNameAndSuffix(t *testing.T) {
	name := "test-gal"
	revision := "59889f974c"
	creds := newSecretForGalera(name)
	config := newConfigMapForGalera(name)

	galera := newGalera(name, creds.Name, config.Name,3)

	credsMap := make(map[string]string, len(creds.Data))
	for k, v := range creds.Data {
		credsMap[k] = string(v)
	}

	pod := newGaleraPod(
		galera,
		revision,
		createUniquePodName(galera.Name, []string{}, []string{}),
		apigalera.RoleWriter,
		apigalera.StateCluster,
		"",
		constants.BootstrapImage,
		constants.BackupImage,
		true,
		credsMap,
	)

	if parent, suffix := getPodParentNameAndSuffix(pod); parent != galera.Name {
		t.Errorf("Extracted the wrong parent name, expected %s found %s", galera.Name, parent)
	} else if len(suffix) != 5 {
		t.Errorf("Extracted the wrong suffix size, expected 5 found %d", len(suffix))
	}

	pod.Name = "foo-$&$&é"
	if parent := getPodParentName(pod); parent != "" {
		t.Errorf("Expected empty string for non-member Pod parent")
	}

	if suffix := getPodSuffix(pod); suffix != "" {
		t.Errorf("Expected empty string for non-member Pod suffix")
	}
}

func TestIsPodMemberOf(t *testing.T) {
	name := "test-gal"
	revision := "59889f974c"
	creds := newSecretForGalera(name)
	config := newConfigMapForGalera(name)

	galera := newGalera(name, creds.Name, config.Name,3)
	galera2 := newGalera(name, creds.Name, config.Name,3)
	galera2.Name = "fake"

	credsMap := make(map[string]string, len(creds.Data))
	for k, v := range creds.Data {
		credsMap[k] = string(v)
	}

	pod := newGaleraPod(
		galera,
		revision,
		createUniquePodName(galera.Name, []string{}, []string{}),
		apigalera.RoleWriter,
		apigalera.StateCluster,
		"",
		constants.BootstrapImage,
		constants.BackupImage,
		true,
		credsMap,
	)

	if !isPodMemberOf(galera, pod) {
		t.Errorf("isPodMemberOf returned false negative")
	}
	if isPodMemberOf(galera2, pod) {
		t.Errorf("isPodMemberOf returned false positive")
	}
}

func TestIsClaimMemberOf(t *testing.T) {
	name := "test-gal"
	revision := "59889f974c"
	creds := newSecretForGalera(name)
	config := newConfigMapForGalera(name)

	galera := newGalera(name, creds.Name, config.Name,3)
	galera2 := newGalera(name, creds.Name, config.Name,3)
	galera2.Name = "fake"

	credsMap := make(map[string]string, len(creds.Data))
	for k, v := range creds.Data {
		credsMap[k] = string(v)
	}

	pod := newGaleraPod(
		galera,
		revision,
		createUniquePodName(galera.Name, []string{}, []string{}),
		apigalera.RoleWriter,
		apigalera.StateCluster,
		"",
		constants.BootstrapImage,
		constants.BackupImage,
		true,
		credsMap,
	)

	claim := getPersistentVolumeClaim(galera, pod)

	if !isClaimMemberOf(galera, claim) {
		t.Errorf("isClaimMemberOf returned false negative")
	}
	if isClaimMemberOf(galera2, claim) {
		t.Errorf("isClaimMemberOf returned false positive")
	}
}

func TestIsRunningAndReady(t *testing.T) {
	name := "test-gal"
	revision := "59889f974c"
	creds := newSecretForGalera(name)
	config := newConfigMapForGalera(name)

	galera := newGalera(name, creds.Name, config.Name,3)

	credsMap := make(map[string]string, len(creds.Data))
	for k, v := range creds.Data {
		credsMap[k] = string(v)
	}

	pod := newGaleraPod(
		galera,
		revision,
		createUniquePodName(galera.Name, []string{}, []string{}),
		apigalera.RoleWriter,
		apigalera.StateCluster,
		"",
		constants.BootstrapImage,
		constants.BackupImage,
		true,
		credsMap,
	)

	state := corev1.ContainerState{Terminated: nil}
	cs := corev1.ContainerStatus{State: state}
	pod.Status.ContainerStatuses = []corev1.ContainerStatus{cs}

	if isRunningAndReady(pod) {
		t.Errorf("isRunningAndReady does not respect Pod phase")
	}
	pod.Status.Phase = corev1.PodRunning
	if isRunningAndReady(pod) {
		t.Errorf("isRunningAndReady does not respect Pod condition")
	}
	condition := corev1.PodCondition{Type: corev1.PodReady, Status: corev1.ConditionTrue}
	podutil.UpdatePodCondition(&pod.Status, &condition)
	if !isRunningAndReady(pod) {
		t.Errorf("Pod should be running and ready")
	}

	cst := corev1.ContainerStateTerminated{
		ExitCode: 1,
		Signal: 1,
		Reason: "testing",
		Message: "testing a terminating container",
	}
	state = corev1.ContainerState{Terminated: &cst}
	cs = corev1.ContainerStatus{State: state}
	pod.Status.ContainerStatuses = []corev1.ContainerStatus{cs}

	if isRunningAndReady(pod) {
		t.Errorf("Pod should not be running and ready")
	}
}

func TestNewPodControllerRef(t *testing.T) {
	name := "test-gal"
	revision := "59889f974c"
	creds := newSecretForGalera(name)
	config := newConfigMapForGalera(name)

	galera := newGalera(name, creds.Name, config.Name,3)

	credsMap := make(map[string]string, len(creds.Data))
	for k, v := range creds.Data {
		credsMap[k] = string(v)
	}

	pod := newGaleraPod(
		galera,
		revision,
		createUniquePodName(galera.Name, []string{}, []string{}),
		apigalera.RoleWriter,
		apigalera.StateCluster,
		"",
		constants.BootstrapImage,
		constants.BackupImage,
		true,
		credsMap,
	)

	controllerRef := metav1.GetControllerOf(pod)
	if controllerRef == nil {
		t.Fatalf("No ControllerRef found on new pod")
	}
	if got, want := controllerRef.APIVersion, apigalera.SchemeGroupVersion.String(); got != want {
		t.Errorf("controllerRef.APIVersion = %q, want %q", got, want)
	}
	if got, want := controllerRef.Kind, "Galera"; got != want {
		t.Errorf("controllerRef.Kind = %q, want %q", got, want)
	}
	if got, want := controllerRef.Name, galera.Name; got != want {
		t.Errorf("controllerRef.Name = %q, want %q", got, want)
	}
	if got, want := controllerRef.UID, galera.UID; got != want {
		t.Errorf("controllerRef.UID = %q, want %q", got, want)
	}
	if got, want := *controllerRef.Controller, true; got != want {
		t.Errorf("controllerRef.Controller = %v, want %v", got, want)
	}
}

/*
func TestCreateApplyRevision(t *testing.T) {
	name := "test-gal"
	creds := newSecretForGalera(name)
	config := newConfigMapForGalera(name)

	galera := newGalera(name, creds.Name, config.Name,3)

	var localSchemeBuilder = runtime.SchemeBuilder{
		sqlv1beta2.AddToScheme,
	}
	Scheme := runtime.NewScheme()
	AddToScheme := localSchemeBuilder.AddToScheme

	utilruntime.Must(AddToScheme(Scheme))

	galera.Status.CollisionCount = new(int32)
	revision, err := newRevision(galera, 1, galera.Status.CollisionCount)
	if err != nil {
		t.Errorf("ici")
		t.Fatal(err)
	}
	galera.Spec.Pod.Image = "newimage:lastest"
	if galera.Annotations == nil {
		galera.Annotations = make(map[string]string)
	}
	key := "foo"
	expectedValue := "bar"
	galera.Annotations[key] = expectedValue
	restoredSet, err := ApplyRevision(galera, revision)
	if err != nil {
		t.Fatal(err)
	}
	restoredRevision, err := newRevision(restoredSet, 2, restoredSet.Status.CollisionCount)
	if err != nil {
		t.Fatal(err)
	}
	if !history.EqualRevision(revision, restoredRevision) {
		t.Errorf("wanted %v got %v", string(revision.Data.Raw), string(restoredRevision.Data.Raw))
	}
	value, ok := restoredRevision.Annotations[key]
	if !ok {
		t.Errorf("missing annotation %s", key)
	}
	if value != expectedValue {
		t.Errorf("for annotation %s wanted %s got %s", key, expectedValue, value)
	}
}
*/

/*
func TestRollingUpdateApplyRevision(t *testing.T) {
	name := "test-gal"
	creds := newSecretForGalera(name)
	config := newConfigMapForGalera(name)

	galera := newGalera(name, creds.Name, config.Name,3)
	galera.Status.CollisionCount = new(int32)
	currentSet := galera.DeepCopy()
	currentRevision, err := newRevision(galera, 1, galera.Status.CollisionCount)
	if err != nil {
		t.Fatal(err)
	}

	//set.Spec.Template.Spec.Containers[0].Env = []v1.EnvVar{{Name: "foo", Value: "bar"}}
	galera.Spec.Pod.Env = []corev1.EnvVar{{Name: "foo", Value: "bar"}}
	updateSet := galera.DeepCopy()
	updateRevision, err := newRevision(galera, 2, galera.Status.CollisionCount)
	if err != nil {
		t.Fatal(err)
	}

	restoredCurrentSet, err := ApplyRevision(galera, currentRevision)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(currentSet.Spec.Pod, restoredCurrentSet.Spec.Pod) {
		t.Errorf("want %v got %v", currentSet.Spec.Pod, restoredCurrentSet.Spec.Pod)
	}

	restoredUpdateSet, err := ApplyRevision(galera, updateRevision)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(updateSet.Spec.Pod, restoredUpdateSet.Spec.Pod) {
		t.Errorf("want %v got %v", updateSet.Spec.Pod, restoredUpdateSet.Spec.Pod)
	}
}
*/

func TestGetPersistentVolumeClaims(t *testing.T) {

}

func newSecretForGalera(name string) *corev1.Secret {
//	strData := map[string]string{"user":"root", "password":"test"}

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
//		StringData: strData,
		Data:       map[string][]byte{"user":[]byte("root"), "password":[]byte("test")},
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
			PersistentVolumeClaimSpec: corev1.PersistentVolumeClaimSpec{
				AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				Resources:        corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: *resource.NewQuantity(1, resource.BinarySI),
					},
				},
			},
			Restore:                   nil,
			RevisionHistoryLimit:      func() *int32 { limit := int32(2); return &limit}(),
		},
	}
}
