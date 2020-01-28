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

package s3

import (
	"bytes"
	"encoding/json"
	"fmt"
	apigalera "galera-operator/pkg/apis/apigalera/v1beta2"
	bkpconstants "galera-operator/pkg/backup/constants"
	"galera-operator/pkg/utils/constants"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"io/ioutil"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"net/http"
	"time"
)

// Provider is storage implementation of provider.Interface.
type Provider struct {
	logger *logrus.Entry
	backupPod *corev1.Pod
	s3 *S3
}

type S3 struct {
	AccessKey string `json:"accessKey"`
	SecretKey string `json:"secretKey"`
	BackupDir string `json:"backupDir"`
	BackupName string `json:"backupName"`
	Endpoint string `json:"endpoint"`
	Bucket string `json:"bucket"`
	Region string `json:"region,omitempty"`
	ForcePathStyle bool `json:"forcePathStyle,omitempty"`
}

type httpResponse struct {
	Status string `json:"status"`
	Message string `json:"message"`
}

var netClient = &http.Client{
	Timeout: time.Second * 10,
}

// NewProvider creates a new S3 (compatible) storage provider.
func NewProvider(provider *apigalera.S3StorageProvider, credentials map[string]string, backupPod *corev1.Pod) (*Provider, error) {
	logger := logrus.WithField("pkg", "backupcontroller")

	accessKey, secretKey, err := getCredentials(credentials)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	newS3 := S3{
		AccessKey:accessKey,
		SecretKey:secretKey,
		Endpoint:provider.Endpoint,
		Bucket:provider.Bucket,
	}

	if provider.Region != "" {
		newS3.Region = provider.Region
	}

	if provider.ForcePathStyle != true {
		newS3.ForcePathStyle = provider.ForcePathStyle
	}

	return &Provider{
		logger:		logger,
		backupPod:	backupPod,
		s3:			&newS3,
	}, nil
}

// Store the given data at the given key.
func (p *Provider) Store(backupDir, key string) error {
	p.logger.Infof("Storing backup (provider=S3, endpoint=%s, bucket=%s, key=%s)", p.s3.Endpoint, p.s3.Bucket, key)

//	key = "toto"
	logrus.Infof("SEB: backupdir = %s", backupDir)

	p.s3.BackupDir = backupDir
	p.s3.BackupName = key

	logrus.Infof("SEB: S3 store = %#v", p.s3)

	data, err := json.Marshal(p.s3)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("http://%s:8080/s3", p.backupPod.Status.PodIP)

	logrus.Infof("SEB: url = %s", url)

	resp, err := netClient.Post(url, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return err
	}

	defer resp.Body.Close()
	buffer, err := ioutil.ReadAll(resp.Body)

	var response httpResponse

	err = json.Unmarshal(buffer, &response)
	if err != nil {
		return err
	}

	p.logger.Infof("response post (backup) : code: %d message: %s", resp.StatusCode, response.Message)

	url = fmt.Sprintf("http://%s:8080/state/%s", p.backupPod.Status.PodIP, key)

	logrus.Infof("SEB: url de check = %s", url)

	// wait the backup to finnish
	for {
		resp, err := netClient.Get(url)
		if err != nil {
			p.logger.Infof("error : GET %s : %s", url, err)
			continue
		}
		defer resp.Body.Close()
		buffer, err := ioutil.ReadAll(resp.Body)
		//		logrus.Infof("BODY : %s", string(buffer))

		var response httpResponse

		err = json.Unmarshal(buffer, &response)
		if err != nil {
			p.logger.Infof("error : unmarshal %+v : %s", response, err)
			continue
		}

		logrus.Infof("SEB: code: %d / response state: %+v, response.Status=%s", resp.StatusCode, response, response.Status)

		if resp.StatusCode == 200 && response.Status == "completed" {
			p.logger.Infof("code: %d / response state: %+v", resp.StatusCode, response)
			break
		}

		logrus.Infof("SEB: waiting 1 second")
		time.Sleep(1 * time.Second)
	}

	return nil
}

