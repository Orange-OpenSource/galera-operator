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
	method "galera-operator/pkg/backup/backup-method"
	storage "galera-operator/pkg/backup/storage-provider"
	corev1 "k8s.io/api/core/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"time"
)

const (
	backupDir = apigalera.BackupVolumeMountPath + "/new"
	restoreDir = apigalera.BackupVolumeMountPath
)


// BackupInterface is an interface used to implement Backup/Restore
type BackupInterface interface {
	Backup(backupName string) (string, error)
	Restore(backupName string) error
}

// BackupHandler implementations can execute backups and store them in storage
// backends.
type BackupHandler struct {
	method method.Interface
	storage storage.Interface
}

// NewBackupHandlerMariabackupS3 creates a BackupHandler configured with the mariabackup Backup/Restore target method and
// S3 storage configurations.
func NewBackupHandlerMariabackupS3(client clientset.Interface, config *rest.Config, backupPod *corev1.Pod, backupCreds map[string]string, provider apigalera.StorageProvider, mapCredS3 map[string]string) (BackupInterface, error) {
	m, err := method.NewMethodMariabackup(client, config, backupPod, backupCreds)
	if err != nil {
		return nil, err
	}

	s, err := storage.NewStorageProviderS3(provider, mapCredS3, backupPod)
	if err != nil {
		return nil, err
	}

	return &BackupHandler{method: m, storage: s}, nil
}

// NewBackupHandlerNoneS3 creates a BackupHandler configured with the none Backup/Restore target method and
// S3 storage configurations.
func NewBackupHandlerNoneS3(backupPod *corev1.Pod, provider apigalera.StorageProvider, mapCredS3 map[string]string) (BackupInterface, error) {
	m, err := method.NewMethodNone()
	if err != nil {
		return nil, err
	}

	s, err := storage.NewStorageProviderS3(provider, mapCredS3, backupPod)
	if err != nil {
		return nil, err
	}

	return &BackupHandler{method: m, storage: s}, nil
}


// Backup performs a backup using the method and then stores it using the storage provider.
func (bh *BackupHandler) Backup(backupName string) (string, error) {
	err := bh.method.Backup(backupDir)
	if err != nil {
		return "", err
	}

	backupFileName := fmt.Sprintf("%s.%s.sql.gz", backupName, time.Now().UTC().Format("20060102150405"))

	err = bh.storage.Store(backupDir, backupFileName)
	if err != nil {
		return "", err
	}

	return backupFileName, nil
}

// Restore performs a retrieve using the storage provider then a restore using
// the method provided.
func (bh *BackupHandler) Restore(backupName string) error {
	err := bh.storage.Retrieve(backupName, restoreDir)
	if err != nil {
		return err
	}

	return bh.method.Restore(backupDir)
}
