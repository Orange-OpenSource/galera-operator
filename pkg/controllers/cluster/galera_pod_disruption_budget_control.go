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
	apigalera "galera-operator/pkg/apis/apigalera/v1beta2"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	clientset "k8s.io/client-go/kubernetes"
	policylisters "k8s.io/client-go/listers/policy/v1beta1"
	"k8s.io/client-go/tools/record"
	"reflect"
)

// GaleraPodDisruptionBudgetControlInterface defines the interface that GaleraController uses to create, update,
// and delete PodDisruptionBudget used by Galera clusters. It is implemented as an interface to provide for testing fakes.
type GaleraPodDisruptionBudgetControlInterface interface {
	// CreateOrUpdateGaleraPDB create and update a PodDisruptionBudget for a Galera.
	// If the returned error is nil the PodDisruptionBudget have been created.
	CreateOrUpdateGaleraPDB(galera *apigalera.Galera) (string, error)
}

func NewRealGaleraPodDisruptionBudgetControl(
	client clientset.Interface,
	pdbLister policylisters.PodDisruptionBudgetLister,
	recorder record.EventRecorder,
) GaleraPodDisruptionBudgetControlInterface {
	logger := logrus.WithField("pkg", "controller")
	return &realGaleraPodDisruptionBudgetControl{logger,client, pdbLister, recorder}
}

// realGaleraPodDisruptionBudgetControl implements GaleraPodDisruptionBudgetControlInterface using a clientset.Interface
// to communicate with the  API server. The struct is package private as the internal details are irrelevant to
// importing packages.
type realGaleraPodDisruptionBudgetControl struct {
	logger    *logrus.Entry
	client    clientset.Interface
	pdbLister policylisters.PodDisruptionBudgetLister
	recorder  record.EventRecorder
}

func (gpc *realGaleraPodDisruptionBudgetControl) CreateOrUpdateGaleraPDB(galera *apigalera.Galera) (string, error) {
	pdbName := getPDBName(galera.Name)
	pdb := newGaleraPodDisruptionBudget(galera)
	curPdb, err := gpc.pdbLister.PodDisruptionBudgets(galera.Namespace).Get(pdbName)

	if apierrors.IsNotFound(err) {
		gpc.logger.Infof("Creating a new PodDisruptionBudget for cluster %s called %s", galera.Name, pdbName)
		_, err = gpc.client.PolicyV1beta1().PodDisruptionBudgets(galera.Namespace).Create(pdb)
	} else {
		if !reflect.DeepEqual(curPdb.Labels, pdb.Labels) {
			curPdb.Labels = pdb.Labels
			_, err = gpc.client.PolicyV1beta1().PodDisruptionBudgets(galera.Namespace).Update(curPdb)
		}
	}
	return pdbName, err
}

func getPDBName(clusterName string) string {
	if len(clusterName) > apigalera.MaxPodDisruptionBudgetLength {
		clusterName = clusterName[:apigalera.MaxPodDisruptionBudgetLength]
	}
	return clusterName + apigalera.PodDisruptionBudgetSuffix
}

var _ GaleraPodDisruptionBudgetControlInterface = &realGaleraPodDisruptionBudgetControl{}