// Retrieve the given key from S3 storage service.
func (p *Provider) Retrieve(key, restoreDir string) error {
	p.logger.Infof("Retrieving backup (provider=S3, endpoint=%s, bucket=%s, key=%s)", p.s3.Endpoint, p.s3.Bucket, key)

	p.s3.BackupName = key
	p.s3.BackupDir = restoreDir

	logrus.Infof("SEB: S3 restore = %#v", p.s3)

	urlRestore := fmt.Sprintf("http://%s:8080/s3", p.backupPod.Status.PodIP)
	urlCheck := fmt.Sprintf("http://%s:8080/state/%s", p.backupPod.Status.PodIP, key)

	logrus.Infof("///////////////////////// ICI ////////////////////")


	state, err := p.getState(urlCheck)
	if err != nil {
		logrus.Infof("SEB: error when running")
		return err
	}

	logrus.Infof("SEB: retour de getState : %s", state)

	if state != bkpconstants.StateRunning {

		logrus.Infof("SEB: running")

		dataS3, err := json.Marshal(p.s3)
		if err != nil {
			return err
		}

		// build restore request
		restoreReq, err := http.NewRequest("GET", urlRestore, bytes.NewBuffer(dataS3))
		if err != nil {
			return err
		}

		logrus.Infof("SEB: restore req : +#v", restoreReq)

		// set headers
		restoreReq.Header.Set("Content-Type", "application/json")

		// do restore request
		restoreResp, err := netClient.Do(restoreReq)
		if err != nil {
			logrus.Infof("SEB: restore response fail : %+v", restoreResp)
			return err
		}

		defer restoreResp.Body.Close()
		buffer, err := ioutil.ReadAll(restoreResp.Body)
		var restoreBody httpResponse
		err = json.Unmarshal(buffer, &restoreBody)
		if err != nil {
			return err
		}

		p.logger.Infof("response get (restore) : code: %d /  message : %s", restoreResp.StatusCode, restoreBody.Message)
	}

	/*
	resp, err := netClient.Get(urlCheck)
	if err != nil {
		p.logger.Infof("error : GET %s : %s", urlCheck, err)
	} else {
		defer resp.Body.Close()
		buffer, err := ioutil.ReadAll(resp.Body)

		var response httpResponse

		err = json.Unmarshal(buffer, &response)
		if err != nil {
			p.logger.Infof("error : unmarshal %+v : %s", response, err)
		} else {
			logrus.Infof("SEB: code: %d / response state: %+v, response.Status=%s", resp.StatusCode, response, response.Status)

			if resp.StatusCode == 200 && response.Status == "completed" {
				p.logger.Infof("code: %d / response state: %+v", resp.StatusCode, response)
				return nil
			}
		}
	}
	*/




	/*
	// wait the restore to finnish
	for {
		resp, err := netClient.Get(url)
		if err != nil {
			p.logger.Infof("error : GET %s : %s", url, err)
			continue
		}
		defer resp.Body.Close()
		buffer, err := ioutil.ReadAll(resp.Body)

		var response httpResponse

		err = json.Unmarshal(buffer, &response)
		if err != nil {
			p.logger.Infof("error : unmarshal %+v : %s", response, err)
			continue
		}

		logrus.Infof("SEB: code: %d / response state: %+v, response.Status=%s", resp.StatusCode, response, response.Status)

		if resp.StatusCode == 200 && response.Status == "completed" {
			p.logger.Infof("code: %d / response state: %+v", resp.StatusCode, response)
			break
		}

		time.Sleep(1 * time.Second)
	}
	*/

	logrus.Infof("SEB: entering infinite loop")

	ticker := time.NewTicker(1000 * time.Millisecond)

	for {
		select {
		case <-ticker.C:
			state, err := p.getState(urlCheck)
			if err == nil {
				if state == bkpconstants.StateCompleted {
					return nil
				}
				if state == bkpconstants.StateFailed {
					return errors.New("restore failed")
				}
			} else {
				return err
			}
		case <-time.After(constants.TimeOut * time.Second):
			return errors.New(fmt.Sprintf("timeout of %d seconds reached when performing restore operation", constants.TimeOut))
		}
	}
}

// getState returns
func (p *Provider) getState(url string) (string, error) {
	resp, err := netClient.Get(url)
	if err != nil {
		p.logger.Infof("error : GET %s : %s", url, err)
		return "", err
	} else {
		defer resp.Body.Close()
		buffer, err := ioutil.ReadAll(resp.Body)

		var response httpResponse

		err = json.Unmarshal(buffer, &response)
		if err != nil {
			p.logger.Infof("error : unmarshal %+v : %s", response, err)
			return "", err
		} else {
			logrus.Infof("SEB:_____________ code: %d / response state: %+v", resp.StatusCode, response)

			return response.Status, nil
			/*
			if resp.StatusCode == 200 {
				p.logger.Infof("code: %d / response state: %+v", resp.StatusCode, response)
				// response.Status == "running" or "completed" or "failed"
				return response.Status, nil
			} else {
				return response.Status, errors.New(fmt.Sprintf("error with status %s and message %s", response.Status, response.Message))
			}
			*/

		}
	}
}

// getCredentials gets an accesskey and secretKey from the provided map.
func getCredentials(credentials map[string]string) (string, string, error) {
	allErrs := field.ErrorList{}
	fldPath := field.NewPath("data")

	if credentials == nil {
		return "", "", errors.New("no credentials provided")
	}

	accessKey, ok := credentials["accessKeyId"]
	if !ok {
		allErrs = append(allErrs, field.Required(fldPath.Child("accessKeyId"), ""))
	}
	secretKey, ok := credentials["secretAccessKey"]
	if !ok {
		allErrs = append(allErrs, field.Required(fldPath.Child("secretAccessKey"), ""))
	}

	if len(allErrs) > 0 {
		return "", "", allErrs.ToAggregate()
	}

	return accessKey, secretKey, nil
}
