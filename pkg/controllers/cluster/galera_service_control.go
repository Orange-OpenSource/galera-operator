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
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/record"
)

// GaleraServiceControlInterface defines the interface that GaleraController uses to create, update, and delete
// Headless Service used by Galera clusters. It is implemented as an interface to provide for testing fakes.
type GaleraServiceControlInterface interface {
	// CreateOrUpdateGaleraServiceWriter create and update a ClusterIP Service used to read/write.
	// If the returned error is nil the Service have been created.
	CreateOrUpdateGaleraServiceWriter(galera *apigalera.Galera) (string, error)
	// CreateOrUpdateGaleraServiceWriterBackup create and update a ClusterIP Service used to read/write.
	// If the returned error is nil the Service have been created.
	CreateOrUpdateGaleraServiceWriterBackup(galera *apigalera.Galera) (string, error)
	// CreateOrUpdateGaleraServiceReader create and update a ClusterIP Service used to read.
	// If the returned error is nil the Service have been created.
	CreateOrUpdateGaleraServiceReader(galera *apigalera.Galera) (string, error)
	// CreateOrUpdateGaleraServiceSpecial create and update a ClusterIP Service used to reach a special member
	// of the galera cluster.
	// If the returned error is nil the Service have been created.
	CreateOrUpdateGaleraServiceSpecial(galera *apigalera.Galera) (string, error)
	// CreateOrUpdateGaleraServiceMonitor create and update a ClusterIP Service used to reach monitor agents.
	// This service is very useful combined with servicemonitor and prometheus operator
	// If the returned error is nil the Service have been created.
	CreateOrUpdateGaleraServiceMonitor(galera *apigalera.Galera) (string, error)
	// CreateOrUpdateGaleraServiceInternal create and update a Headless ClusterIP Service used between
	// Galera nodes to synchronise
	// If the returned error is nil the Service have been created.
	CreateOrUpdateGaleraServiceInternal(galera *apigalera.Galera) (string, error)
}

func NewRealGaleraServiceControl(
	client clientset.Interface,
	serviceLister corelisters.ServiceLister,
	recorder record.EventRecorder,
) GaleraServiceControlInterface {
	logger := logrus.WithField("pkg", "controller")
	return &realGaleraServiceControl{logger,client, serviceLister, recorder}
}

// realGaleraServiceControl implements GaleraServiceControlInterface using a clientset.Interface to communicate with the
// API server. The struct is package private as the internal details are irrelevant to importing packages.
type realGaleraServiceControl struct {
	logger        *logrus.Entry
	client        clientset.Interface
	serviceLister corelisters.ServiceLister
	recorder      record.EventRecorder
}

var _ GaleraServiceControlInterface = &realGaleraServiceControl{}

func (gsc *realGaleraServiceControl) CreateOrUpdateGaleraServiceWriter(galera *apigalera.Galera) (string, error) {
	svcName := getServiceName(galera.Name, apigalera.ServiceWriterSuffix, apigalera.MaxServiceWriterLength)
	_, err := gsc.serviceLister.Services(galera.Namespace).Get(svcName)
	// If the resource doesn't exist, we'll create it
	if apierrors.IsNotFound(err) {
		gsc.logger.Infof("Creating a new writer service for cluster %s/%s called %s", galera.Namespace, galera.Name, svcName)
		svc := newGaleraService(galera, svcName, apigalera.RoleWriter)
		_, err = gsc.client.CoreV1().Services(galera.Namespace).Create(svc)
	}
	return svcName, nil
}

func (gsc *realGaleraServiceControl) CreateOrUpdateGaleraServiceWriterBackup(galera *apigalera.Galera) (string, error) {
	svcName := getServiceName(galera.Name, apigalera.ServiceWriterBackupSuffix, apigalera.MaxServiceWriterBackupLength)
	_, err := gsc.serviceLister.Services(galera.Namespace).Get(svcName)
	// If the resource doesn't exist, we'll create it
	if apierrors.IsNotFound(err) {
		gsc.logger.Infof("Creating a new writer backup service for cluster %s/%s called %s", galera.Namespace, galera.Name, svcName)
		svc := newGaleraService(galera, svcName, apigalera.RoleBackupWriter)
		_, err = gsc.client.CoreV1().Services(galera.Namespace).Create(svc)
	}
	return svcName, err
}

func (gsc *realGaleraServiceControl) CreateOrUpdateGaleraServiceReader(galera *apigalera.Galera) (string, error) {
	svcName := getServiceName(galera.Name, apigalera.ServiceReaderSuffix, apigalera.MaxServiceReaderLength)
	_, err := gsc.serviceLister.Services(galera.Namespace).Get(svcName)
	// If the resource doesn't exist, we'll create it
	if apierrors.IsNotFound(err) {
		gsc.logger.Infof("Creating a new reader service for cluster %s/%s called %s", galera.Namespace, galera.Name, svcName)
		svc := newGaleraService(galera, svcName, apigalera.Reader)
		_, err = gsc.client.CoreV1().Services(galera.Namespace).Create(svc)
	}
	return svcName, err
}

func (gsc *realGaleraServiceControl) CreateOrUpdateGaleraServiceSpecial(galera *apigalera.Galera) (string, error) {
	svcName := getServiceName(galera.Name, apigalera.ServiceSpecialSuffix, apigalera.MaxServiceSpecialLength)
	_, err := gsc.serviceLister.Services(galera.Namespace).Get(svcName)
	// If the resource doesn't exist, we'll create it
	if apierrors.IsNotFound(err) {
		gsc.logger.Infof("Creating a new special service for cluster %s/%s called %s", galera.Namespace, galera.Name, svcName)
		svc := newGaleraService(galera, svcName, apigalera.RoleSpecial)
		_, err = gsc.client.CoreV1().Services(galera.Namespace).Create(svc)
	}
	return svcName, err
}

func (gsc *realGaleraServiceControl) CreateOrUpdateGaleraServiceMonitor(galera *apigalera.Galera) (string, error) {
	svcName := getServiceName(galera.Name, apigalera.ServiceMonitorSuffix, apigalera.MaxServiceMonitorLength)
	_, err := gsc.serviceLister.Services(galera.Namespace).Get(svcName)
	// If the resource doesn't exist, we'll create it
	if apierrors.IsNotFound(err) {
		gsc.logger.Infof("Creating a new monitor service for cluster %s/%s called %s", galera.Namespace, galera.Name, svcName)
		svc := newGaleraServiceMonitor(galera)
		_, err = gsc.client.CoreV1().Services(galera.Namespace).Create(svc)
	}
	return svcName, err
}

func (gsc *realGaleraServiceControl) CreateOrUpdateGaleraServiceInternal(galera *apigalera.Galera) (string, error) {
	svcName := getServiceName(galera.Name, apigalera.HeadlessServiceSuffix, apigalera.MaxHeadlessServiceLength)
	_, err := gsc.serviceLister.Services(galera.Namespace).Get(svcName)
	// If the resource doesn't exist, we'll create it
	if apierrors.IsNotFound(err) {
		gsc.logger.Infof("Creating a new internal service for cluster %s/%s called %s", galera.Namespace, galera.Name, svcName)
		svc := newGaleraHeadlessService(galera)
		_, err = gsc.client.CoreV1().Services(galera.Namespace).Create(svc)
	}
	return svcName, err
}

func getServiceName(clusterName, suffix string, max int) string {
	if len(clusterName) > max {
		clusterName = clusterName[:max]
	}
	return clusterName + suffix
}
