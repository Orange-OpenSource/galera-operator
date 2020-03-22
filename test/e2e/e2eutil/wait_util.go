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
	"bytes"
	"fmt"
	apigalera "galera-operator/pkg/apis/apigalera/v1beta2"
	"galera-operator/pkg/client/clientset/versioned"
	"galera-operator/test/e2e/retryutil"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	podutil "k8s.io/kubernetes/pkg/api/v1/pod"
	"testing"
	"time"
)

var retryInterval = 30 * time.Second

type filterPodFunc func(*corev1.Pod) bool
type filterClaimFunc func(*corev1.PersistentVolumeClaim) bool
type filterServiceFunc func(*corev1.Service) bool

// WaitUntilOperatorReady will wait until the first pod selected for the label name=etcd-operator is ready.
func WaitUntilOperatorReady(kubecli kubernetes.Interface, namespace string) error {
	var podName string
	lo := metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(OperatorLabelSelector()).String(),
	}
	err := retryutil.Retry(10*time.Second, 6, func() (bool, error) {
		podList, err := kubecli.CoreV1().Pods(namespace).List(lo)
		if err != nil {
			return false, err
		}
		if len(podList.Items) > 0 {
			pod := podList.Items[0]
			podName = pod.Name
			if podutil.IsPodReady(&pod) {
				return true, nil
			}
		}
		return false, nil
	})
	if err != nil {
		return fmt.Errorf("failed to wait for pod (%v) to become ready: %v", podName, err)
	}
	return nil
}


