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

package e2eutil

import (
	"galera-operator/pkg/client/clientset/versioned"
	//	"galera-operator/test/e2e/retryutil"
	"k8s.io/client-go/kubernetes"
	"testing"
	apigalera "galera-operator/pkg/apis/apigalera/v1beta2"
	//	"time"
	corev1 "k8s.io/api/core/v1"
)

func CreateGalera(t *testing.T, galeraClient versioned.Interface, namespace string, galera *apigalera.Galera) (*apigalera.Galera, error) {
	res, err := galeraClient.SqlV1beta2().Galeras(namespace).Create(galera)
	if err != nil {
		return nil, err
	}
	t.Logf("creating galera cluster: %s", res.Name)

	return res, nil
}

func DeleteGalera(t *testing.T, galeraClient versioned.Interface, kubeClient kubernetes.Interface, galera *apigalera.Galera) error {
	t.Logf("deleting galera cluster: %v", galera.Name)
	err := galeraClient.SqlV1beta2().Galeras(galera.Namespace).Delete(galera.Name, nil)
	if err != nil {
		return err
	}
	return waitResourcesDeleted(t, kubeClient, galera)
}

func CreateConfigMap(t *testing.T, kubeClient kubernetes.Interface, namespace string, configMap *corev1.ConfigMap) (*corev1.ConfigMap, error) {
	res, err := kubeClient.CoreV1().ConfigMaps(namespace).Create(configMap)
	if err != nil {
		return nil, err
	}
	t.Logf("creating configMap for galera cluster: %s", res.Name)

	return res, nil
}

func DeleteConfigMap(t *testing.T, kubeClient kubernetes.Interface, configMap *corev1.ConfigMap) error {
	t.Logf("deleting configmap: %v", configMap.Name)
	return kubeClient.CoreV1().ConfigMaps(configMap.Namespace).Delete(configMap.Name, nil)
}

func CreateSecret(t *testing.T, kubeClient kubernetes.Interface, namespace string, secret *corev1.Secret) (*corev1.Secret, error) {
	res, err := kubeClient.CoreV1().Secrets(namespace).Create(secret)
	if err != nil {
		return nil, err
	}
	t.Logf("creating secret for galera cluster: %s", res.Name)

	return res, nil
}

func DeleteSecret(t *testing.T, kubeClient kubernetes.Interface, secret *corev1.Secret) error {
	t.Logf("deleting secret: %v", secret.Name)
	return kubeClient.CoreV1().Secrets(secret.Namespace).Delete(secret.Name, nil)
}


/*
func UpdateCluster(crClient versioned.Interface, cl *api.EtcdCluster, maxRetries int, updateFunc k8sutil.EtcdClusterCRUpdateFunc) (*api.EtcdCluster, error) {
	return AtomicUpdateClusterCR(crClient, cl.Name, cl.Namespace, maxRetries, updateFunc)
}

func AtomicUpdateClusterCR(crClient versioned.Interface, name, namespace string, maxRetries int, updateFunc k8sutil.EtcdClusterCRUpdateFunc) (*api.EtcdCluster, error) {
	result := &api.EtcdCluster{}
	err := retryutil.Retry(1*time.Second, maxRetries, func() (done bool, err error) {
		etcdCluster, err := crClient.EtcdV1beta2().EtcdClusters(namespace).Get(name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		updateFunc(etcdCluster)

		result, err = crClient.EtcdV1beta2().EtcdClusters(namespace).Update(etcdCluster)
		if err != nil {
			if apierrors.IsConflict(err) {
				return false, nil
			}
			return false, err
		}
		return true, nil
	})
	return result, err
}
 */

