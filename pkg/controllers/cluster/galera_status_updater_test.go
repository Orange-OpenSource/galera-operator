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
	"errors"
	apigalera "galera-operator/pkg/apis/apigalera/v1beta2"
	"galera-operator/pkg/client/clientset/versioned/fake"
	galeralisters "galera-operator/pkg/client/listers/apigalera/v1beta2"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	core "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
	"testing"
)

func TestGaleraStatusUpdater_UpdatesGaleraStatus(t *testing.T) {
	name := "test-gal"
	creds := newSecretForGalera(name)
	config := newConfigMapForGalera(name)

	galera := newGalera(name, creds.Name, config.Name,3)

	status := apigalera.GaleraStatus{ObservedGeneration: 1, Replicas: 2}
	fakeClient := &fake.Clientset{}
	updater := NewRealGaleraStatusUpdater(fakeClient, nil)

	fakeClient.AddReactor("update", "galeras", func(action core.Action) (bool, runtime.Object, error) {
		update := action.(core.UpdateAction)
		return true, update.GetObject(), nil
	})

	if err := updater.UpdateGaleraStatus(galera, &status); err != nil {
		t.Errorf("Error returned on successful status update: %s", err)
	}

	if galera.Status.Replicas != 2 {
		t.Errorf("UpdateGaleraStatus mutated the galeras replicas %d", galera.Status.Replicas)
	}
}


func TestGaleraStatusUpdater_UpdatesObservedGeneration(t *testing.T) {
	name := "test-gal"
	creds := newSecretForGalera(name)
	config := newConfigMapForGalera(name)

	galera := newGalera(name, creds.Name, config.Name,3)

	status := apigalera.GaleraStatus{ObservedGeneration: 3, Replicas: 2}
	fakeClient := &fake.Clientset{}
	updater := NewRealGaleraStatusUpdater(fakeClient, nil)
	fakeClient.AddReactor("update", "galeras", func(action core.Action) (bool, runtime.Object, error) {
		update := action.(core.UpdateAction)
		gal := update.GetObject().(*apigalera.Galera)
		if gal.Status.ObservedGeneration != 3 {
			t.Errorf("expected observedGeneration to be synced with generation for galera %q", galera.Name)
		}
		return true, gal, nil
	})
	if err := updater.UpdateGaleraStatus(galera, &status); err != nil {
		t.Errorf("Error returned on successful status update: %s", err)
	}
}

func TestGaleraStatusUpdater_UpdateReplicasFailure(t *testing.T) {
	name := "test-gal"
	creds := newSecretForGalera(name)
	config := newConfigMapForGalera(name)

	galera := newGalera(name, creds.Name, config.Name,3)

	status := apigalera.GaleraStatus{ObservedGeneration: 3, Replicas: 2}
	fakeClient := &fake.Clientset{}
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	indexer.Add(galera)
	setLister := galeralisters.NewGaleraLister(indexer)
	updater := NewRealGaleraStatusUpdater(fakeClient, setLister)
	fakeClient.AddReactor("update", "galeras", func(action core.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewInternalError(errors.New("API server down"))
	})
	if err := updater.UpdateGaleraStatus(galera, &status); err == nil {
		t.Error("Failed update did not return error")
	}
}

func TestGaleraStatusUpdater_UpdateReplicasConflict(t *testing.T) {
	name := "test-gal"
	creds := newSecretForGalera(name)
	config := newConfigMapForGalera(name)

	galera := newGalera(name, creds.Name, config.Name,3)

	status := apigalera.GaleraStatus{ObservedGeneration: 3, Replicas: 2}
	conflict := false
	fakeClient := &fake.Clientset{}
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	indexer.Add(galera)
	setLister := galeralisters.NewGaleraLister(indexer)
	updater := NewRealGaleraStatusUpdater(fakeClient, setLister)
	fakeClient.AddReactor("update", "galeras", func(action core.Action) (bool, runtime.Object, error) {
		update := action.(core.UpdateAction)
		if !conflict {
			conflict = true
			return true, update.GetObject(), apierrors.NewConflict(action.GetResource().GroupResource(), galera.Name, errors.New("object already exists"))
		}
		return true, update.GetObject(), nil

	})
	if err := updater.UpdateGaleraStatus(galera, &status); err != nil {
		t.Errorf("UpdateGaleraStatus returned an error: %s", err)
	}
	if galera.Status.Replicas != 2 {
		t.Errorf("UpdateGaleraStatus mutated the sets replicas %d", galera.Status.Replicas)
	}
}

func TestGaleraStatusUpdater_UpdateReplicasConflictFailure(t *testing.T) {
	name := "test-gal"
	creds := newSecretForGalera(name)
	config := newConfigMapForGalera(name)

	galera := newGalera(name, creds.Name, config.Name,3)

	status := apigalera.GaleraStatus{ObservedGeneration: 3, Replicas: 2}
	fakeClient := &fake.Clientset{}
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	indexer.Add(galera)
	setLister := galeralisters.NewGaleraLister(indexer)
	updater := NewRealGaleraStatusUpdater(fakeClient, setLister)
	fakeClient.AddReactor("update", "galeras", func(action core.Action) (bool, runtime.Object, error) {
		update := action.(core.UpdateAction)
		return true, update.GetObject(), apierrors.NewConflict(action.GetResource().GroupResource(), galera.Name, errors.New("object already exists"))
	})
	if err := updater.UpdateGaleraStatus(galera, &status); err == nil {
		t.Error("UpdateGaleraStatus failed to return an error on get failure")
	}
}
