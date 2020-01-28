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

package backup_method

import (
	"galera-operator/pkg/backup/backup-method/mariabackup"
	"galera-operator/pkg/backup/backup-method/none"
	corev1 "k8s.io/api/core/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Interface will execute backup operations via a tool such as mariabackup, xtrabackup or rsync
type Interface interface {
	// Backup runs a backup operation using the given credentials, returning the content.
	Backup(backupDir string) error
	// Restore restores the given content to the mysql node.
	Restore(backupDir string) error
}

func NewMethodMariabackup(client clientset.Interface, config *rest.Config, backupPod *corev1.Pod, backupCreds map[string]string) (Interface, error) {
	return mariabackup.NewMethod(client, config, backupPod, backupCreds)
}

func NewMethodNone() (Interface, error) {
	return none.NewMethod()
}