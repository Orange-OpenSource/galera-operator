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

// +genclient
// +genclient:method=GetScale,verb=get,subresource=scale,result=k8s.io/api/autoscaling/v1.Scale
// +genclient:method=UpdateScale,verb=update,subresource=scale,input=k8s.io/api/autoscaling/v1.Scale,result=k8s.io/api/autoscaling/v1.Scale
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Galera describes a specification for a Galera Cluster
type Galera struct {
	// we embed these types so Galera implements runtime.Object
	metav1.TypeMeta `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec GaleraSpec `json:"spec"`

	// Status is the current status of Galera. This data may be out of date by some window of time
	Status GaleraStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GaleraList defines a List type for our custom Galera type.
// This is needed in order to make List operations work.
type GaleraList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list metadata
	// More info: http://releases.k8s.io/HEAD/docs/devel/api-conventions.md#metadata
	metav1.ListMeta `json:"metadata, omitempty"`

	Items []Galera `json:"items"`
}

// GaleraSpec is the spec for a Galera resource
type GaleraSpec struct {
	// Replicas is the desired number of Galera nodes
	// The valid range is from 3 to 9 with only odd values.
	Replicas *int32 `json:"replicas"`

	// Pod defines the template used to create pod for the galera cluster.
	// This field is mandatory
	Pod *PodTemplate `json:"pod"`

	// PersistentVolumeClaimSpec is the spec to describe PVC for the db container
	// This field is mandatory
	PersistentVolumeClaimSpec corev1.PersistentVolumeClaimSpec `json:"persistentVolumeClaimSpec"`

	// Special pod for dedicated purpose like batch integration
	// +optional
	Special *SpecialSpec `json:"special,omitempty"`

	// Galera cluster TLS configuration
	// +optional
	//TLS TLSPolicy `json:"TLS,omitempty"`

	// Restore is the spec to describe data and method used to build a Galera cluster from a backup
	Restore *RestoreSpec `json:"restore, omitempty"`

	// RevisionHistoryLimit is the maximum number of revisions that will
	// be maintained in the Galera's revision history. The revision history
	// consists of all revisions not represented by a currently applied
	// Galera version. The default value is 10.
	// +optional
	RevisionHistoryLimit *int32 `json:"revisionHistoryLimit,omitempty"`
}

// PodTemplate defines the template to create pod for the galera container.
type PodTemplate struct {
	// Tolerations specifies the pod's tolerations.
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// NodeSelector specifies a map of key-value pairs. For the pod to be eligible
	// to run on a node, the node must have each of the indicated key-value pairs as
	// labels.
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// The scheduling constraints on Galera pods.
	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// CredentialsSecret is a reference to the Secret containing the credentials (user/password) used but the
	// operator to communicate with the managed galera cluster
	CredentialsSecret *corev1.LocalObjectReference `json:"credentialsSecret"`

	// Image used to deploy galera cluster. See doc.md about required environment variables and features.
	// Image must follow this naming convention : [repository]/[product]:[version][option]
	// The version must follow the [semver]( http://semver.org) format, for example "10.4.5".
	Image string `json:"image"`

	// Resources is the resource requirements for the Galera containers.
	// This field cannot be updated once the cluster is created.
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// List of environment variables to set in the Galera container.
	// This is used to configure Galera process. Galera cluster cannot be created, when
	// bad environment variables are provided. Do not overwrite any flags used to
	// bootstrap the cluster (for example `--initial-cluster` flag).
	// This field cannot be updated.
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// MycnfConfigMap used to identify a ConfigMap providing a my.cnf configuration file used by Galera cluster
	MycnfConfigMap *corev1.LocalObjectReference `json:"mycnfConfigMap"`

	// Metric container used for metric and/or monitoring purpose
	// +optional
	Metric *MetricSpec `json:"metric,omitempty"`

	// If specified, indicates the pod's priority.
	// If not specified, the pod priority will be default or zero if there is no
	// default.
	// +optional
	PriorityClassName string `json:"priorityClassName,omitempty"`

	// SecurityContext specifies the security context for the entire pod
	// More info: https://kubernetes.io/docs/tasks/configure-pod-container/security-context
	// +optional
	SecurityContext *corev1.PodSecurityContext `json:"securityContext,omitempty"`
}



type MetricSpec struct {
	// Image used by a sidecar container used for metrics and monitoring purposes.
	// +optional
	Image string `json:"image"`

	// Port used by the sidecar container to publish metrics. For example, if prom/mysqld-exporter
	// is used as sidecar, the port 9104 will be used.
	// By default port 9104 is provided
	// +optional
	Port *int32 `json:"port,omitempty"`

	// List of environment variables to set in the metric container.
	// This field cannot be updated.
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// MetricResources is the resource requirements for the metric sidecar containers.
	// This field cannot be updated once the cluster is created.
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
}

type SpecialSpec struct {
	// SpecialResources is the resource requirements for the Galera containers.
	// If not specified, Resources from PodTemplate will be used.
	// This field cannot be updated once the cluster is created.
	// +optional
	SpecialResources *corev1.ResourceRequirements `json:"specialResources,omitempty"`

	// List of environment variables to set in the Galera Special container.
	// This field cannot be updated.
	// +optional
	GaleraSpecialEnv []corev1.EnvVar `json:"specialEnv,omitempty"`

	// MycnfSpecialConfigMap used to identify a ConfigMap providing a my.cnf configuration file used by Galera cluster
	// If not specified, MycnfConfigMap from PodTemplate will be used.
	// This field cannot be updated once the cluster is created.
	// +optional
	MycnfSpecialConfigMap *corev1.LocalObjectReference `json:"mycnfSpecialConfigMap,omitempty"`
}

// Restore is used to manage a restoration from a backup of a Galera cluster.
type RestoreSpec struct {
	// Name of the backup file to restore (it is a Tar + Gzip file)
	Name string `json:"name"`

	// MethodType is the galera restore method type used to backup the galera cluster.
	//MethodType MethodType `json:"restoreMethodType"`

	// StorageProvider is the galera storage provider type.
	// We need this field because CRD doesn't support validation against invalid fields
	// and we cannot verify invalid backup storage provider.
	StorageProviderType StorageProviderType `json:"storageProviderType"`

	// StorageProvider configures where and how backups should be stored.
	StorageProvider `json:",inline"`
}

func (g *Galera) AsOwner() metav1.OwnerReference {
	trueVar := true
	return metav1.OwnerReference{
		APIVersion: SchemeGroupVersion.String(),
		Kind:       GaleraCRDResourceKind,
		Name:       g.Name,
		UID:        g.UID,
		Controller: &trueVar,
	}
}
