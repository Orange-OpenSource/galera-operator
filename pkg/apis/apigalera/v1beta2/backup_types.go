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

// ***************************************************************************
// IMPORTANT FOR CODE GENERATION
// If the types in this file are updated, you will need to run
// `make codegen` to generate the new types under the pkg/client folder.
// ***************************************************************************

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type StorageProviderType string
type MethodType string

const (
	BackupProviderTypeS3        StorageProviderType = "S3"
	BackupMethodTypeMariabackup MethodType          = "mariabackup"
)

// ***************************************************************************
// IMPORTANT FOR CODE GENERATION
// If the types in this file are updated, you will need to run
// `make codegen` to generate the new types under the pkg/client folder.
// ***************************************************************************

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GaleraBackup describes a specification to backup a Galera cluster.
type GaleraBackup struct {
	// we embed these types so GaleraBackup implements runtime.Object
	metav1.TypeMeta `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec GaleraBackupSpec `json:"spec"`
	Status GaleraBackupStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GaleraBackupList defines a List type for our custom GaleraBackup type.
// This is needed in order to make List operations work.
type GaleraBackupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []GaleraBackup `json:"items"`
}

// GaleraBackupSpec contains a backup specification for a Galera cluster.
type GaleraBackupSpec struct {
	// GaleraName is the name of the Galera cluster to backup.
	GaleraName string `json:"galeraName"`

	//	GaleraNamespace is used to specify the namespace when clusterwide operator mode is chosen
	// +optional
	GaleraNamespace string `json:"galeraNamespace,omitempty"`

	// Schedule specifies the cron string used for backup scheduling.
	// +optional
	Schedule string `json:"schedule,omitempty"`

	// MethodType is the galera backup method type used to backup the galera cluster.
	MethodType MethodType `json:"backupMethodType"`

	MethodProvider `json:",inline"`

	// StorageProvider is the galera storage provider type.
	// We need this field because CRD doesn't support validation against invalid fields
	// and we cannot verify invalid backup storage provider.
	StorageProviderType StorageProviderType `json:"storageProviderType"`

	// StorageProvider configures where and how backups should be stored.
	StorageProvider `json:",inline"`

	// ClientTLSSecret is the secret containing the galera TLS client certs and
	// must contain the following data items:
	// data:
	//    "galera-client.crt": <pem-encoded-cert>
	//    "galera-client.key": <pem-encoded-key>
	//    "galera-client-ca.crt": <pem-encoded-ca-cert>
//	ClientTLSSecret string `json:"clientTLSSecret,omitempty"`
}

type MethodProvider struct {
	//
	MariaBackup *MariaBackupMethodProvider `json:"mariabackup,omitempty"`
}

type MariaBackupMethodProvider struct {
	// CredentialsBackup is a reference to the Secret containing the
	// user and password used by mariabackup.
	CredentialsBackup *corev1.LocalObjectReference `json:"credentialsBackup"`
}

// StorageProvider contains the supported backup or restore sources.
type StorageProvider struct {
	// S3 defines the S3 backup source spec.
	S3 *S3StorageProvider `json:"s3,omitempty"`
}
// S3StorageProvider represents an S3 compatible bucket for storing Backups.
type S3StorageProvider struct {
	// Region in which the S3 compatible bucket is located.
	// +optional
	Region string `json:"region,omitempty"`
	// Endpoint (hostname only or fully qualified URI) of S3 compatible
	// storage service.
	Endpoint string `json:"endpoint"`
	// Bucket in which to store the Backup.
	Bucket string `json:"bucket"`
	// ForcePathStyle when set to true forces the request to use path-style
	// addressing, i.e., `http://s3.amazonaws.com/BUCKET/KEY`. By default,
	// the S3 client will use virtual hosted bucket addressing when possible
	// (`http://BUCKET.s3.amazonaws.com/KEY`).
	// +optional
	ForcePathStyle bool `json:"forcePathStyle,omitempty"`
	// CredentialsSecret is a reference to the Secret containing the
	// credentials authenticating with the S3 compatible storage service.
	CredentialsSecret *corev1.LocalObjectReference `json:"credentialsSecret"`
}
