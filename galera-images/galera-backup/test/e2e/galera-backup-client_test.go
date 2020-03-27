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
	"bytes"
	"encoding/json"
	"galera-backup/pkg/models"
	"io/ioutil"
	"net/http"
	"os"
	"testing"
	"time"
)

var netClient = &http.Client{
	Timeout: time.Second * 10,
}

func TestProbe(t *testing.T) {
	GetProbe := "http://localhost:8080/probe"
	resp, _ := netClient.Get(GetProbe)
	defer resp.Body.Close()
	buffer, _ := ioutil.ReadAll(resp.Body)

	var response models.Response
	json.Unmarshal(buffer, &response)
	if resp.StatusCode != 200 && *response.Message != "Running" {
		t.Fatalf("error when performing liveness test %s", GetProbe)
	}
}

func TestGet(t *testing.T) {
	fakeGet := "http://localhost:8080/state/fake"
	resp, _ := netClient.Get(fakeGet)
	defer resp.Body.Close()
	buffer, _ := ioutil.ReadAll(resp.Body)

	var response models.Response
	json.Unmarshal(buffer, &response)
	if resp.StatusCode != 404 && *response.Status != "Unknown" {
		t.Fatalf("error when performing fake GET %s", fakeGet)
	}
}

func TestPostS3(t *testing.T) {
	accessKey :=  os.Getenv("GB_S3Login")
	secretKey := os.Getenv("GB_S3Password")
	endpoint := os.Getenv("GB_S3Endpoint")
	bucket := os.Getenv("GB_S3Bucket")
	backupName := os.Getenv("GB_S3BackupName")
	backup := os.Getenv("GB_Backup")

	s3 := models.S3{
		AccessKey: &accessKey,
		SecretKey: &secretKey,
		Endpoint: &endpoint,
		Bucket: &bucket,
		BackupName: &backupName,
		BackupDir: &backup,
	}

	data, _ := json.Marshal(s3)

	resp, _ := netClient.Post("http://localhost:8080/s3", "application/json", bytes.NewBuffer(data))
	defer resp.Body.Close()
	buffer, _ := ioutil.ReadAll(resp.Body)

	var response models.Response

	json.Unmarshal(buffer, &response)

	if resp.StatusCode != 200 && *response.Status != "scheduled" {
		t.Fatalf("error when performing POST S3")
	}
}

func TestGetS3(t *testing.T) {
	accessKey :=  os.Getenv("GB_S3Login")
	secretKey := os.Getenv("GB_S3Password")
	endpoint := os.Getenv("GB_S3Endpoint")
	bucket := os.Getenv("GB_S3Bucket")
	backupName := os.Getenv("GB_S3BackupName")
	restore := os.Getenv("GB_Restore")

	// First check if the previous POST is over
	for {
		resp, _ := netClient.Get("http://localhost:8080/state/"+backupName)
		defer resp.Body.Close()
		buffer, _ := ioutil.ReadAll(resp.Body)

		var response models.Response
		json.Unmarshal(buffer, &response)

		if resp.StatusCode != 200 {
			t.Fatalf("error when performing GET S3 state")
		}

		if resp.StatusCode == 200 && *response.Status == "completed" {
			break
		}

		time.Sleep(1 * time.Second)
	}

	url := "http://localhost:8080/s3"

	s3 := models.S3{
		AccessKey: &accessKey,
		SecretKey: &secretKey,
		Endpoint: &endpoint,
		Bucket: &bucket,
		BackupName: &backupName,
		BackupDir: &restore,
	}

	data, _ := json.Marshal(s3)

	restoreReq, _ := http.NewRequest("GET", url, bytes.NewBuffer(data))
	// set headers
	restoreReq.Header.Set("Content-Type", "application/json")

	restoreResp, _ := netClient.Do(restoreReq)
	defer restoreResp.Body.Close()

	buffer, _ := ioutil.ReadAll(restoreResp.Body)
	var response models.Response
	json.Unmarshal(buffer, &response)

	if restoreResp.StatusCode != 200 && *response.Status != "scheduled" {
		t.Fatalf("error when performing GET S3")
	}

	// wait the end of the backup
	for {
		resp, _ := netClient.Get("http://localhost:8080/state/"+backupName)
		defer resp.Body.Close()
		buffer, _ := ioutil.ReadAll(resp.Body)

		var response models.Response
		json.Unmarshal(buffer, &response)

		if resp.StatusCode != 200 {
			t.Fatalf("error when performing GET S3 state")
		}

		if resp.StatusCode == 200 && *response.Status == "completed" {
			break
		}

		time.Sleep(1 * time.Second)
	}
}