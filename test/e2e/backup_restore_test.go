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

package e2e

import (
	"fmt"
	apigalera "galera-operator/pkg/apis/apigalera/v1beta2"
	"galera-operator/test/e2e/e2eutil"
	"galera-operator/test/e2e/framework"
	"galera-operator/test/e2e/retryutil"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
	"testing"
	"time"
)

var bkpName string

func TestBackupS3Mariabackup(t *testing.T) {
	galeraImage := os.Getenv(envImage)

	f := framework.Global
	testConfigMap, err := e2eutil.CreateConfigMap(t, f.KubeClient, f.Namespace, e2eutil.NewConfigMapForGalera("test-galera-configmap", f.Namespace))
	if err != nil {
		t.Fatalf("failed to create configMap: %v", err)
	}

	testSecret, err := e2eutil.CreateSecret(t, f.KubeClient, f.Namespace, e2eutil.NewSecretForGalera("test-galera-secret", f.Namespace))
	if err != nil {
		t.Fatalf("failed to create secret: %v", err)
	}

	replicas := 3

	e2eutil.LogfWithTimestamp(t, "creating %d nodes galera cluster", replicas)
	testGalera, err := e2eutil.CreateGalera(t, f.GaleraClient, f.Namespace, e2eutil.NewGalera("test-galera", galeraImage, f.Namespace, testSecret.Name, testConfigMap.Name, replicas))
	if err != nil {
		t.Fatalf("failed to create galera cluster: %v", err)
	}

	defer func() {
		if err := e2eutil.DeleteConfigMap(t, f.KubeClient, testConfigMap); err != nil {
			t.Fatalf("failed to delete configMap: %v", err)
		}

		if err := e2eutil.DeleteSecret(t, f.KubeClient, testSecret); err != nil {
			t.Fatalf("failed to delete secret: %v", err)
		}
	}()

	if _, err := e2eutil.WaitUntilSizeReached(t, f.GaleraClient, replicas, 12, testGalera); err != nil {
		t.Fatalf("failed to create %d members galera cluster: %v", replicas, err)
	}
	e2eutil.LogfWithTimestamp(t, "galera cluster created, %d nodes are ready", replicas)

	logrus.Infof("adding data on one node")
	if err := e2eutil.AddData(f.KubeClient, f.Config, testGalera); err != nil {
		t.Fatalf("failed to add data to galera %s/%s", testGalera.Namespace, testGalera.Name)
	}

	id := os.Getenv(envS3Id)
	secret := os.Getenv(envS3Secret)
	endpoint := os.Getenv(envS3Endpoint)
	bucket := os.Getenv(envS3Bucket)

	testSecretS3, err := e2eutil.CreateSecret(t, f.KubeClient, f.Namespace, e2eutil.NewSecretForGaleraBackupS3("test-s3-secret", f.Namespace, id, secret))
	if err != nil {
		t.Fatalf("failed to create S3 secret: %v", err)
	}

	e2eutil.LogfWithTimestamp(t, "creating galera backup using mariabackup and S3 storage")
	backup := e2eutil.NewGaleraBackupS3Mariabackup("test-galerabackup", f.Namespace, testGalera.Name, testSecret.Name, endpoint, bucket, testSecretS3.Name)
	testGaleraBackup, err := e2eutil.CreateGaleraBackup(t, f.GaleraClient, f.Namespace, backup)
	if err != nil {
		t.Fatalf("failed to create galera backup: %v", err)
	}

	defer func() {
		if err := e2eutil.DeleteGaleraBackup(t, f.GaleraClient, testGaleraBackup); err != nil {
			t.Fatalf("failed to delete galera cluster: %v", err)
		}

		if err := e2eutil.DeleteSecret(t, f.KubeClient, testSecretS3); err != nil {
			t.Fatalf("failed to delete s3 secret: %v", err)
		}
	}()

	err = retryutil.Retry(10*time.Second, 3, func() (bool, error) {
		curGB, err := f.GaleraClient.SqlV1beta2().GaleraBackups(f.Namespace).Get(testGaleraBackup.Name, metav1.GetOptions{})
		if err != nil {
			return false, fmt.Errorf("failed to retrieve galera backup CR: %v", err)
		}
		for _, c := range curGB.Status.Conditions {
			if c.Type == apigalera.BackupComplete {
				return true, nil
			}
			if c.Type == apigalera.BackupFailed {
				return false, fmt.Errorf("backup failed with reason: %v ", c.Reason)
			}
		}
		return false, nil
	})
	if err != nil {
		t.Fatalf("failed to verify backup: %v", err)
	}

	curGB, err := f.GaleraClient.SqlV1beta2().GaleraBackups(f.Namespace).Get(testGaleraBackup.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to retrieve galera backup CR: %v", err)
	}

	bkpName = curGB.Status.OutcomeLocation

	e2eutil.LogfWithTimestamp(t, "backup completed %s/%s, deleting galera cluster", bucket, bkpName)
	if err := e2eutil.DeleteGalera(t, f.GaleraClient, f.KubeClient, testGalera); err != nil {
		t.Fatalf("failed to delete galera cluster: %v", err)
	}
}

