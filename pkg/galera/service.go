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
	apigalera "galera-operator/pkg/apis/apigalera/v1beta2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewGaleraService will create a new Kubernetes service for a Galera cluster
func NewGaleraService(galLabels map[string]string, clusterName, clusterNamespace, svcName, role string, owner metav1.OwnerReference) *corev1.Service {
	svc := newGaleraService(galLabels, clusterName, clusterNamespace, svcName, role)
	addOwnerRefToObject(svc.GetObjectMeta(), owner)
	return svc
}

// NewGaleraServiceMonitor will create a new Kubernetes service used to monitor all pods forming a Galera cluster
func NewGaleraServiceMonitor(galLabels map[string]string, clusterName, clusterNamespace, svcName string, port int32,owner metav1.OwnerReference) *corev1.Service {
	svc := newGaleraServiceMonitor(galLabels, clusterName, clusterNamespace, svcName, port)
	addOwnerRefToObject(svc.GetObjectMeta(), owner)
	return svc
}

// NewGaleraHeadlessService will create a new headless Kubernetes service for a Galera cluster
func NewGaleraHeadlessService(galLabels map[string]string, clusterName, clusterNamespace, svcName string, owner metav1.OwnerReference) *corev1.Service {
	svc := newGaleraHeadlessService(galLabels, clusterName, clusterNamespace, svcName)
	addOwnerRefToObject(svc.GetObjectMeta(), owner)
	return svc
}

func newGaleraService(galLabels map[string]string, clusterName, clusterNamespace, svcName, role string) *corev1.Service {
	labels := serviceSelectorForGalera(clusterName, clusterNamespace, role)
	mergeLabels(labels, galLabels)

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: svcName,
			Labels: galLabels,
		},
		Spec: corev1.ServiceSpec{
			Ports: galeraMySQLPorts(),
			Type: corev1.ServiceTypeClusterIP,
			Selector: labels,
		},
	}
}

func newGaleraServiceMonitor(galLabels map[string]string, clusterName, clusterNamespace, svcName string, port int32) *corev1.Service {
	labels := serviceSelectorForGalera(clusterName, clusterNamespace, "")
	mergeLabels(labels, galLabels)

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: svcName,
			Labels: galLabels,
		},
		Spec: corev1.ServiceSpec{
			Ports: galeraMonitorPorts(port),
			Type: corev1.ServiceTypeClusterIP,
			Selector: labels,
		},
	}
}

func newGaleraHeadlessService(galLabels map[string]string, clusterName, clusterNamespace, svcName string) *corev1.Service {
	labels := serviceSelectorForGalera(clusterName, clusterNamespace,"")
	mergeLabels(labels, galLabels)

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: svcName,
			Labels: galLabels,
		},
		Spec: corev1.ServiceSpec{
			Ports: galeraServicePorts(),
			Type: corev1.ServiceTypeClusterIP,
			ClusterIP: corev1.ClusterIPNone,
			Selector: labels,
		},
	}
}

func galeraMySQLPorts() []corev1.ServicePort {
	return []corev1.ServicePort{
		{
			Name:     "mysql",
			Port:     int32(apigalera.MySQLPort),
			Protocol: corev1.ProtocolTCP,
		},
	}
}

func galeraMonitorPorts(port int32) []corev1.ServicePort {
	return []corev1.ServicePort{
		{
			Name:     "metrics",
			Port:     port,
			Protocol: corev1.ProtocolTCP,
		},
	}
}

func galeraServicePorts() []corev1.ServicePort {
	return []corev1.ServicePort{
		{
			Name:     "mysql",
			Port:     int32(apigalera.MySQLPort),
			Protocol: corev1.ProtocolTCP,
		},
		{
			Name:     "sst",
			Port:     int32(apigalera.StateSnapshotTransfertPort),
			Protocol: corev1.ProtocolTCP,
		},
		{
			Name:     "ist",
			Port:     int32(apigalera.IncrementalStateTransferPort),
			Protocol: corev1.ProtocolTCP,
		},
		{
			Name:     "replication",
			Port:     int32(apigalera.GaleraReplicationPort),
			Protocol: corev1.ProtocolTCP,
		},
	}
}