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
	pkgbackup "galera-operator/pkg/backup"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientset "k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"sync"
)

// GaleraRestoreControlInterface implements the controle logic for restoring Galera clusters. It is implemented
// as an interface to provide for testing fakes.
type GaleraRestoreControlInterface interface {
	// HandleGaleraRestore implements the control logic for restoring a Galera cluster. All pod receive a copy of the backup
	// and MethodType is used to rebuilt the database
	HandleGaleraRestore(galera *apigalera.Galera, pods []*corev1.Pod, mapCredGalera map[string]string)
}

func NewRealGaleraRestoreControl(
	client clientset.Interface,
	config *rest.Config,
	podLister corelisters.PodLister,
	pvcLister corelisters.PersistentVolumeClaimLister,
	secretLister corelisters.SecretLister,
	recorder record.EventRecorder) GaleraRestoreControlInterface {
	logger := logrus.WithField("pkg", "controller")
	return &realGaleraRestoreControl{
		logger,
		client,
		config,
		podLister,
		pvcLister,
		secretLister,
		recorder}
}

type realGaleraRestoreControl struct {
	logger			*logrus.Entry
	client			clientset.Interface
	config			*rest.Config
	podLister		corelisters.PodLister
	pvcLister		corelisters.PersistentVolumeClaimLister
	secretLister	corelisters.SecretLister
	recorder		record.EventRecorder
}

func (grc *realGaleraRestoreControl) HandleGaleraRestore(
	galera *apigalera.Galera,
	pods []*corev1.Pod,
	mapCredGalera map[string]string) {

	switch galera.Spec.Restore.StorageProviderType {
	case apigalera.BackupProviderTypeS3:
		// Get credentials, credentials are used to access galera containers and also to create readiness and liveness probes
		mapCredS3, err := grc.getRestoreS3Creds(galera)
		if err != nil {
			grc.logger.Fatalf("impossible to get S3 credentials : %s", err)
			return
		}
//		switch galera.Spec.Restore.MethodType {
//		case apigalera.BackupMethodTypeMariabackup:
		var wg sync.WaitGroup
		failedPodNames := make(chan string, len(pods))

		// parallel restore for each pod in the galera cluster
		for _, pod := range pods {
			wg.Add(1)
			go func(pod *corev1.Pod) {
				defer wg.Done()
				restoreHandler, err := pkgbackup.NewBackupHandlerNoneS3(pod, galera.Spec.Restore.StorageProvider, mapCredS3)
				if err != nil {
					grc.recorder.Event(galera, corev1.EventTypeWarning, "Restore Failed", err.Error())
					grc.logger.Infof("error creating target method none / storage %v used to restore : %s", galera.Spec.Restore.StorageProviderType, err)
					failedPodNames <- pod.Name
				} else {
					err = restoreHandler.Restore(galera.Spec.Restore.Name)
					if err != nil {
						grc.recorder.Event(galera, corev1.EventTypeWarning, "Restore Failed", err.Error())
						grc.logger.Infof("error when restoring with storage %v : %s", galera.Spec.Restore.StorageProviderType, err)
						failedPodNames <- pod.Name
					} else {
						failedPodNames <- ""
					}
				}
			}(pod)
		}

		wg.Wait()
		var failedPods []*corev1.Pod
		close(failedPodNames)

		// delete pod and keep claim if pod restore succeeded
		for failedPodName := range failedPodNames {
			var failed *corev1.Pod
			for  _, pod := range pods {
				if failedPodName == pod.Name {
					failed = pod
				}
			}

			if failed != nil {
				failedPods = append(failedPods, failed)
			}
		}

		// delete claim if pod restore failed
		for _, pod := range failedPods {
			claimName := getPersistentVolumeClaimName(galera, getPodSuffix(pod))
			go retry.RetryOnConflict(retry.DefaultBackoff, func() error{
				claimUpdated, updateErr := grc.podLister.Pods(galera.Namespace).Get(claimName)
				if apierrors.IsNotFound(err) {
					return nil
				}
				if updateErr != nil {
					return updateErr
				}
				if updateErr = grc.client.CoreV1().PersistentVolumeClaims(galera.Namespace).Delete(claimUpdated.Name, nil); updateErr == nil {
					return nil
				} else {
					utilruntime.HandleError(fmt.Errorf("error deleting pod %s/%s : %v", galera.Namespace, claimName, updateErr))
				}
				return updateErr
			})
		}

		// delete all pods
		for _, pod := range pods {
			podUpdated, err := grc.podLister.Pods(galera.Namespace).Get(pod.Name)
			if err == nil {
				// delete pod
				podName := podUpdated.Name
				go retry.RetryOnConflict(retry.DefaultBackoff, func() error{
					podUpdated, updateErr := grc.podLister.Pods(galera.Namespace).Get(podName)
					if apierrors.IsNotFound(err) {
						return nil
					}
					if updateErr != nil {
						return updateErr
					}
					if updateErr = grc.client.CoreV1().Pods(galera.Namespace).Delete(podUpdated.Name, nil); updateErr == nil {
						return nil
					} else {
						utilruntime.HandleError(fmt.Errorf("error deleting pod %s/%s : %v", galera.Namespace, podName, updateErr))
					}
					return updateErr
				})
			}
		}
//		default:
//			grc.logger.Fatalf("unknown restoreMethodType: %v", galera.Spec.Restore.MethodType)
//		}
	default:
		grc.logger.Fatalf("unknown StorageProviderType: %v", galera.Spec.Restore.StorageProviderType)
	}

	return
}

// getRestoreS3Creds returns a map containing S3 credentials.
// If the returned error is nil, S3 credentials are valid.
func (grc *realGaleraRestoreControl) getRestoreS3Creds(galera *apigalera.Galera) (map[string]string, error) {

	secretS3, err := grc.secretLister.Secrets(galera.Namespace).Get(galera.Spec.Restore.StorageProvider.S3.CredentialsSecret.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("galera '%s/%s' can not find CredentialsSecret %s", galera.Namespace, galera.Name, galera.Spec.Restore.StorageProvider.S3.CredentialsSecret.Name))
			return nil, err
		}
		utilruntime.HandleError(fmt.Errorf("unable to retrieve Galera '%s/%s' CredentialsSecret: %v", galera.Namespace, galera.Name, err))
		return nil, err
	}

	mapCredS3 := make(map[string]string, len(secretS3.Data))
	for k, v := range secretS3.Data {
		mapCredS3[k] = string(v)
	}

	if mapCredS3["accessKeyId"] == "" || mapCredS3["secretAccessKey"] == "" {
		err = fmt.Errorf("all users and/or passwords are not set in secret %s", galera.Spec.Restore.StorageProvider.S3.CredentialsSecret.Name)
	}

	return mapCredS3, err
}

var _ GaleraRestoreControlInterface = &realGaleraRestoreControl{}