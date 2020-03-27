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
	apigalera "galera-operator/pkg/apis/apigalera/v1beta2"
	"galera-operator/test/e2e/e2eutil"
	"galera-operator/test/e2e/framework"
	"os"
	"testing"
)

func TestCreateGaleraWithSpecial(t *testing.T) {
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
	newGalera := e2eutil.NewGalera("test-special", galeraImage, f.Namespace, testSecret.Name, testConfigMap.Name, replicas)
	newGalera = e2eutil.AddingSpecial(newGalera)

	testGalera, err := e2eutil.CreateGalera(t, f.GaleraClient, f.Namespace, newGalera)
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

	if _, err := e2eutil.WaitUntilSizeReached(t, f.GaleraClient, replicas+1, 12, testGalera); err != nil {
		t.Fatalf("failed to create %d members galera cluster: %v", replicas, err)
	}

	ok, err := e2eutil.MatchGaleraSize(f.KubeClient, f.Config, testGalera, replicas+1)
	if err != nil || ok == false {
		t.Fatalf("failed to have wsrep_cluster_size equals to %d : %v", replicas+1, err)
	}

	e2eutil.LogfWithTimestamp(t, "adding data on one node")
	if err := e2eutil.AddData(f.KubeClient, f.Config, testGalera); err != nil {
		t.Fatalf("failed to add data to galera %s/%s", testGalera.Namespace, testGalera.Name)
	}

	e2eutil.LogfWithTimestamp(t, "checking data on all nodes")
	ok, err = e2eutil.CheckData(f.KubeClient, f.Config, testGalera)
	if err != nil || ok == false {
		t.Fatalf("failed to check data : %s", err)
	}
}

func TestUpgradeGaleraWithSpecial(t *testing.T) {
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
	newGalera := e2eutil.NewGalera("test-special", galeraImage, f.Namespace, testSecret.Name, testConfigMap.Name, replicas)
	newGalera = e2eutil.AddingSpecial(newGalera)

	testGalera, err := e2eutil.CreateGalera(t, f.GaleraClient, f.Namespace, newGalera)
	if err != nil {
		t.Fatalf("failed to create galera cluster: %v", err)
	}

	defer func() {
		/*
		if err := e2eutil.DeleteGalera(t, f.GaleraClient, f.KubeClient, testGalera); err != nil {
			t.Fatalf("failed to delete galera cluster: %v", err)
		}
		*/

		if err := e2eutil.DeleteConfigMap(t, f.KubeClient, testConfigMap); err != nil {
			t.Fatalf("failed to delete configMap: %v", err)
		}

		if err := e2eutil.DeleteSecret(t, f.KubeClient, testSecret); err != nil {
			t.Fatalf("failed to delete secret: %v", err)
		}
	}()

	err = e2eutil.WaitSizeAndImageReached(t, f.KubeClient, galeraImage, replicas+1, 12, testGalera)
	if err != nil {
		t.Fatalf("failed to create %d nodes galera cluster: %v", replicas+1, err)
	}
	e2eutil.LogfWithTimestamp(t, "reached to %d galera nodes with image %s", replicas+1, galeraImage)

	e2eutil.LogfWithTimestamp(t, "adding data on one node")
	if err := e2eutil.AddData(f.KubeClient, f.Config, testGalera); err != nil {
		t.Fatalf("failed to add data to galera %s/%s", testGalera.Namespace, testGalera.Name)
	}

	galeraImageUpgrade := os.Getenv(envImageUpgrade)
	updateFunc := func(galera *apigalera.Galera) {
		galera = e2eutil.GaleraWithNewImage(galera, galeraImageUpgrade)
	}

	e2eutil.LogfWithTimestamp(t, "upgrading galera nodes with image %s", galeraImageUpgrade)
	if _, err = e2eutil.UpdateGalera(f.GaleraClient, testGalera.Name, testGalera.Namespace, 3, updateFunc); err != nil {
		t.Fatalf("failed to upgrade galera cluster")
	}

	if err := e2eutil.WaitSizeAndImageReached(t, f.KubeClient, galeraImageUpgrade, replicas+1, 20, testGalera); err != nil {
		t.Fatalf("failed to wait new image on each galera node: %v", err)
	}

	e2eutil.LogfWithTimestamp(t, "checking data on all nodes")
	ok, err := e2eutil.CheckData(f.KubeClient, f.Config, testGalera)
	if err != nil || ok == false {
		t.Fatalf("failed to check data : %s", err)
	}

	e2eutil.LogfWithTimestamp(t, "reached to %d galera nodes with image %s", replicas+1, galeraImageUpgrade)
}
