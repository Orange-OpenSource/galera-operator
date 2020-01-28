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

package mariabackup

import (
	"fmt"
	"galera-operator/pkg/exec"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type MariaBackup struct {
	logger *logrus.Entry
	client clientset.Interface
	config *rest.Config
	backupPod *corev1.Pod
	user string
	password string
}

// NewMethod creates a provider capable of creating and restoring backups with the mariabackup tool
func NewMethod(client clientset.Interface, config *rest.Config, backupPod *corev1.Pod, backupCreds map[string]string) (*MariaBackup, error) {
	logger := logrus.WithField("pkg", "backupcontroller")

	user, password, err := getCredentials(backupCreds)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return &MariaBackup{logger:logger, client:client, config:config, backupPod:backupPod, user:user, password:password}, nil
}

// Backup performs a full cluster backup using the mariabackup tool
func (mb *MariaBackup) Backup(backupDir string) error {
	// remove backup directory
	cmd := []string{"rm", "-rf", backupDir}

	_, stderr, err := exec.ExecCmd(mb.client, mb.config, mb.backupPod.Namespace, mb.backupPod, cmd)

	if err != nil {
		mb.logger.Infof("error (%e) executing rm command : %s", err, stderr)
		return err
	}

	// create backup directory
	cmd = []string{"mkdir", backupDir}

	_, stderr, err = exec.ExecCmd(mb.client, mb.config, mb.backupPod.Namespace, mb.backupPod, cmd)

	if err != nil {
		mb.logger.Infof("error (%e) executing mkdir command : %s", err, stderr)
		return err
	}

	//mariabackup --backup --user=root --password=test --target-dir=/var/lib/bak
	//mariabackup --prepare --target-dir=/var/lib/bak -uroot -ptest

	// build the complete mariabackup command used to backup
	cmd = []string{"mariabackup", "--backup"}
	cmd = append(cmd, fmt.Sprintf("--user=%s", mb.user))
	cmd = append(cmd, fmt.Sprintf("--password=%s", mb.password))
	cmd = append(cmd, fmt.Sprintf("--target-dir=%s", backupDir))

	_, stderr, err = exec.ExecCmd(mb.client, mb.config, mb.backupPod.Namespace, mb.backupPod, cmd)

	if err != nil {
		mb.logger.Infof("error (%e) executing mariabackup command : %s", err, stderr)
		return err
	}

	// build the complete mariabackup command used to prepare backup
	cmd = []string{"mariabackup", "--prepare"}
	cmd = append(cmd, fmt.Sprintf("--user=%s", mb.user))
	cmd = append(cmd, fmt.Sprintf("--password=%s", mb.password))
	cmd = append(cmd, fmt.Sprintf("--target-dir=%s", backupDir))

	_, stderr, err = exec.ExecCmd(mb.client, mb.config, mb.backupPod.Namespace, mb.backupPod, cmd)

	if err != nil {
		mb.logger.Infof("error (%e) executing mariabackup command : %s", err, stderr)
	}

	return err
}

// Restore a cluster from a mariabackup.
func (mb *MariaBackup) Restore(backupDir string) error {
	// build the complete mariabackup command used to restore
	cmd := []string{"mariabackup", "--copy-back"}
	cmd = append(cmd, fmt.Sprintf("--user=%s", mb.user))
	cmd = append(cmd, fmt.Sprintf("--password=%s", mb.password))
	cmd = append(cmd, fmt.Sprintf("--target-dir=%s", backupDir))

	_, stderr, err := exec.ExecCmd(mb.client, mb.config, mb.backupPod.Namespace, mb.backupPod, cmd)

	if err != nil {
		mb.logger.Infof("error (%e) executing mariabackup command : %s", err, stderr)
		return err
	}

	// adjust the owner of the data directory to match the user and group for the MariaDB Server
	cmd = []string{"chown", "-R", "mysql:mysql", "/var/lib/mysql"}

	_, stderr, err = exec.ExecCmd(mb.client, mb.config, mb.backupPod.Namespace, mb.backupPod, cmd)

	if err != nil {
		mb.logger.Infof("error (%e) executing chown command : %s", err, stderr)
	}

	return err
}

// getCredentials gets an user and password from the provided map.
func getCredentials(credentials map[string]string) (string, string, error) {
	allErrs := field.ErrorList{}
	fldPath := field.NewPath("data")

	if credentials == nil {
		return "", "", errors.New("no credentials provided")
	}

	user, ok := credentials["user"]
	if !ok {
		allErrs = append(allErrs, field.Required(fldPath.Child("user"), ""))
	}
	password, ok := credentials["password"]
	if !ok {
		allErrs = append(allErrs, field.Required(fldPath.Child("password"), ""))
	}

	if len(allErrs) > 0 {
		return "", "", allErrs.ToAggregate()
	}

	return user, password, nil
}