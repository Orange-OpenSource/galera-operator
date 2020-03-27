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

package galera_handlers

import (
	"fmt"
	"github.com/go-openapi/runtime/middleware"
	"github.com/go-openapi/swag"
	"galera-backup/pkg/models"
	"galera-backup/pkg/restapi/operations"
)

type StateName string

const (
	StateNone StateName = ""
	StateUnknown StateName = "unknown"
	StateRunning StateName = "running"
	StateCompleted StateName = "completed"
	StateFailed StateName = "failed"
)

// StateMap is used to keep state of scheduled backup or restore
var StateMap = make(map[string]StateName)

func State() func(params operations.GetStateByNameParams) middleware.Responder {
	return func(params operations.GetStateByNameParams) middleware.Responder {
		switch StateMap[params.Name] {
		case StateNone:
			return operations.NewGetStateByNameDefault(404).WithPayload(
				&models.Response{
					Status: swag.String(string(StateUnknown)),
					Message: swag.String(fmt.Sprintf("%s is unknow", params.Name)),
				})
		default:
			return operations.NewGetStateByNameOK().WithPayload(
				&models.Response{
					Status: swag.String(string(StateMap[params.Name])),
					Message: swag.String(fmt.Sprintf("%s is %s", params.Name, string(StateMap[params.Name]))),
				})
		}
	}
}
