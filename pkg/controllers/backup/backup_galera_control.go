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

package backup

import (
	"fmt"
	apigalera "galera-operator/pkg/apis/apigalera/v1beta2"
	pkgbackup "galera-operator/pkg/backup"
	galeraclientset "galera-operator/pkg/client/clientset/versioned"
	listers "galera-operator/pkg/client/listers/apigalera/v1beta2"
	"galera-operator/pkg/controllers/cluster"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientset "k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"time"
)

type GaleraBackupControlInterface interface {
	HandleBackup(backup *apigalera.GaleraBackup, galera *apigalera.Galera) error
}

func NewDefaultGaleraBackup(
	kubeClient clientset.Interface,
	config *rest.Config,
	galeraClient galeraclientset.Interface,
	galeraBackupLister listers.GaleraBackupLister,
	podLister corelisters.PodLister,
	secretLister corelisters.SecretLister,
	galeraBackupStatusUpdater GaleraBackupStatusUpdaterInterface,
	galeraStatusUpdater cluster.GaleraStatusUpdaterInterface,
	recorder record.EventRecorder) GaleraBackupControlInterface {
	logger := logrus.WithField("pkg", "backupcontroller")
	return &defaultGaleraBackupControl{
		logger,
		kubeClient,
		config,
		galeraClient,
		galeraBackupLister,
		podLister,
		secretLister,
		galeraBackupStatusUpdater,
		galeraStatusUpdater,
		recorder}
}

type defaultGaleraBackupControl struct {
	logger						*logrus.Entry
	// client interface
	kubeClient					clientset.Interface
	// K8S config
	config 						*rest.Config
	// galeraClient is a clientset for our own API group
	galeraClient				galeraclientset.Interface
	galeraBackupLister			listers.GaleraBackupLister
	// podLister is able to list/get pods from a shared informer's store
	podLister                 corelisters.PodLister
	secretLister              corelisters.SecretLister
	galeraBackupStatusUpdater GaleraBackupStatusUpdaterInterface
	galeraStatusUpdater       cluster.GaleraStatusUpdaterInterface
	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API
	recorder         			record.EventRecorder
}

func (bgc *defaultGaleraBackupControl) HandleBackup(backup *apigalera.GaleraBackup, galera *apigalera.Galera) error {
	started := time.Now()

	galeraNamespace := backup.Spec.GaleraNamespace
	if galeraNamespace == "" {
		galeraNamespace = backup.Namespace
	}

	switch backup.Spec.StorageProviderType {
	case apigalera.BackupProviderTypeS3:
		secretS3, err := bgc.secretLister.Secrets(galeraNamespace).Get(backup.Spec.StorageProvider.S3.CredentialsSecret.Name)
		if err != nil {
			if errors.IsNotFound(err) {
				utilruntime.HandleError(fmt.Errorf("galeraBackup '%s/%s' can not find CredentialsSecret", backup.Namespace, backup.Name))
				return err
			}
			utilruntime.HandleError(fmt.Errorf("unable to retrieve GaleraBackup '%s/%s' CredentialsSecret: %v", backup.Namespace, backup.Name, err))
			return err
		}

		mapCredS3 := make(map[string]string, len(secretS3.Data))
		for k, v := range secretS3.Data {
			mapCredS3[k] = string(v)
		}

		backupPod, err := bgc.podLister.Pods(galera.Namespace).Get(galera.Status.Members.Backup)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("unable to retrieve the backup pod %s for the galera '%s/%s' : %v", galera.Status.Members.Backup, galera.Namespace, galera.Name, err))
			return err
		}

		switch backup.Spec.MethodType {
		case apigalera.BackupMethodTypeMariabackup:
			secretGalera, err := bgc.secretLister.Secrets(backup.Namespace).Get(backup.Spec.MethodProvider.MariaBackup.CredentialsBackup.Name)

			if err != nil {
				if errors.IsNotFound(err) {
					utilruntime.HandleError(fmt.Errorf("galeraBackup '%s/%s' can not find CredentialsBackup", backup.Namespace, backup.Name))
					return err
				}
				utilruntime.HandleError(fmt.Errorf("unable to retrieve GaleraBackup '%s/%s' CredentialsBackup: %v", backup.Namespace, backup.Name, err))
				return err
			}

			mapCredGalera := make(map[string]string, len(secretGalera.Data))
			for k, v := range secretGalera.Data {
				mapCredGalera[k] = string(v)
			}

			backuphandler, err := pkgbackup.NewBackupHandlerMariabackupS3(bgc.kubeClient, bgc.config, backupPod, mapCredGalera, backup.Spec.StorageProvider, mapCredS3)
			if err != nil {
				bgc.recorder.Event(backup, corev1.EventTypeWarning, "Backup Failed", err.Error())
				bgc.logger.Infof("error creating target method %v / storage %v  for backup", backup.Spec.MethodType, backup.Spec.StorageProviderType)
				return err
			}

			status := &apigalera.GaleraBackupStatus{}
			status.TimeStarted = metav1.Time{Time: started}
			status.SetRunningCondition(backup.Name, galeraNamespace)
			err = bgc.updateGaleraBackupStatus(backup, status)
			if err != nil {
				return err
			}

			go func() {
				key, err := backuphandler.Backup(fmt.Sprintf("%s-%s", galera.Name, backup.Name))
				if err != nil {
					bgc.recorder.Event(backup, corev1.EventTypeWarning, "Backup Failed", err.Error())
					bgc.logger.Infof("error when backuping using method %v with storage %v", backup.Spec.MethodType, backup.Spec.StorageProviderType)
					return
				}

				finished := time.Now()

				status = backup.Status.DeepCopy()

				status.OutcomeLocation = key
				status.TimeCompleted = metav1.Time{Time: finished}
				status.ClearCondition(apigalera.BackupRunning)
				status.SetCompleteCondition()

				err = bgc.updateGaleraBackupStatus(backup, status)
				if err != nil {
					return
				}

				backupsTotal.Inc()
				bgc.logger.Infof("Backup %q succeeded in %v", backup.Name, finished.Sub(started))
				bgc.recorder.Event(backup, corev1.EventTypeNormal, "Complete", "Backup complete")
			}()

			return nil
		default:
			bgc.logger.Fatalf("unknown BackupMethodType: %v", backup.Spec.MethodType)
			return nil
		}
	default:
		bgc.logger.Fatalf("unknown StorageProviderType: %v", backup.Spec.StorageProviderType)
		return nil
	}
}

// updateGaleraBackupStatus will not allow changes to the Spec of the resource, which is ideal for ensuring
// nothing other than resource status has been updated.
func (bgc *defaultGaleraBackupControl) updateGaleraBackupStatus(
	backup *apigalera.GaleraBackup,
	status *apigalera.GaleraBackupStatus) error {

	// copy GaleraBackup and update its status
	backup = backup.DeepCopy()
	if err := bgc.galeraBackupStatusUpdater.UpdateGaleraBackupStatus(backup, status); err != nil {
		return err
	}

	return nil
}

// updateGaleraStatus will not allow changes to the Spec of the resource, which is ideal for ensuring
// nothing other than resource status has been updated.
func (bgc *defaultGaleraBackupControl) updateGaleraStatus(
	galera *apigalera.Galera,
	status *apigalera.GaleraStatus) error {

	// copy Galera and update its status
	galera = galera.DeepCopy()
	if err := bgc.galeraStatusUpdater.UpdateGaleraStatus(galera, status); err != nil {
		return err
	}

	return nil
}

var _ GaleraBackupControlInterface = &defaultGaleraBackupControl{}