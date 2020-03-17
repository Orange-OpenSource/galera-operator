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

package framework

import (
	"bytes"
	"flag"
	"fmt"
	"galera-operator/pkg/client/clientset/versioned"
	"galera-operator/pkg/utils/constants"
	"galera-operator/pkg/utils/probe"
	"galera-operator/test/e2e/e2eutil"
	"galera-operator/test/e2e/k8sutil"
	"galera-operator/test/e2e/retryutil"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"os/exec"
	"time"
)

var Global *Framework

type Framework struct {
	opImage      string
	KubeClient   kubernetes.Interface
	GaleraClient versioned.Interface
	Namespace    string
}

// Setup setups a test framework and points "Global" to it.
func Setup() error {
	kubeconfig := flag.String("kubeconfig", "", "kube config path, e.g. $HOME/.kube/config")
	opImage := flag.String("operator-image", "", "operator image, e.g. sebs42/galera-operator:0.4")
	ns := flag.String("namespace", "default", "e2e test namespace")
	flag.Parse()

	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		return err
	}
	cli, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}

	galeraCli, err := versioned.NewForConfig(config)
	if err != nil {
		return err
	}

	Global = &Framework{
		KubeClient:   cli,
		GaleraClient: galeraCli,
		Namespace:    *ns,
		opImage:      *opImage,
	}

	// Skip the galera-operator deployment setup if the operator image was not specified
	if len(Global.opImage) == 0 {
		return nil
	}

	return Global.setup()
}

func Teardown() error {
	// Skip the galera-operator teardown if the operator image was not specified
	if len(Global.opImage) == 0 {
		return nil
	}

	err := Global.deleteOperatorCompletely("galera-operator-test")
	if err != nil {
		return err
	}
	Global = nil
	logrus.Info("e2e teardown successfully")
	return nil
}

func (f *Framework) setup() error {
	err := f.setupGaleraOperatorPod()
	if err != nil {
		return fmt.Errorf("failed to setup galera operator: %v", err)
	}
	logrus.Info("galera operator pod deployed successfully")

	logrus.Info("e2e setup successfully")
	return nil
}

func (f *Framework) setupGaleraOperatorPod() error {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "galera-operator-test",
			Labels: e2eutil.OperatorLabelSelector(),
		},
		Spec: corev1.PodSpec{
			ServiceAccountName: "galera-operator-test",
			Containers: []corev1.Container{{
				Name:            "galera-operator",
				Image:           f.opImage,
				ImagePullPolicy: corev1.PullAlways,
				Command:         []string{"galera-operator"},
				Env: []corev1.EnvVar{
					{
						Name:      constants.EnvOperatorPodNamespace,
						ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.namespace"}},
					},
					{
						Name:      constants.EnvOperatorPodName,
						ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.name"}},
					},
				},
				LivenessProbe: &corev1.Probe{
					Handler: corev1.Handler{
						HTTPGet: &corev1.HTTPGetAction{
							Path: probe.HTTPProbeEndpoint,
							Port: intstr.IntOrString{Type: intstr.Int, IntVal: 8080},
						},
					},
					InitialDelaySeconds: 3,
					PeriodSeconds:       3,
					FailureThreshold:    3,
				},
			}},
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}

	p, err := k8sutil.CreateAndWaitPod(f.KubeClient, f.Namespace, pod, 60*time.Second)
	if err != nil {
		describePod(f.Namespace, "galera-operator-test")
		return err
	}
	logrus.Infof("galera operator pod is running on node (%s)", p.Spec.NodeName)

	return e2eutil.WaitUntilOperatorReady(f.KubeClient, f.Namespace)
}

func describePod(ns, name string) {
	// assuming `kubectl` installed on $PATH
	cmd := exec.Command("kubectl", "-n", ns, "describe", "pod", name)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Run() // Just ignore the error...
	logrus.Infof("describing %s pod: %s", name, out.String())
}

func (f *Framework) deleteOperatorCompletely(name string) error {
	err := f.KubeClient.CoreV1().Pods(f.Namespace).Delete(name, metav1.NewDeleteOptions(1))
	if err != nil {
		return err
	}
	// Grace period isn't exactly accurate. It took ~10s for operator pod to completely disappear.
	// We work around by increasing the wait time. Revisit this later.
	err = retryutil.Retry(5*time.Second, 6, func() (bool, error) {
		_, err := f.KubeClient.CoreV1().Pods(f.Namespace).Get(name, metav1.GetOptions{})
		if err == nil {
			return false, nil
		}
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		return false, err
	})
	if err != nil {
		return fmt.Errorf("fail to wait operator (%s) pod gone from API: %v", name, err)
	}
	return nil
}

