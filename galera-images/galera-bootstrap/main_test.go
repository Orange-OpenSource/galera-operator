// Copyright 2019 Orange and/or its affiliates. All rights reserved
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

package main

import (
	"bytes"
	"io/ioutil"
	"os"
	"testing"
)

func TestModifyFile(t *testing.T) {
	//  Modify parameters
	err := os.Setenv("POD_IP", "10.20.30.40")
	if err != nil {
		t.Error("unable to set the env variable")
	}
	inputConf = "testdata/my.cnf"
	clusterName = "test-cluster"
	clusterAddresses = "10.20.30.41,10.20.30.42"
	nodeName = "test-node"
	user = "test-user"
	password = "test-password"
	clusterEnabled = false
	actual := modifyFile()
	expected, err := ioutil.ReadFile(inputConf+".golden")
	if err != nil {
		t.Error("unable to open output golden file")
	}
	if !bytes.Equal([]byte(*actual), expected) {
		t.Error("generated and golden are not equal for my.cnf")
	}
	err = os.Unsetenv("POD_IP")
	if err != nil {
		t.Error("unable to unset the env variable")
	}

	//  Insert parameters
	err = os.Setenv("POD_IP", "10.20.30.50")
	if err != nil {
		t.Error("unable to set the env variable")
	}
	inputConf = "testdata/my2.cnf"
	clusterName = "test-cluster2"
	clusterAddresses = "10.20.30.51,10.20.30.52"
	nodeName = "test-node2"
	user = "test-user2"
	password = "test-password2"
	clusterEnabled = true
	actual = modifyFile()
	expected, err = ioutil.ReadFile(inputConf+".golden")
	if err != nil {
		t.Error("unable to open output golden file")
	}
	if !bytes.Equal([]byte(*actual), expected) {
		t.Error("generated and golden are not equal for my2.cnf")
	}
	err = os.Unsetenv("POD_IP")
	if err != nil {
		t.Error("unable to unset the env variable")
	}
}