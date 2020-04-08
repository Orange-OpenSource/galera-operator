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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// UpgradeRule holds the UpgradeRule configuration.
type UpgradeRule struct {
	// we embed these types so UpgradeRule implements runtime.Object
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// For deprecated options, operator must check if this option is used in galera conf. If it is the case,
	// the upgrade process is stopped and an error is sent
	// +optional
	RemovedOption *RemovedOption `json:"removedOption,omitempty"`

	// For option where the default parameter is modified. If the option is not present in the config file,
	// Galera Operator must send a warning
	// +optional
	ChangedDefaultOption *ChangedDefaultOption `json:"changedDefaultOption,omitempty"`

	// For replaced option, options must be rewritten
	// A warning is sent
	// +optional
	ReplacedOption *ReplacedOption `json:"replacedOption,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// UpgradeRuleList defines a List type for our custom UpgradeRule type.
// This is needed in order to make List operations work.
type UpgradeRuleList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list metadata
	// More info: http://releases.k8s.io/HEAD/docs/devel/api-conventions.md#metadata
	metav1.ListMeta `json:"metadata, omitempty"`

	Items []UpgradeRule `json:"items"`
}

// RemovedOption holds the RemovedOption configuration.
type RemovedOption struct {
	Option      string `json:"option"`
}

// ChangedDefaultOption holds the ChangedDefaultOption configuration.
type ChangedDefaultOption struct {
	Option      string `json:"option"`
	Old			string `json:"oldDefault"`
	New			string `json:"newDefault"`
}

// ReplacedOption holds the ReplacedOption configuration.
type ReplacedOption struct {
	Option      string `json:"option"`
	Replacement string `json:"replacement"`
}