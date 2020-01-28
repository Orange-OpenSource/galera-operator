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

package backup

import (
	"fmt"
	apigalera "galera-operator/pkg/apis/apigalera/v1beta2"
	galeraclientset "galera-operator/pkg/client/clientset/versioned"
	listers "galera-operator/pkg/client/listers/apigalera/v1beta2"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/util/retry"
)

type GaleraBackupStatusUpdaterInterface interface {
	UpdateGaleraBackupStatus(backup *apigalera.GaleraBackup, status *apigalera.GaleraBackupStatus) error
}

func NewRealGaleraBackupStatusUpdater(
	client galeraclientset.Interface,
	galeraBackupLister listers.GaleraBackupLister) GaleraBackupStatusUpdaterInterface {
	return &realGaleraBackupStatusUpdater{client, galeraBackupLister}
}

type realGaleraBackupStatusUpdater struct {
	client galeraclientset.Interface
	galeraBackupLister listers.GaleraBackupLister
}
func (bsu *realGaleraBackupStatusUpdater) UpdateGaleraBackupStatus(
	backup *apigalera.GaleraBackup,
	status *apigalera.GaleraBackupStatus) error {
	// don't wait due to limited number of clients, but backoff after the default number of steps
	return retry.RetryOnConflict(retry.DefaultBackoff, func() error{
		backup.Status = *status
		_, updateErr := bsu.client.SqlV1beta2().GaleraBackups(backup.Namespace).UpdateStatus(backup)
		if updateErr == nil {
			return nil
		}
		if updated, err := bsu.galeraBackupLister.GaleraBackups(backup.Namespace).Get(backup.Name); err == nil {
			// make a copy so we don't mutate the shared cache
			backup = updated.DeepCopy()
		} else {
			utilruntime.HandleError(fmt.Errorf("error getting updated GaleraBackup %s/%s from lister: %v", backup.Namespace, backup.Name, err))
		}

		return updateErr
	})
}

var _ GaleraBackupStatusUpdaterInterface = &realGaleraBackupStatusUpdater{}
