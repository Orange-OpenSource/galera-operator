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

package galera

import (
	"testing"
	apigalera "galera-operator/pkg/apis/apigalera/v1beta2"
)

func TestPodLabelsForGalera(t *testing.T) {
	labels := make(map[string]string)

	labels["foo"] = "bar"
	name := "test-gal"
	namespace := "default"
	role := apigalera.RoleWriter
	revision := "59889f974c"
	state := apigalera.StateCluster

	retLabels := PodLabelsForGalera(name, namespace, role, revision, state)

	if retLabels["foo"] != "bar" {
		t.Errorf("Expected to find the input values [foo = bar]")
	}
	if retLabels[apigalera.GaleraClusterName] != name {
		t.Errorf("Expected to find the name")
	}
	if retLabels[apigalera.GaleraClusterNamespace] != namespace {
		t.Errorf("Expected to find the namespace")
	}
	if retLabels[apigalera.GaleraStateLabel] != state {
		t.Errorf("Expected to find the state")
	}
	if retLabels[apigalera.GaleraRevisionLabel] != revision {
		t.Errorf("Expected to find the revision")
	}
	if retLabels[apigalera.GaleraRoleLabel] != role {
		t.Errorf("Expected to find the role")
	}
	if retLabels[apigalera.GaleraReaderLabel] != "false" {
		t.Errorf("Expected to find false for the ReaderLabel")
	}
	if size := len(retLabels); size != 6 {
		t.Errorf("Expected a size of 6, got a size of %d", size)
	}
}