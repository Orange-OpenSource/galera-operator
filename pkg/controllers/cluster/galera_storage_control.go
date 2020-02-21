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

package cluster

import (
	"fmt"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	storagelisters "k8s.io/client-go/listers/storage/v1"
)


// GaleraStorageControlInterface defines the interface that GaleraController uses to access Storage
// used by Galera clusters. It is implemented as an interface to provide for testing fakes.
type GaleraStorageControlInterface interface {
	// GetNameDefaultStorageClass returns the name of the default storage class. If no default class
	// exists, it return an empty string as name of the default storage class. Error is returned if we
	// cannot list the storage class
	GetNameDefaultStorageClass() (string, error)
}

func NewRealGaleraStorageControl(storageClassLister storagelisters.StorageClassLister)  GaleraStorageControlInterface {
	return &realGaleraStorageControl{storageClassLister}
}

// realGaleraStorageControl implements GaleraStorageControlInterface using a clientset.Interface
// to communicate with the  API server. The struct is package private as the internal details are irrelevant to
// importing packages.
type realGaleraStorageControl struct {
	storageClassLister storagelisters.StorageClassLister
}

func (gsc *realGaleraStorageControl) GetNameDefaultStorageClass() (string, error) {
//	et []*v1.StorageClass, err error
	storageClasses, err := gsc.storageClassLister.List(labels.Everything())
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("unable to list storage classes"))
		return "", err
	}

	for _, sc := range storageClasses {
		annotations := sc.GetAnnotations()
		if annotations["storageclass.kubernetes.io/is-default-class"] == "true" {
			return sc.Name, nil
		}
	}

	return "", nil
}

var _ GaleraStorageControlInterface = &realGaleraStorageControl{}