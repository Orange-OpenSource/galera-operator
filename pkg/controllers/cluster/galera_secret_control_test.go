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
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"testing"
)

func TestGetGaleraCreds(t *testing.T) {
	name := "test-gal"
	creds := newSecretForGalera(name)
	config := newConfigMapForGalera(name)

	galera := newGalera(name, creds.Name, config.Name,3)

	secretIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	secretIndexer.Add(creds)
	secretLister := corelisters.NewSecretLister(secretIndexer)
	control := NewRealGaleraSecretControl(secretLister)

	if credsMap, err := control.GetGaleraCreds(galera); err != nil {
		t.Errorf("Failed to retrieve Secret error: %s", err)
	} else {
		if len(credsMap) != 2 {
			t.Errorf("Error, expected 2 key-value, get %d", len(credsMap))
		}
		if credsMap["user"] != "root" {
			t.Errorf("Error, user value not provided or incorrect")
		}
		if credsMap["password"] != "test" {
			t.Errorf("Error, password value not provided or incorrect")
		}
	}
}

func TestGetGaleraCreds_GetSecretFailure(t *testing.T) {
	name := "test-gal"
	creds := newSecretForGalera(name)
	config := newConfigMapForGalera(name)

	galera := newGalera(name, creds.Name, config.Name,3)

	secretIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	secretLister := corelisters.NewSecretLister(secretIndexer)
	control := NewRealGaleraSecretControl(secretLister)

	if _, err := control.GetGaleraCreds(galera); err == nil {
		t.Errorf("Error finding secret")
	}
}