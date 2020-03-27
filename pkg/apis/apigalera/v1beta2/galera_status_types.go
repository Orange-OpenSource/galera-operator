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
	"fmt"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type GaleraPhase string

const (
	// used for Galera Cluster that are not in a defined state
	GaleraPhaseNone GaleraPhase = ""
	// used for Galera Cluster that are currently running
	GaleraPhaseRunning GaleraPhase = "Running"
	// used for Galera Cluster that are currently running & backuping
	GaleraPhaseBackuping GaleraPhase = "Running&Backuping"
	// used for Galera Cluster that are currently creating
	GaleraPhaseCreating GaleraPhase = "Creating"
	// used for Galera cluster that are preparing for a restore (1st step of restoration)
	GaleraPhaseScheduleRestore GaleraPhase = "ScheduleRestore"
	// used for Galera cluster that are currently restoring (2rd step of restoration)
	GaleraPhaseCopyRestore GaleraPhase = "CopyRestore"
	// used for Galera cluster that are currently restoring (3rd step of restoration)
	GaleraPhaseRestoring GaleraPhase = "Restoring"
	// used for Galera Cluster that are in failed state
	GaleraPhaseFailed GaleraPhase = "Failed"
)

type GaleraConditionType string

const (
	GaleraConditionReady  GaleraConditionType = "Ready"
	GaleraConditionFailed GaleraConditionType = "Failed"
	GaleraConditionScaling GaleraConditionType = "Scaling"
	GaleraConditionUpgrading GaleraConditionType  = "Upgrading"
	GaleraConditionRestoring  GaleraConditionType = "Restoring"
)

// GaleraStatus is the status for a Galera resource
type GaleraStatus struct {
	// observedGeneration is the most recent generation observed for this Galera. It corresponds to the
	// Galera's generation, which is updated on mutation by the API Server.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration"`

	// Phase is the cluster running phase
	Phase GaleraPhase `json:"phase"`

	// ServiceWriter is the LB service for accessing Galera Writer node
	ServiceWriter string `json:"serviceWriter"`

	// ServiceWriter is the LB service for accessing Galera Writer Backup node
	ServiceWriterBackup string `json:"serviceWriterBackup"`

	// ServiceReader is the LB service for accessing Galera Reader nodes
	ServiceReader string `json:"serviceReader"`

	// ServiceSpecial is the LB service for accessing Galera Special node
	ServiceSpecial string `json:"serviceSpecial,omitempty"`

	// HeadlessService is a clusterIPNone service used to allow ports needed between Galera cluster members
	HeadlessService string `json:"headlessService"`

	// ServiceMonitor is the LB service for accessing all Galera nodes if a sidecar for monitoring is deployed
	ServiceMonitor string `json:"serviceMonitor,omitempty"`

	// PodDisruptionBudgetName is the PodDisruptionBudget associated with the galera cluster
	PodDisruptionBudgetName string `json:"podDisruptionBudgetName"`

	// Current size of the cluster, it is the number of Pods created by the Galera controller
	Replicas int32 `json:"replicas"`

	// Members are the Galera members in the cluster
	Members MembersStatus `json:"members"`

	// CurrentRevision, if not empty, indicates the version of the Galera used to generate Pods for the
	// running Galera cluster
	CurrentRevision string `json:"currentRevision"`

	// CurrentReplicas is the number of Pods created by the Galera controller from the Galera version
	// indicated by currentRevision
	CurrentReplicas int32 `json:"currentReplicas"`

	// NextRevision, if not empty, indicates the version of the Galera used to generate Pods when
	// updating or upgrading
	NextRevision string `json:"nextRevision"`

	// NextReplicas is the number of Pods created by the Galera controller from the Galera version
	// indicated by nextRevision
	NextReplicas int32 `json:"nextReplicas"`

	// collisionCount is the count of hash collisions for the Galera. The Galera controller
	// uses this field as a collision avoidance mechanism when it needs to create the name for the
	// newest ControllerRevision
	// +optional
	CollisionCount *int32 `json:"collisionCount"`

	// Condition keeps track of all cluster conditions, if they exist.
	Conditions []GaleraCondition `json:"conditions"`
}

// GaleraCondition represents one current condition of a Galera cluster.
// A condition might not show up if it is not happening.
// For example, if a cluster is not upgrading, the Upgrading condition would not show up.
// If a cluster is upgrading and encountered a problem that prevents the upgrade,
// the Upgrading condition's status will would be False and communicate the problem back.
type GaleraCondition struct {
	// Type of Galera condition
	Type GaleraConditionType `json:"type"`

	// Status of the condition, one of True, False, Unknown
	Status corev1.ConditionStatus `json:"status"`

	// The last time this condition was updated.
	//LastUpdateTime string			`json:"lastUpdateTime,omitempty"`

	// Last time the condition transitioned from one status to another
	LastTransitionTime metav1.Time `json:"lastTransitionTime"`

	// The reason for the condition's last transition
	Reason string `json:"reason"`

	// A human readable message indicating details about the transition
	Message string `json:"message"`
}

