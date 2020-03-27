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
	"fmt"
	apigalera "galera-operator/pkg/apis/apigalera/v1beta2"
	"galera-operator/pkg/exec"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"math/rand"
	"regexp"
	"strconv"
)

// wsrepClusterSize is a regular expression that extracts the wsrep_cluster_size value form a string
var wsrepClusterSize = regexp.MustCompile("(?im)^(.*)wsrep_cluster_size(.*)$")

func MatchGaleraSize(kubeClient kubernetes.Interface, config *rest.Config, galera *apigalera.Galera, size int) (bool, error) {
	podList, err := kubeClient.CoreV1().Pods(galera.Namespace).List(GaleraListOpt())
	if err != nil {
		return false, err
	}
	if len(podList.Items) < 1 {
		return false, fmt.Errorf("no pod found for galera %s/%s", galera.Namespace, galera.Name)
	}
	pod := &podList.Items[0]

	cmd := []string{"mysql"}
	cmd = append(cmd, fmt.Sprintf("-u%s", user))
	cmd = append(cmd, fmt.Sprintf("-p%s", password))
	cmd = append(cmd, "-e")
	cmd = append(cmd, "SHOW GLOBAL STATUS LIKE 'wsrep_cluster_size'")

	stdout, _, err := exec.ExecCmd(kubeClient, config, pod.Namespace, pod, cmd)

	if err != nil {
		return false, err
	}

	result := wsrepClusterSize.Find([]byte(stdout))

	matchSize := regexp.MustCompile(strconv.Itoa(size))
	if matchSize.Match(result) {
		return true, nil
	}

	return false, nil
}

func AddData(kubeClient kubernetes.Interface, config *rest.Config, galera *apigalera.Galera) error {
	podList, err := kubeClient.CoreV1().Pods(galera.Namespace).List(GaleraListOpt())
	if err != nil {
		return err
	}
	if len(podList.Items) < 1 {
		return fmt.Errorf("no pod found for galera %s/%s", galera.Namespace, galera.Name)
	}
	pod := &podList.Items[0]

	initCmd := func() []string {
		cmd := []string{"mysql"}
		cmd = append(cmd, fmt.Sprintf("-u%s", user))
		cmd = append(cmd, fmt.Sprintf("-p%s", password))
		cmd = append(cmd, "-e")
		return cmd
	}()

	cmd := append(initCmd, "CREATE DATABASE test")
	_, _, err = exec.ExecCmd(kubeClient, config, pod.Namespace, pod, cmd)
	if err != nil {
		return err
	}

	cmd = append(initCmd, "USE test; CREATE TABLE shop (article INT UNSIGNED DEFAULT '0000' NOT NULL, dealer  CHAR(20) DEFAULT '' NOT NULL, price DECIMAL(16,2) DEFAULT '0.00' NOT NULL, PRIMARY KEY(article, dealer)) ENGINE=InnoDB")
	_, _, err = exec.ExecCmd(kubeClient, config, pod.Namespace, pod, cmd)
	if err != nil {
		return err
	}

	cmd  = append(initCmd, "USE test; INSERT INTO shop VALUES (1,'A',3.45),(1,'B',3.99),(2,'A',10.99),(3,'B',1.45), (3,'C',1.69),(3,'D',1.25),(4,'D',19.95)")
	_, _, err = exec.ExecCmd(kubeClient, config, pod.Namespace, pod, cmd)
	if err != nil {
		return err
	}
/*
	CREATE TABLE shop (
		article INT UNSIGNED  DEFAULT '0000' NOT NULL,
		dealer  CHAR(20)      DEFAULT ''     NOT NULL,
		price   DECIMAL(16,2) DEFAULT '0.00' NOT NULL,
		PRIMARY KEY(article, dealer));
	INSERT INTO shop VALUES
	(1,'A',3.45),(1,'B',3.99),(2,'A',10.99),(3,'B',1.45),
	(3,'C',1.69),(3,'D',1.25),(4,'D',19.95);
*/

	return nil
}


func CheckData(kubeClient kubernetes.Interface, config *rest.Config, galera *apigalera.Galera) (bool, error) {
	podList, err := kubeClient.CoreV1().Pods(galera.Namespace).List(GaleraListOpt())
	if err != nil {
		return false, err
	}
	if len(podList.Items) < 1 {
		return false, fmt.Errorf("no pod found for galera %s/%s", galera.Namespace, galera.Name)
	}

	initCmd := func() []string {
		cmd := []string{"mysql"}
		cmd = append(cmd, fmt.Sprintf("-u%s", user))
		cmd = append(cmd, fmt.Sprintf("-p%s", password))
		cmd = append(cmd, "-e")
		return cmd
	}()

	/*
		SELECT * FROM shop ORDER BY article;
		+---------+--------+-------+
		| article | dealer | price |
		+---------+--------+-------+
		|       1 | A      |  3.45 |
		|       1 | B      |  3.99 |
		|       2 | A      | 10.99 |
		|       3 | B      |  1.45 |
		|       3 | C      |  1.69 |
		|       3 | D      |  1.25 |
		|       4 | D      | 19.95 |
		+---------+--------+-------+
	*/

	expected := "article\tdealer\tprice\n1\tA\t3.45\n1\tB\t3.99\n2\tA\t10.99\n3\tB\t1.45\n3\tC\t1.69\n3\tD\t1.25\n4\tD\t19.95\n"

	for _, pod := range podList.Items {
		cmd := append(initCmd, "USE test; SELECT * FROM shop ORDER BY article")
		stdout, _, err := exec.ExecCmd(kubeClient, config, pod.Namespace, &pod, cmd)
		if err != nil {
			return false, err
		}
		if stdout != expected {
			return false, fmt.Errorf("mysql request result is not correct for pod %s", pod.Name)
		}
	}

	return true, nil
}

func KillGaleraNode(kubeClient kubernetes.Interface, namespace string, size int) error {
	podList, err := kubeClient.CoreV1().Pods(namespace).List(GaleraListOpt())
	if err != nil {
		return err
	}

	condemned := rand.Intn(size)
	if condemned > len(podList.Items) {
		return fmt.Errorf("unexpected error")
	}

	name := podList.Items[condemned].Name
	logrus.Infof("killing pod %s/%s", namespace, name)

	return kubeClient.CoreV1().Pods(namespace).Delete(name, metav1.NewDeleteOptions(0))
}
