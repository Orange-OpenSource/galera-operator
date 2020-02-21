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

package cluster

import (
	"fmt"
	apigalera "galera-operator/pkg/apis/apigalera/v1beta2"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	corelisters "k8s.io/client-go/listers/core/v1"
)

// GaleraSecretControlInterface defines the interface that GaleraController uses to access Secret
// used by Galera clusters. It is implemented as an interface to provide for testing fakes.
type GaleraSecretControlInterface interface {
	// GetGaleraCreds returns a map containing user and password credentials to lunch mariadb commands.
	// If the returned error is nil, credentails ara valid.
	GetGaleraCreds(galera *apigalera.Galera) (map[string]string, error)
}

func NewRealGaleraSecretControl(secretLister corelisters.SecretLister) GaleraSecretControlInterface {
	logger := logrus.WithField("pkg", "controller")
	return &realGaleraSecretControl{logger, secretLister}
}

// realGaleraSecretControl implements GaleraSecretControlInterface using a clientset.Interface
// to communicate with the  API server. The struct is package private as the internal details are irrelevant to
// importing packages.
type realGaleraSecretControl struct {
	logger       *logrus.Entry
	secretLister corelisters.SecretLister
}

func (gsc *realGaleraSecretControl) GetGaleraCreds(galera *apigalera.Galera) (map[string]string, error) {

	creds, err := gsc.secretLister.Secrets(galera.Namespace).Get(galera.Spec.Pod.CredentialsSecret.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("galera '%s/%s' can not find CredentialsSecret %s", galera.Namespace, galera.Name, galera.Spec.Pod.CredentialsSecret.Name))
			return nil, err
		}
		utilruntime.HandleError(fmt.Errorf("unable to retrieve Galera '%s/%s' CredentialsSecret: %v", galera.Namespace, galera.Name, err))
		return nil, err
	}

	credsMap := make(map[string]string, len(creds.Data))
	for k, v := range creds.Data {
		credsMap[k] = string(v)
	}

	if credsMap["user"] == "" || credsMap["password"] == "" {
		err = fmt.Errorf("user and/or password are not set in secret %s", galera.Spec.Pod.CredentialsSecret.Name)
//	} else {
//		gsc.logger.Infof("successfully loaded user and password from secret for galera %s/%s", galera.Namespace, galera.Name)
	}

	return credsMap, err
}

var _ GaleraSecretControlInterface = &realGaleraSecretControl{}