type MembersStatus struct {
	// Ready are the Galera members that are ready to serve requests
	// The member names are the same as the Galera pod names
	Ready []string `json:"ready"`

	// Unready are the Galera members not ready to serve requests
	Unready []string `json:"unready"`

	// writer is the current Galera member designated to write
	Writer string `json:"writer"`

	// BackupWriter is a Galera member designated to backup the Writer member
	BackupWriter string `json:"backupWriter"`

	// Backup is a Galera member with a dedicated drive space used to process
	// galera backup & restore
	Backup string `json:"backup"`

	// Special is a Galera member with a special configuration, it is used
	// for batch or high consummation requests
	Special string `json:"special,omitempty"`
}

func (gs *GaleraStatus) IsCreated() bool {
	return gs != nil
}

func (gs *GaleraStatus) IsRunning() bool {
	if gs == nil {
		return false
	}
	return gs.Phase == GaleraPhaseRunning
}

func (gs *GaleraStatus) IsReady() bool {
	if gs == nil {
		return false
	}
	if gs.Phase == GaleraPhaseRunning {
		_, cp := getGaleraCondition(gs, GaleraConditionReady)
		if cp != nil {
			return true
		}
	}
	return false
}

func (gs *GaleraStatus) IsFailed() bool {
	if gs == nil {
		return false
	}
	return gs.Phase == GaleraPhaseFailed
}

func (gs *GaleraStatus) IsUpgrading() bool {
	if gs == nil {
		return false
	}
	_, cp := getGaleraCondition(gs, GaleraConditionUpgrading)
	if cp == nil {
		return false
	} else {
		return true
	}
}

func (gs *GaleraStatus) SetScalingUpCondition(from, to int32) {
	c := newGaleraCondition(GaleraConditionScaling, corev1.ConditionTrue, "Scaling up", scalingMsg(from, to))
	gs.setGaleraCondition(*c)
}

func (gs *GaleraStatus) SetScalingDownCondition(from, to int32) {
	c := newGaleraCondition(GaleraConditionScaling, corev1.ConditionTrue, "Scaling down", scalingMsg(from, to))
	gs.setGaleraCondition(*c)
}

func (gs *GaleraStatus) SetFailedCondition() {
	c := newGaleraCondition(GaleraConditionFailed, corev1.ConditionTrue,
		"Disaster recovery", "Majority is down. Recovering from backup")
	gs.setGaleraCondition(*c)

	gs.ClearCondition(GaleraConditionReady)
}

func (gs *GaleraStatus) SetUpgradingCondition(to string) {
	// TODO: show x/y members has upgraded.
	c := newGaleraCondition(GaleraConditionUpgrading, corev1.ConditionTrue,
		"Cluster upgrading", "upgrading to "+to)
	gs.setGaleraCondition(*c)
}

func (gs *GaleraStatus) SetReadyCondition() {
	c := newGaleraCondition(GaleraConditionReady, corev1.ConditionTrue, "Cluster ready", "")
	gs.setGaleraCondition(*c)
}

func (gs *GaleraStatus) SetRestoringCondition(restore string) {
	c := newGaleraCondition(GaleraConditionRestoring, corev1.ConditionTrue, "Restoring cluster", "restoring from "+restore)
	gs.setGaleraCondition(*c)
}

func (gs *GaleraStatus) ClearCondition(t GaleraConditionType) {
	pos, _ := getGaleraCondition(gs, t)
	if pos == -1 {
		return
	}
	gs.Conditions = append(gs.Conditions[:pos], gs.Conditions[pos+1:]...)
}

func (gs *GaleraStatus) setGaleraCondition(c GaleraCondition) {
	pos, cp := getGaleraCondition(gs, c.Type)
	if cp != nil && cp.Status == c.Status && cp.Reason == c.Reason && cp.Message == c.Message {
		return
	}

	if cp != nil {
		gs.Conditions[pos] = c
	} else {
		gs.Conditions = append(gs.Conditions, c)
	}
}

func getGaleraCondition(status *GaleraStatus, t GaleraConditionType) (int, *GaleraCondition) {
	for i, c := range status.Conditions {
		if t == c.Type {
			return i, &c
		}
	}
	return -1, nil
}

func newGaleraCondition(condType GaleraConditionType, status corev1.ConditionStatus, reason, message string) *GaleraCondition {
//	now := time.Now().Format(time.RFC3339)
	return &GaleraCondition{
		Type:               condType,
		Status:             status,
//		LastUpdateTime:     now,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	}
}

func scalingMsg(from, to int32) string {
	return fmt.Sprintf("Current cluster size: %d, desired cluster size: %d", from, to)
}