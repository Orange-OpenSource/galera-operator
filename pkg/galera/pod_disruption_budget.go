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
	policyv1 "k8s.io/api/policy/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func NewGaleraPodDisruptionBudget(galspec *apigalera.GaleraSpec, labels map[string]string, clusterName, clusterNamespace, pdbName string, maxUnavailable int, owner metav1.OwnerReference) *policyv1.PodDisruptionBudget {
	pdb := newGaleraPodDisruptionBudget(labels, clusterName, clusterNamespace, pdbName, maxUnavailable)
	addOwnerRefToObject(pdb.GetObjectMeta(), owner)

	return pdb
}

func newGaleraPodDisruptionBudget(galLabels map[string]string, clusterName, clusterNamespace, pdbName string, maxUnavailable int) *policyv1.PodDisruptionBudget {
	i :=  intstr.FromInt(maxUnavailable)

	labelsToMatch := labelsForGalera(clusterName, clusterNamespace)
	labelsToMatch[apigalera.GaleraStateLabel] = apigalera.StateCluster

	labelSelector := metav1.SetAsLabelSelector(labelsToMatch)

	lsr := metav1.LabelSelectorRequirement{
		Key:      apigalera.GaleraRoleLabel,
		Operator: metav1.LabelSelectorOpNotIn,
		Values:   []string{apigalera.RoleSpecial},
	}

	labelSelector.MatchExpressions = []metav1.LabelSelectorRequirement{lsr}

	return &policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Name:   pdbName,
			Labels: galLabels,
		},
		Spec: policyv1.PodDisruptionBudgetSpec{
			MaxUnavailable: &i,
			Selector: labelSelector,
		},
	}
}