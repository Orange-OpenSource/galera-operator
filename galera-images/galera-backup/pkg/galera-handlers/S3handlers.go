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
	"galera-backup/pkg/models"
	"galera-backup/pkg/restapi/operations"
	"galera-backup/pkg/storage-provider/s3"
	"galera-backup/pkg/utils/targzip"
	"github.com/go-openapi/runtime/middleware"
	"github.com/go-openapi/swag"
	"github.com/sirupsen/logrus"
	"io"
)

func S3Backup() func(params operations.AddS3BackupParams) middleware.Responder{
	return func(params operations.AddS3BackupParams) middleware.Responder {
		s3provider := s3.S3StorageProvider{
			Endpoint: *params.S3.Endpoint,
			Bucket: *params.S3.Bucket,
			AccessKey: *params.S3.AccessKey,
			SecretKey: *params.S3.SecretKey,
		}
		if params.S3.Region == nil {
			s3provider.Region = "us-east-1"
		} else {
			s3provider.Region = *params.S3.Region
		}
		if params.S3.ForcePathStyle == nil {
			s3provider.ForcePathStyle = true
		} else {
			s3provider.ForcePathStyle = *params.S3.ForcePathStyle
		}

		StateMap[*params.S3.BackupName] = StateRunning

		provider, err := s3.NewProvider(&s3provider)
		if err != nil {
			logrus.Infof("error when creating s3 provider")
			StateMap[*params.S3.BackupName] = StateFailed
			return operations.NewAddS3BackupDefault(404).WithPayload(
				&models.Response{
					Status: swag.String("failed"),
					Message: swag.String("unable to access to s3"),
				})
		}

//		backupName := fmt.Sprintf("%s.%s.sql.gz", *params.S3.BackupName, time.Now().UTC().Format("20060102150405"))

		pr, pw := io.Pipe()

		go func() {
			defer pw.Close()
			errgow := targzip.TarGzip(*params.S3.BackupDir, pw)
			if errgow != nil {
				StateMap[*params.S3.BackupName] = StateFailed
				logrus.Infof("error during backup with tar+gzip operation : %s", errgow)
			}
		}()

		go func() {
			errgor := provider.Store(*params.S3.BackupName, pr)
			if errgor != nil {
				StateMap[*params.S3.BackupName] = StateFailed
				logrus.Infof("error with s3 storage during backup : %s", errgor)
				return
			}
			StateMap[*params.S3.BackupName] = StateCompleted
		}()

		return operations.NewGetS3BackupOK().WithPayload(
			&models.Response{
				Status: swag.String("scheduled"),
				Message: swag.String(fmt.Sprintf("%s is scheduled", *params.S3.BackupName)),
			})
	}
}

func S3Restore() func(params operations.GetS3BackupParams) middleware.Responder {
	return func(params operations.GetS3BackupParams) middleware.Responder {
		s3provider := s3.S3StorageProvider{
			Endpoint: *params.S3.Endpoint,
			Bucket: *params.S3.Bucket,
			AccessKey: *params.S3.AccessKey,
			SecretKey: *params.S3.SecretKey,
		}
		if params.S3.Region == nil {
			s3provider.Region = "us-east-1"
		} else {
			s3provider.Region = *params.S3.Region
		}
		if params.S3.ForcePathStyle == nil {
			s3provider.ForcePathStyle = true
		} else {
			s3provider.ForcePathStyle = *params.S3.ForcePathStyle
		}

		StateMap[*params.S3.BackupName] = StateRunning

		provider, err := s3.NewProvider(&s3provider)
		if err != nil {
			logrus.Infof("error when creating s3 provider")
			StateMap[*params.S3.BackupName] = StateFailed
			return operations.NewAddS3BackupDefault(404).WithPayload(
				&models.Response{
					Status: swag.String("failed"),
					Message: swag.String("unable to access to s3"),
				})
		}

		pr, err := provider.Retrieve(*params.S3.BackupName)
		if err != nil {
			StateMap[*params.S3.BackupName] = StateFailed
			logrus.Infof("error with s3 storage during restore : %s", err)
			return operations.NewAddS3BackupDefault(404).WithPayload(
				&models.Response{
					Status: swag.String("failed"),
					Message: swag.String("error with s3 storage during restore"),
				})
		}

		go func() {
			defer pr.Close()
			errgor := targzip.UnTarGzip(*params.S3.BackupDir, pr)
			if errgor != nil {
				StateMap[*params.S3.BackupName] = StateFailed
				logrus.Infof("error during restore with tar+gzip operation")
				return
			}
			StateMap[*params.S3.BackupName] = StateCompleted
		}()

		return operations.NewGetS3BackupOK().WithPayload(
			&models.Response{
				Status: swag.String("scheduled"),
				Message: swag.String(fmt.Sprintf("%s is scheduled", *params.S3.BackupName)),
			})
	}
}
