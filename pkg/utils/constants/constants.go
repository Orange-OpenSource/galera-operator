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

package constants

const (
	EnvOperatorPodName = "MY_POD_NAME"
	EnvOperatorPodNamespace = "MY_POD_NAMESPACE"

	ResyncPeriod = 30
	RunThread = 2
	BackupThread = 2
	TimeOut = 3600

	ListenAddress = ":8080"
	BootstrapImage = "sebs42/galera-bootstrap:0.4"
	BackupImage = "sebs42/galera-backup:0.4"
	UpgradeConfig = ""
)
