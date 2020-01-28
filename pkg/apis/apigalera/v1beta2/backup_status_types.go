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

// BackupConditionType represents a valid condition of a Backup.
type BackupConditionType string

const (
	// BackupScheduled means the Backup is a scheduled backup triggered by spec.schedule
	BackupScheduled BackupConditionType = "Scheduled"
	// BackupRunning means the Backup is currently being executed
	BackupRunning BackupConditionType = "Running"
	// BackupComplete means the Backup has successfully executed and the
	// resulting artifact has been stored in object storage.
	BackupComplete BackupConditionType = "Complete"
	// BackupFailed means the Backup has failed.
	BackupFailed BackupConditionType = "Failed"
)

// GaleraBackupStatus captures the current status of a GaleraBackup.
type GaleraBackupStatus struct {
	// OutcomeLocation holds the results of a the last successful backup.
	// +optional
	OutcomeLocation string `json:"outcomeLocation"`
	// TimeStarted is the time at which the backup was started.
	// +optional
	TimeStarted metav1.Time `json:"timeStarted"`
	// TimeCompleted is the time at which the backup completed.
	// +optional
	TimeCompleted metav1.Time `json:"timeCompleted"`
	// +optional
	Conditions []BackupCondition
}

// BackupCondition describes the observed state of a Backup at a certain point.
type BackupCondition struct {
	Type BackupConditionType
	Status corev1.ConditionStatus
	// +optional
	LastTransitionTime metav1.Time
	// +optional
	Reason string
	// +optional
	Message string
}


func getGaleraBackupCondition(status *GaleraBackupStatus, t BackupConditionType) (int, *BackupCondition) {
	for i, c := range status.Conditions {
		if t == c.Type {
			return i, &c
		}
	}
	return -1, nil
}

func newGaleraBackupCondition(condType BackupConditionType, status corev1.ConditionStatus, reason, message  string) *BackupCondition {
	return &BackupCondition{
		Type: 				condType,
		Status: 			status,
		LastTransitionTime: metav1.Now(),
		Reason: 			reason,
		Message: 			message,
	}
}

func (gbs *GaleraBackupStatus) setGaleraBackupCondition(c BackupCondition) {
	pos, cp := getGaleraBackupCondition(gbs, c.Type)
	if cp != nil && cp.Status == c.Status && cp.Reason == c.Reason && cp.Message == c.Message {
		return
	}

	if cp != nil {
		gbs.Conditions[pos] = c
	} else {
		gbs.Conditions = append(gbs.Conditions, c)
	}
}

func (gbs *GaleraBackupStatus) SetScheduledCondition() {
	c := newGaleraBackupCondition(BackupScheduled, corev1.ConditionTrue, "backup scheduler", "")
	gbs.setGaleraBackupCondition(*c)
}

func (gbs *GaleraBackupStatus) SetRunningCondition(pod, namespace string) {
	c := newGaleraBackupCondition(BackupRunning, corev1.ConditionTrue, "backup running", fmt.Sprintf("backup running on pod %s/%s", namespace, pod))
	gbs.setGaleraBackupCondition(*c)
}

func (gbs *GaleraBackupStatus) SetCompleteCondition() {
	c := newGaleraBackupCondition(BackupComplete, corev1.ConditionTrue, "backup completed", "")
	gbs.setGaleraBackupCondition(*c)
}

func (gbs *GaleraBackupStatus) SetFailedCondition() {
	c := newGaleraBackupCondition(BackupFailed, corev1.ConditionTrue, "backup failed", "")
	gbs.setGaleraBackupCondition(*c)
}

func (gbs *GaleraBackupStatus) ClearCondition(t BackupConditionType) {
	pos, _ := getGaleraBackupCondition(gbs, t)
	if pos == -1 {
		return
	}
	gbs.Conditions =  append(gbs.Conditions[:pos], gbs.Conditions[pos+1:]...)
}