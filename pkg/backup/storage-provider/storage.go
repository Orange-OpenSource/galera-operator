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

package storage_provider

import (
	apigalera "galera-operator/pkg/apis/apigalera/v1beta2"
	"galera-operator/pkg/backup/storage-provider/s3"
	corev1 "k8s.io/api/core/v1"
)

// Interface abstracts the underlying storage provider.
type Interface interface {
	// Store creates a new object in the underlying provider's datastore if it does not exist,
	// or replaces the existing object if it does exist.
	Store(backupDir,key string) error
	// Retrieve return the object in the underlying provider's datastore if it exists.
	Retrieve(key, restoreDir string) error
}

// NewStorageProviderS3 accepts a secret map and uses its contents to determine the
// desired object storage provider implementation.
func NewStorageProviderS3(config apigalera.StorageProvider, credentials map[string]string, backupPod *corev1.Pod) (Interface, error) {
	return s3.NewProvider(config.S3, credentials, backupPod)
}
