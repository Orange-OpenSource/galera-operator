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
	"galera-operator/test/e2e/e2eutil"
	"galera-operator/test/e2e/framework"
	"os"
	"testing"
)

func TestOneMemberRecovery(t *testing.T) {
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

	testGalera, err := e2eutil.CreateGalera(t, f.GaleraClient, f.Namespace, e2eutil.NewGalera("test-galera", galeraImage, f.Namespace, testSecret.Name, testConfigMap.Name, replicas))
	if err != nil {
		t.Fatalf("failed to create galera cluster: %v", err)
	}

	defer func() {
		if err := e2eutil.DeleteGalera(t, f.GaleraClient, f.KubeClient, testGalera); err != nil {
			t.Fatalf("failed to delete galera cluster: %v", err)
		}

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

	e2eutil.LogfWithTimestamp(t, "adding data on one node")
	if err := e2eutil.AddData(f.KubeClient, f.Config, testGalera); err != nil {
		t.Fatalf("failed to add data to galera %s/%s", testGalera.Namespace, testGalera.Name)
	}

	// kill a random pod
	if err := e2eutil.KillGaleraNode(f.KubeClient, f.Namespace, replicas); err != nil {
		t.Fatalf("failed to kill a pod: %v", err)
	}

	if _, err := e2eutil.WaitUntilSizeReached(t, f.GaleraClient, replicas, 12, testGalera); err != nil {
		t.Fatalf("failed to recover to %d members galera cluster: %v", replicas, err)
	}

	e2eutil.LogfWithTimestamp(t, "checking data on all nodes")
	ok, err := e2eutil.CheckData(f.KubeClient, f.Config, testGalera)
	if err != nil || ok == false {
		t.Fatalf("failed to check data : %s", err)
	}
}
