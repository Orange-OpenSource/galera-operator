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

package none

// None method is used to provide a way to do nothing and following the backup interface pattern.
// It is mainly used when restoring, because restore is provided by the copy and mount operations.
type None struct {
}

func NewMethod() (*None, error) {
	return &None{}, nil
}

func (mb *None) Backup(backupDir string) error {
	return nil
}

func (mb *None) Restore(backupDir string) error {
	return nil
}
