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
	"encoding/json"
	"fmt"
	apigalera "galera-operator/pkg/apis/apigalera/v1beta2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func PodLabelsForGalera(labels map[string]string, clusterName, clusterNamespace, role, revision, state string) map[string]string {
	l := make(map[string]string)

	for label, value := range labels {
		l[label] = value
	}

	l[apigalera.GaleraClusterName]      = clusterName
	l[apigalera.GaleraClusterNamespace] = clusterNamespace
	l[apigalera.GaleraStateLabel]       = state
	l[apigalera.GaleraRevisionLabel]    = revision

	switch role {
	case apigalera.RoleWriter:
		l[apigalera.GaleraRoleLabel]   = role
		l[apigalera.GaleraReaderLabel] = "false"
	case apigalera.RoleBackupWriter:
		l[apigalera.GaleraRoleLabel]   = role
		l[apigalera.GaleraReaderLabel] = "true"
	case apigalera.Backup:
		l[apigalera.GaleraBackupLabel] = "true"
		l[apigalera.GaleraReaderLabel] = "true"
	case apigalera.Restore:
		l[apigalera.GaleraRoleLabel]   = role
	case apigalera.Reader:
		l[apigalera.GaleraReaderLabel] = "true"
	case apigalera.RoleSpecial:
		l[apigalera.GaleraRoleLabel]   = role
		l[apigalera.GaleraReaderLabel] = "false"
	default:
	}

	return l
}

func ClaimLabelsForGalera(labels map[string]string, clusterName, clusterNamespace, revision string) map[string]string {
	l := make(map[string]string)

	for label, value := range labels {
		l[label] = value
	}

	l[apigalera.GaleraClusterName]      = clusterName
	l[apigalera.GaleraClusterNamespace] = clusterNamespace
	l[apigalera.GaleraRevisionLabel]    = revision

	return l
}

func buildName(name, prefix string) string {
	l := apigalera.MaxK8SNameLength - len(prefix) - len(name)
	t := 0
	if l < 0 {
		t = -l
	}

	return fmt.Sprintf("%s%s", prefix, name[t:])
}

func BuildClaimNameForGalera(podName string) string {
	return buildName(podName, apigalera.ClaimPrefix)
}

func BuildBackupClaimNameForGalera(podName string) string {
	return buildName(podName, apigalera.BackupClaimPrefix)
}

func labelsForGalera(labels map[string]string, clusterName, clusterNamespace string) map[string]string {
	l := make(map[string]string)

	for label, value := range labels {
		l[label] = value
	}

	l[apigalera.GaleraClusterName]      = clusterName
	l[apigalera.GaleraClusterNamespace] = clusterNamespace

	return l
}

func serviceSelectorForGalera(clusterName, clusterNamespace, role string) map[string]string {
	switch role {
	case apigalera.RoleWriter:
		return map[string]string{
			apigalera.GaleraClusterName:      clusterName,
			apigalera.GaleraClusterNamespace: clusterNamespace,
			apigalera.GaleraRoleLabel:        role,
		}
	case apigalera.RoleBackupWriter:
		return map[string]string{
			apigalera.GaleraClusterName:      clusterName,
			apigalera.GaleraClusterNamespace: clusterNamespace,
			apigalera.GaleraRoleLabel:        role,
		}
	case apigalera.Reader:
		return map[string]string{
			apigalera.GaleraClusterName:      clusterName,
			apigalera.GaleraClusterNamespace: clusterNamespace,
			apigalera.GaleraReaderLabel:      "true",
		}
	case apigalera.RoleSpecial:
		return map[string]string{
			apigalera.GaleraClusterName:      clusterName,
			apigalera.GaleraClusterNamespace: clusterNamespace,
			apigalera.GaleraRoleLabel:        role,
			apigalera.GaleraReaderLabel:      "false",
		}
	default:
		return map[string]string{
			apigalera.GaleraClusterName:      clusterName,
			apigalera.GaleraClusterNamespace: clusterNamespace,
		}
	}
}

func SelectorForGalera(labels map[string]string ,clusterName, clusterNamespace string) (labels.Selector, error) {
	return metav1.LabelSelectorAsSelector(metav1.SetAsLabelSelector(labelsForGalera(labels, clusterName, clusterNamespace)))
}

func addOwnerRefToObject(o metav1.Object, r metav1.OwnerReference) {
	o.SetOwnerReferences(append(o.GetOwnerReferences(), r))
}

// mergeLabels merges l2 into l1. Conflicting label will be skipped.
func mergeLabels(l1, l2 map[string]string) {
	for k, v := range l2 {
		if _, ok := l1[k]; ok {
			continue
		}
		l1[k] = v
	}
}

func PodSpecToJSON(pod *corev1.Pod) (string, error) {
	bytes, err := json.MarshalIndent(pod.Spec, "", "    ")
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}