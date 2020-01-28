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
	"reflect"
)

func CreateGaleraClaim(galspec *apigalera.GaleraSpec, claimName, clusterName, clusterNamespace, revision string, owner metav1.OwnerReference) *corev1.PersistentVolumeClaim {
	claim := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name :     claimName,
			Namespace: clusterNamespace,
			Labels:    ClaimLabelsForGalera(clusterName, clusterNamespace, revision),
		},
		Spec: galspec.PersistentVolumeClaimSpec,
	}

	addOwnerRefToObject(claim.GetObjectMeta(), owner)
	return claim
}

// CheckClaim returns true is the provided pvc is compliant with the pvc spec described in the Galera object
func CheckClaim(galera *apigalera.Galera, claim *corev1.PersistentVolumeClaim) bool {
	if !reflect.DeepEqual(galera.Spec.PersistentVolumeClaimSpec.AccessModes, claim.Spec.AccessModes) {
		return false
	}

	if !reflect.DeepEqual(galera.Spec.PersistentVolumeClaimSpec.Selector, claim.Spec.Selector) {
		return false
	}

	if !reflect.DeepEqual(galera.Spec.PersistentVolumeClaimSpec.Resources, claim.Spec.Resources) {
		return false
	}

	if !reflect.DeepEqual(galera.Spec.PersistentVolumeClaimSpec.StorageClassName, claim.Spec.StorageClassName) {
		return false
	}
	/*
	if *galera.Spec.PersistentVolumeClaimSpec.StorageClassName != *claim.Spec.StorageClassName {
		return false
	}
	*/

	if !reflect.DeepEqual(galera.Spec.PersistentVolumeClaimSpec.VolumeMode, claim.Spec.VolumeMode) {
		// Value of Filesystem is implied when not included in claim spec.
		if *claim.Spec.VolumeMode == corev1.PersistentVolumeFilesystem && galera.Spec.PersistentVolumeClaimSpec.VolumeMode == nil {
		} else {
			return false
		}
	}
	/*
	if *galera.Spec.PersistentVolumeClaimSpec.VolumeMode != *claim.Spec.VolumeMode {
		return false
	}
	*/
	return true
}