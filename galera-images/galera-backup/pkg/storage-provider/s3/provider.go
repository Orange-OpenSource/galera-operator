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

package s3

import (
	"io"

	"github.com/aws/aws-sdk-go/aws"
	s3credentials "github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/sirupsen/logrus"
	"github.com/pkg/errors"

//	"k8s.io/apimachinery/pkg/util/validation/field"
)

type S3StorageProvider struct {
	Region string
	Endpoint string
	Bucket string
	ForcePathStyle bool
	AccessKey string
	SecretKey string
}


// Provider is storage implementation of provider.Interface.
type Provider struct {
	S3StorageProvider

	s3 *s3.S3
	s3Uploader *s3manager.Uploader
}

// NewProvider creates a new S3 (compatible) storage provider.
func NewProvider(provider *S3StorageProvider) (*Provider, error) {
	sess, err := session.NewSession(
		aws.NewConfig().
			WithCredentials(s3credentials.NewStaticCredentials(provider.AccessKey, provider.SecretKey, "")).
			WithEndpoint(provider.Endpoint).
			WithRegion(provider.Region).
			WithS3ForcePathStyle(provider.ForcePathStyle).
			WithDisableSSL(false),
	)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if _, err := sess.Config.Credentials.Get(); err != nil {
		return nil, errors.WithStack(err)
	}

	return &Provider{
		S3StorageProvider: *provider,
		s3:                s3.New(sess),
		s3Uploader:        s3manager.NewUploader(sess),
	}, nil
}

// Store the given data at the given key.
func (p *Provider) Store(key string, body io.ReadCloser) error {
	logrus.Infof("Storing backup (provider=\"S3\", endpoint=%q, bucket=%q, key=%q)", p.Endpoint, p.Bucket, key)

	defer body.Close()

	_, err := p.s3Uploader.Upload(&s3manager.UploadInput{
		Bucket: &p.Bucket,
		Key:    &key,
		Body:   body,
	})
	return errors.Wrapf(err, "error storing backup (provider=\"S3\", endpoint=%q, bucket=%q, key=%q)", p.Endpoint, p.Bucket, key)
}

// Retrieve the given key from S3 storage service.
func (p *Provider) Retrieve(key string) (io.ReadCloser, error) {
	logrus.Infof("Retrieving backup (provider=\"s3\", endpoint=%q, bucket=%q, key=%q)", p.Endpoint, p.Bucket, key)

	obj, err := p.s3.GetObject(&s3.GetObjectInput{Bucket: &p.Bucket, Key: &key})
	if err != nil {
		return nil, errors.Wrapf(err, "error retrieving backup (provider='S3', endpoint='%s', bucket='%s', key='%s')", p.Endpoint, p.Bucket, key)
	}

	return obj.Body, nil
}

/*
// getCredentials gets an accesskey and secretKey from the provided map.
func getCredentials(credentials map[string]string) (string, string, error) {
	allErrs := field.ErrorList{}
	fldPath := field.NewPath("data")

	if credentials == nil {
		return "", "", errors.New("no credentials provided")
	}

	accessKey, ok := credentials["accessKey"]
	if !ok {
		allErrs = append(allErrs, field.Required(fldPath.Child("accessKey"), ""))
	}
	secretKey, ok := credentials["secretKey"]
	if !ok {
		allErrs = append(allErrs, field.Required(fldPath.Child("secretKey"), ""))
	}

	if len(allErrs) > 0 {
		return "", "", allErrs.ToAggregate()
	}

	return accessKey, secretKey, nil
}
*/