func WaitPhaseReached(t *testing.T, galeraClient versioned.Interface, retries int, galera *apigalera.Galera, phase apigalera.GaleraPhase) error {
	return retryutil.Retry(retryInterval, retries, func() (bool, error) {
		currCluster, err := galeraClient.SqlV1beta2().Galeras(galera.Namespace).Get(galera.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		if currCluster.Status.Phase == phase {
			return true, nil
		}
		return false, nil
	})
}

func WaitUntilSizeReached(t *testing.T, galeraClient versioned.Interface, size, retries int, galera *apigalera.Galera) ([]string, error) {
	var names []string
	var special int

	err := retryutil.Retry(retryInterval, retries, func() (done bool, err error) {
		currCluster, err := galeraClient.SqlV1beta2().Galeras(galera.Namespace).Get(galera.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		special = 0
		if galera.Spec.Pod.Special != nil {
			if currCluster.Status.Members.Special != "" {
				special = 1
			}
		}

		names = currCluster.Status.Members.Ready
		LogfWithTimestamp(t, "waiting size (%d), healthy galera nodes: names (%v)", size, names)
		if len(names) + special != size {
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		return nil, err
	}
	return names, nil
}

func WaitSizeAndImageReached(t *testing.T, kubeClient kubernetes.Interface, image string, size, retries int, galera *apigalera.Galera) error {
	return retryutil.Retry(retryInterval, retries, func() (done bool, err error) {
		var names []string
		podList, err := kubeClient.CoreV1().Pods(galera.Namespace).List(GaleraListOpt())
		if err != nil {
			return false, err
		}
		names = nil
		var nodeNames []string
		for i := range podList.Items {
			pod := &podList.Items[i]
			if pod.Status.Phase != corev1.PodRunning {
				continue
			}

			find := false
			for _, s := range pod.Status.ContainerStatuses {
				if s.Image == image {
					find = true
				}
			}
			if find == false {
				LogfWithTimestamp(t, "pod(%v): expected image(%v) not found", pod.Name, image)
			} else {
				names = append(names, pod.Name)
				nodeNames = append(nodeNames, pod.Spec.NodeName)
			}
		}
		LogfWithTimestamp(t, "waiting size (%d), galera pods: names (%v), nodes (%v)", size, names, nodeNames)
		if len(names) != size {
			return false, nil
		}
		return true, nil
	})
}

func LogfWithTimestamp(t *testing.T, format string, args ...interface{}) {
	t.Log(time.Now(), fmt.Sprintf(format, args...))
}

func waitResourcesDeleted(t *testing.T, kubeClient kubernetes.Interface, galera *apigalera.Galera) error {
	undeletedPods, err := waitPodsDeleted(kubeClient, galera.Namespace, 3,  GaleraListOpt())
	if err != nil {
		if retryutil.IsRetryFailure(err) && len(undeletedPods) > 0 {
			p := undeletedPods[0]
			LogfWithTimestamp(t, "waiting pod (%s) to be deleted.", p.Name)

			buf := bytes.NewBuffer(nil)
			buf.WriteString("init container status:\n")
			printContainerStatus(buf, p.Status.InitContainerStatuses)
			buf.WriteString("container status:\n")
			printContainerStatus(buf, p.Status.ContainerStatuses)
			t.Logf("pod (%s) status.phase is (%s): %v", p.Name, p.Status.Phase, buf.String())
		}

		return fmt.Errorf("fail to wait pods deleted: %v", err)
	}

	undeletedClaims, err := waitClaimsDeleted(kubeClient, galera.Namespace, 3,  GaleraListOpt())
	if err != nil {
		if retryutil.IsRetryFailure(err) && len(undeletedClaims) > 0 {
			c := undeletedClaims[0]
			LogfWithTimestamp(t, "waiting claim (%s) to be deleted.", c.Name)
			t.Logf("claim (%s) status.phase is (%s)", c.Name, c.Status.Phase)
		}

		return fmt.Errorf("fail to wait claims deleted: %v", err)
	}

	undeletedServices, err := waitServicesDeleted(kubeClient, galera.Namespace, 3,  GaleraListOpt())
	if err != nil {
		if retryutil.IsRetryFailure(err) && len(undeletedServices) > 0 {
			s := undeletedServices[0]
			LogfWithTimestamp(t, "waiting service (%s) to be deleted.", s.Name)

			t.Logf("pod (%s) status is (%s)", s.Name, s.Status)
		}

		return fmt.Errorf("fail to wait services deleted: %v", err)
	}


	/*
	err = retryutil.Retry(retryInterval, 3, func() (done bool, err error) {
		list, err := kubeClient.CoreV1().Services(galera.Namespace).List(GaleraListOpt())
		if err != nil {
			return false, err
		}
		if len(list.Items) > 0 {
			LogfWithTimestamp(t, "waiting service (%s) to be deleted", list.Items[0].Name)
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		return fmt.Errorf("fail to wait services deleted: %v", err)
	}
	*/
	err = retryutil.Retry(retryInterval, 3, func() (done bool, err error) {
		list, err := kubeClient.PolicyV1beta1().PodDisruptionBudgets(galera.Namespace).List(GaleraListOpt())
		if err != nil {
			return false, err
		}
		if len(list.Items) > 0 {
			LogfWithTimestamp(t, "waiting pod disruption budget (%s) to be deleted", list.Items[0].Name)
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		return fmt.Errorf("fail to wait pod disruption budget deleted: %v", err)
	}

	return nil
}

func printContainerStatus(buf *bytes.Buffer, ss []corev1.ContainerStatus) {
	for _, s := range ss {
		if s.State.Waiting != nil {
			buf.WriteString(fmt.Sprintf("%s: Waiting: message (%s) reason (%s)\n", s.Name, s.State.Waiting.Message, s.State.Waiting.Reason))
		}
		if s.State.Terminated != nil {
			buf.WriteString(fmt.Sprintf("%s: Terminated: message (%s) reason (%s)\n", s.Name, s.State.Terminated.Message, s.State.Terminated.Reason))
		}
	}
}


func waitPodsDeleted(kubecli kubernetes.Interface, namespace string, retries int, lo metav1.ListOptions) ([]*corev1.Pod, error) {
	f := func(p *corev1.Pod) bool { return p.DeletionTimestamp != nil }
	return waitPodsDeletedWithFilters(kubecli, namespace, retries, lo, f)
}

func waitPodsDeletedWithFilters(kubecli kubernetes.Interface, namespace string, retries int, lo metav1.ListOptions, filters ...filterPodFunc) ([]*corev1.Pod, error) {
	var pods []*corev1.Pod
	err := retryutil.Retry(retryInterval, retries, func() (bool, error) {
		podList, err := kubecli.CoreV1().Pods(namespace).List(lo)
		if err != nil {
			return false, err
		}
		pods = nil
		for i := range podList.Items {
			p := &podList.Items[i]
			filtered := false
			for _, filter := range filters {
				if filter(p) {
					filtered = true
				}
			}
			if !filtered {
				pods = append(pods, p)
			}
		}
		return len(pods) == 0, nil
	})
	return pods, err
}

func waitClaimsDeleted(kubecli kubernetes.Interface, namespace string, retries int, lo metav1.ListOptions) ([]*corev1.PersistentVolumeClaim, error) {
	f := func(s *corev1.PersistentVolumeClaim) bool { return s.DeletionTimestamp != nil }
	return waitClaimsDeletedWithFilters(kubecli, namespace, retries, lo, f)
}

func waitClaimsDeletedWithFilters(kubecli kubernetes.Interface, namespace string, retries int, lo metav1.ListOptions, filters ...filterClaimFunc) ([]*corev1.PersistentVolumeClaim, error) {
	var claims []*corev1.PersistentVolumeClaim
	err := retryutil.Retry(retryInterval, retries, func() (bool, error) {
		pvcList, err := kubecli.CoreV1().PersistentVolumeClaims(namespace).List(lo)
		if err != nil {
			return false, err
		}
		claims = nil
		for i := range pvcList.Items {
			c := &pvcList.Items[i]
			filtered := false
			for _, filter := range filters {
				if filter(c) {
					filtered = true
				}
			}
			if !filtered {
				claims = append(claims, c)
			}
		}
		return len(claims) == 0, nil
	})
	return claims, err
}

func waitServicesDeleted(kubecli kubernetes.Interface, namespace string, retries int, lo metav1.ListOptions) ([]*corev1.Service, error) {
	f := func(s *corev1.Service) bool { return s.DeletionTimestamp != nil }
	return waitServicesDeletedWithFilters(kubecli, namespace, retries, lo, f)
}

func waitServicesDeletedWithFilters(kubecli kubernetes.Interface, namespace string, retries int, lo metav1.ListOptions, filters ...filterServiceFunc) ([]*corev1.Service, error) {
	var services []*corev1.Service
	err := retryutil.Retry(retryInterval, retries, func() (bool, error) {
		svcList, err := kubecli.CoreV1().Services(namespace).List(lo)
		if err != nil {
			return false, err
		}
		services = nil
		for i := range svcList.Items {
			s := &svcList.Items[i]
			filtered := false
			for _, filter := range filters {
				if filter(s) {
					filtered = true
				}
			}
			if !filtered {
				services = append(services, s)
			}
		}
		return len(services) == 0, nil
	})
	return services, err
}