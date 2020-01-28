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

package v1beta2

const (
	// ClusterCRDResourceKind is the Kind of a Cluster.
	GaleraCRDResourceKind = "Galera"
	// GaleraBackupCRDResourceKind is the Kind of a Backup.
	//GaleraBackupCRDResourceKind = "GaleraBackup"

	BootstrapContainerName = "bootstrap"
	GaleraContainerName    = "galera"
	MetricContainerName    = "metric"
	BackupContainerName    = "backup"

	GaleraClusterName      = "galera.v1beta2.sql.databases/galera-name"
	GaleraClusterNamespace = "galera.v1beta2.sql.databases/galera-namespace"

	DataVolumeName      = "galera-data"
	DataVolumeMountPath = "/var/lib/mysql"

	BackupVolumeName      = "galera-backup"
	BackupVolumeMountPath = "/var/backup"

	BootstrapVolumeName      = "bootstrap"
	BootstrapVolumeMountPath = "/bootstrap"

	RestoreVolumeName = "restore"

	ConfigMapVolumeName = "configmap"
	ConfigMapVolumeMountPath = "/configmap"

	// GaleraAnnotationScope annotation name for defining instance scope. Used for specifing cluster wide clusters.
	GaleraAnnotationScope = "galera.v1beta2.sql.databases/scope"
	//AnnotationClusterWide annotation value for cluster wide clusters.
	GaleraAnnotationClusterWide = "clusterwide"

	GaleraRevisionLabel = "galera.v1beta2.sql.databases/controller-revision-hash"
	GaleraStateLabel    = "galera.v1beta2.sql.databases/state"
	StateCluster        = "cluster"
	StateStandalone     = "standalone"
//	GaleraPodNameLabel  = "galera.v1beta2.sql.databases/pod-name"

	GaleraRoleLabel     = "galera.v1beta2.sql.databases/role"
	GaleraBackupLabel   = "galera.v1beta2.sql.databases/backup"
	GaleraReaderLabel   = "galera.v1beta2.sql.databases/reader"
	RoleWriter          = "writer"
	RoleBackupWriter    = "backup-writer"
	RoleSpecial         = "special"
	Backup              = "backup"
	Reader				= "reader"
	Restore				= "restore"

	DefaultRevisionHistoryLimit = 10

	MySQLPort = 3306
	StateSnapshotTransfertPort = 4444
	IncrementalStateTransferPort = 4568
	GaleraReplicationPort = 4567
	GaleraMetrics = 9104
	BackupAPIPort = 8080

	// k8s object name has a maximum length
	MaxK8SNameLength   = 63
	RandomSuffixLength = 5
	ClaimPrefix        = "pvc-"
	BackupClaimPrefix  = "pvcbkp-"
	// Name for a pod will be {GaleraName}-xxxxx
	// Name for a claim will be pvc-{GaleraName}-xxxxx
	// Name for a backup claim will be pvcbkp-{GaleraName}-xxxxx
	MaxNameLength = MaxK8SNameLength - RandomSuffixLength - 1 - 7
	ServiceWriterSuffix = "-writer-svc"
	MaxServiceWriterLength = MaxK8SNameLength - len(ServiceWriterSuffix)
	ServiceWriterBackupSuffix = "-writer-bkp-svc"
	MaxServiceWriterBackupLength = MaxK8SNameLength - len(ServiceWriterBackupSuffix)
	ServiceReaderSuffix = "-reader-svc"
	MaxServiceReaderLength = MaxK8SNameLength - len(ServiceReaderSuffix)
	ServiceSpecialSuffix = "-special-svc"
	MaxServiceSpecialLength = MaxK8SNameLength - len(ServiceSpecialSuffix)
	ServiceMonitorSuffix = "-monitor-svc"
	MaxServiceMonitorLength = MaxK8SNameLength - len(ServiceMonitorSuffix)
	PodDisruptionBudgetSuffix = "-pdb"
	MaxPodDisruptionBudgetLength = MaxK8SNameLength - len(PodDisruptionBudgetSuffix)
)
