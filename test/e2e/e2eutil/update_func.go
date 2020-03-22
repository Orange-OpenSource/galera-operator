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
	apigalera "galera-operator/pkg/apis/apigalera/v1beta2"
)

type GaleraUpdateFunc func(*apigalera.Galera)

func GaleraWithNewImage(galera *apigalera.Galera, image string) *apigalera.Galera {
	galera.Spec.Pod.Image = image
	return galera
}

func GaleraSize(galera *apigalera.Galera, size int) *apigalera.Galera {
	newSize := int32(size)
	galera.Spec.Replicas = &newSize
	return galera
}