func TestRestoreS3Mariabackup(t *testing.T) {
	galeraImage := os.Getenv(envImage)

	f := framework.Global
	testConfigMap, err := e2eutil.CreateConfigMap(t, f.KubeClient, f.Namespace, e2eutil.NewConfigMapForGalera("test-restore-configmap", f.Namespace))
	if err != nil {
		t.Fatalf("failed to create configMap: %v", err)
	}

	testSecret, err := e2eutil.CreateSecret(t, f.KubeClient, f.Namespace, e2eutil.NewSecretForGalera("test-restore-secret", f.Namespace))
	if err != nil {
		t.Fatalf("failed to create secret: %v", err)
	}

	defer func() {
		if err := e2eutil.DeleteConfigMap(t, f.KubeClient, testConfigMap); err != nil {
			t.Fatalf("failed to delete configMap: %v", err)
		}

		if err := e2eutil.DeleteSecret(t, f.KubeClient, testSecret); err != nil {
			t.Fatalf("failed to delete secret: %v", err)
		}
	}()

	replicas := 3
	id := os.Getenv(envS3Id)
	secret := os.Getenv(envS3Secret)
	endpoint := os.Getenv(envS3Endpoint)
	bucket := os.Getenv(envS3Bucket)

	testSecretS3, err := e2eutil.CreateSecret(t, f.KubeClient, f.Namespace, e2eutil.NewSecretForGaleraBackupS3("test-s3-secret", f.Namespace, id, secret))
	if err != nil {
		t.Fatalf("failed to create S3 secret: %v", err)
	}

	e2eutil.LogfWithTimestamp(t, "restoring galera cluster")
	restoreGalera, err := e2eutil.CreateGalera(t, f.GaleraClient, f.Namespace, e2eutil.NewRestoreS3("test-restore", galeraImage, f.Namespace, testSecret.Name, testConfigMap.Name, bkpName, endpoint, bucket, testSecretS3.Name, replicas))
	if err != nil {
		t.Fatalf("failed to restore galera cluster: %v", err)
	}

	defer func() {
		if err := e2eutil.DeleteGalera(t, f.GaleraClient, f.KubeClient, restoreGalera); err != nil {
			t.Fatalf("failed to delete restore galera cluster: %v", err)
		}

		if err := e2eutil.DeleteSecret(t, f.KubeClient, testSecretS3); err != nil {
			t.Fatalf("failed to delete s3 secret: %v", err)
		}
	}()

	// Restore phase
	e2eutil.LogfWithTimestamp(t, "creating %d members galera cluster", replicas)
	if _, err := e2eutil.WaitUntilSizeReached(t, f.GaleraClient, replicas, 12, restoreGalera); err != nil {
		t.Fatalf("failed to create %d members galera cluster: %v", replicas, err)
	}

	e2eutil.LogfWithTimestamp(t, "waiting running phase")
	if err = e2eutil.WaitPhaseReached(t, f.GaleraClient, 12, restoreGalera, apigalera.GaleraPhaseRunning); err != nil {
		t.Fatalf("failed to reach running phase for galera %s/%s: %v", restoreGalera.Namespace, restoreGalera.Name, err)
	}

	// Running phase
	if _, err := e2eutil.WaitUntilSizeReached(t, f.GaleraClient, replicas, 12, restoreGalera); err != nil {
		t.Fatalf("failed to have %d members galera cluster: %v", replicas, err)
	}

	e2eutil.LogfWithTimestamp(t, "checking data on all nodes")
	ok, err := e2eutil.CheckData(f.KubeClient, f.Config, restoreGalera)
	if err != nil || ok == false {
		t.Fatalf("failed to check data : %s", err)
	}
}
