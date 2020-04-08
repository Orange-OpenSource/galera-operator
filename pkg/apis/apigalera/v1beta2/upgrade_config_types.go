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

package v1beta2

// ***************************************************************************
// IMPORTANT FOR CODE GENERATION
// If the types in this file are updated, you will need to run
// `make codegen` to generate the new types under the pkg/client folder.
// ***************************************************************************

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// UpgradeConfig describes a specification made of upgrade rules to validate Galera Cluster upgrade
type UpgradeConfig struct {
	// we embed these types so UpgradeConfig implements runtime.Object
	metav1.TypeMeta `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	UpgradeRules []RulesForVersion `json:"upgradeRules"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// UpgradeConfigList defines a List type for our custom UpgradeConfig type.
// This is needed in order to make List operations work.
type UpgradeConfigList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list metadata
	// More info: http://releases.k8s.io/HEAD/docs/devel/api-conventions.md#metadata
	metav1.ListMeta `json:"metadata, omitempty"`

	Items []UpgradeConfig `json:"items"`
}

type RulesForVersion struct {
	// Specify the target version when upgrading a Galera Cluster, rules will apply to the target version.
	// The version must follow the [semver]( http://semver.org) format, for example "10.4.5".
	TargetVersion string `json:"targetVersion"`

	// Rules is slice of UpgradeRule resources
	// +optional
	Rules []corev1.LocalObjectReference `json:"rules,omitempty"`
}