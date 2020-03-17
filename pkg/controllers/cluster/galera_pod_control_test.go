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
	"fmt"
	apigalera "galera-operator/pkg/apis/apigalera/v1beta2"
	"galera-operator/pkg/utils/constants"
	"github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	corelisters "k8s.io/client-go/listers/core/v1"
	core "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	_ "k8s.io/kubernetes/pkg/apis/apps/install"
	_ "k8s.io/kubernetes/pkg/apis/core/install"
	"strings"
	"testing"
)

type fakeIndexer struct {
	cache.Indexer
	getError error
}

func (f fakeIndexer) GetByKey(key string) (interface{}, bool, error) {
	return nil, true, f.getError
}

func collectEvents(source <-chan string) []string {
	done := false
	events := make([]string, 0)
	for !done {
		select {
		case event := <-source:
			events = append(events, event)
		default:
			done = true
		}
	}
	return events
}

func TestGaleraPodControl_CreatePodAndClaim(t *testing.T) {
	recorder := record.NewFakeRecorder(10)
	name := "test-gal"
	revision := "59889f974c"
	creds := newSecretForGalera(name)
	config := newConfigMapForGalera(name)
	galera := newGalera(name, creds.Name, config.Name,3)
	credsMap := make(map[string]string, len(creds.Data))
	for k, v := range creds.Data {
		credsMap[k] = string(v)
	}
	pod := newGaleraPod(
		galera,
		revision,
		createUniquePodName(galera.Name, []string{}, []string{}),
		apigalera.RoleWriter,
		apigalera.StateCluster,
		"",
		constants.BootstrapImage,
		constants.BackupImage,
		true,
		credsMap,
	)

	fakeClient := &fake.Clientset{}
	pvcIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	pvcLister := corelisters.NewPersistentVolumeClaimLister(pvcIndexer)
	control := NewRealGaleraPodControl(fakeClient, nil, nil, pvcLister, recorder)
//	fakeClient.AddReactor("get", "persistentvolumeclaims", func(action core.Action) (bool, runtime.Object, error) {
//		return true, nil, apierrors.NewNotFound(action.GetResource().GroupResource(), action.GetResource().Resource)
//	})
	fakeClient.AddReactor("create", "persistentvolumeclaims", func(action core.Action) (bool, runtime.Object, error) {
		create := action.(core.CreateAction)
		return true, create.GetObject(), nil
	})
	fakeClient.AddReactor("create", "pods", func(action core.Action) (bool, runtime.Object, error) {
		create := action.(core.CreateAction)
		return true, create.GetObject(), nil
	})
	if err := control.CreatePodAndClaim(galera, pod, apigalera.RoleWriter); err != nil {
		t.Errorf("Failed to create Pod error: %s", err)
	}
	events := collectEvents(recorder.Events)
	if eventCount := len(events); eventCount != 2 {
		t.Errorf("Expected 2 events for successful create found %d", eventCount)
	}
	for i := range events {
		if !strings.Contains(events[i], v1.EventTypeNormal) {
			t.Errorf("Found unexpected non-normal event %s", events[i])
		}
	}
}

func TestGaleraPodControl_CreateBackupPodAndClaim(t *testing.T) {
	recorder := record.NewFakeRecorder(10)
	name := "test-gal"
	revision := "59889f974c"
	creds := newSecretForGalera(name)
	config := newConfigMapForGalera(name)
	galera := newGalera(name, creds.Name, config.Name,3)
	credsMap := make(map[string]string, len(creds.Data))
	for k, v := range creds.Data {
		credsMap[k] = string(v)
	}
	pod := newGaleraPod(
		galera,
		revision,
		createUniquePodName(galera.Name, []string{}, []string{}),
		apigalera.Backup,
		apigalera.StateCluster,
		"",
		constants.BootstrapImage,
		constants.BackupImage,
		true,
		credsMap,
	)

	fakeClient := &fake.Clientset{}
	pvcIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	pvcLister := corelisters.NewPersistentVolumeClaimLister(pvcIndexer)
	control := NewRealGaleraPodControl(fakeClient, nil, nil, pvcLister, recorder)
	fakeClient.AddReactor("create", "persistentvolumeclaims", func(action core.Action) (bool, runtime.Object, error) {
		create := action.(core.CreateAction)
		return true, create.GetObject(), nil
	})
	fakeClient.AddReactor("create", "pods", func(action core.Action) (bool, runtime.Object, error) {
		create := action.(core.CreateAction)
		return true, create.GetObject(), nil
	})
	if err := control.CreatePodAndClaim(galera, pod, apigalera.Backup); err != nil {
		t.Errorf("Failed to create Pod error: %s", err)
	}
	events := collectEvents(recorder.Events)
	if eventCount := len(events); eventCount != 3 {
		t.Errorf("Expected 3 events for successful create found %d", eventCount)
	}
	for i := range events {
		if !strings.Contains(events[i], v1.EventTypeNormal) {
			t.Errorf("Found unexpected non-normal event %s", events[i])
		}
	}
}

func TestGaleraPodControl_CreatePodAndClaim_PodAndPVCExist(t *testing.T) {
	recorder := record.NewFakeRecorder(10)
	name := "test-gal"
	revision := "59889f974c"
	creds := newSecretForGalera(name)
	config := newConfigMapForGalera(name)
	galera := newGalera(name, creds.Name, config.Name,3)
	credsMap := make(map[string]string, len(creds.Data))
	for k, v := range creds.Data {
		credsMap[k] = string(v)
	}
	pod := newGaleraPod(
		galera,
		revision,
		createUniquePodName(galera.Name, []string{}, []string{}),
		apigalera.RoleWriter,
		apigalera.StateCluster,
		"",
		constants.BootstrapImage,
		constants.BackupImage,
		true,
		credsMap,
	)
	pvc := getPersistentVolumeClaim(galera, pod)

	fakeClient := &fake.Clientset{}
	pvcIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	pvcIndexer.Add(pvc)
	pvcLister := corelisters.NewPersistentVolumeClaimLister(pvcIndexer)
	control := NewRealGaleraPodControl(fakeClient, nil, nil, pvcLister, recorder)
// 	control.SetLogLevel(logrus.ErrorLevel)
//	fakeClient.AddReactor("get", "persistentvolumeclaims", func(action core.Action) (bool, runtime.Object, error) {
//		return true, nil, apierrors.NewNotFound(action.GetResource().GroupResource(), action.GetResource().Resource)
// 	})

	fakeClient.AddReactor("create", "persistentvolumeclaims", func(action core.Action) (bool, runtime.Object, error) {
		create := action.(core.CreateAction)
		return true, create.GetObject(), nil
	})
	fakeClient.AddReactor("create", "pods", func(action core.Action) (bool, runtime.Object, error) {
		return true, pod, apierrors.NewAlreadyExists(action.GetResource().GroupResource(), pod.Name)
	})
	if err := control.CreatePodAndClaim(galera, pod, apigalera.RoleWriter); !apierrors.IsAlreadyExists(err) {
		t.Errorf("Failed to create Pod error: %s", err)
	}
	events := collectEvents(recorder.Events)

	if eventCount := len(events); eventCount != 1 {
		t.Errorf("Pod and PVC exist: got %d events, but want 1", eventCount)
		for i := range events {
			t.Log(events[i])
		}
	}
}

func TestGaleraPodControl_CreatePodAndClaim_PVCExists(t *testing.T) {
	recorder := record.NewFakeRecorder(10)
	name := "test-gal"
	revision := "59889f974c"
	creds := newSecretForGalera(name)
	config := newConfigMapForGalera(name)
	galera := newGalera(name, creds.Name, config.Name,3)
	credsMap := make(map[string]string, len(creds.Data))
	for k, v := range creds.Data {
		credsMap[k] = string(v)
	}
	pod := newGaleraPod(
		galera,
		revision,
		createUniquePodName(galera.Name, []string{}, []string{}),
		apigalera.RoleWriter,
		apigalera.StateCluster,
		"",
		constants.BootstrapImage,
		constants.BackupImage,
		true,
		credsMap,
	)
	pvc := getPersistentVolumeClaim(galera, pod)

	fakeClient := &fake.Clientset{}
	pvcIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	pvcIndexer.Add(pvc)
	pvcLister := corelisters.NewPersistentVolumeClaimLister(pvcIndexer)
	control := NewRealGaleraPodControl(fakeClient, nil, nil, pvcLister, recorder)
	fakeClient.AddReactor("create", "persistentvolumeclaims", func(action core.Action) (bool, runtime.Object, error) {
		create := action.(core.CreateAction)
		return true, create.GetObject(), nil
	})
	fakeClient.AddReactor("create", "pods", func(action core.Action) (bool, runtime.Object, error) {
		create := action.(core.CreateAction)
		return true, create.GetObject(), nil
	})
	if err := control.CreatePodAndClaim(galera, pod, apigalera.RoleWriter); err != nil {
		t.Errorf("Failed to create Pod error: %s", err)
	}
	events := collectEvents(recorder.Events)

	if eventCount := len(events); eventCount != 2 {
		t.Errorf("Pod and PVC exist: got %d events, but want 2", eventCount)
		for i := range events {
			t.Log(events[i])
		}
	} else {
		event := fmt.Sprintf("Normal SuccessfulCreate create Pod %s in Galera %s/%s successful", pod.Name, galera.Namespace, galera.Name)
		if event != events[1] {
			t.Errorf("Received not the expected event : %b", event[1])
		}
	}
}

func TestGaleraPodControl_CreatePodAndClaim_PVCCreateFailure(t *testing.T) {
	recorder := record.NewFakeRecorder(10)
	name := "test-gal"
	revision := "59889f974c"
	creds := newSecretForGalera(name)
	config := newConfigMapForGalera(name)
	galera := newGalera(name, creds.Name, config.Name,3)
	credsMap := make(map[string]string, len(creds.Data))
	for k, v := range creds.Data {
		credsMap[k] = string(v)
	}
	pod := newGaleraPod(
		galera,
		revision,
		createUniquePodName(galera.Name, []string{}, []string{}),
		apigalera.RoleWriter,
		apigalera.StateCluster,
		"",
		constants.BootstrapImage,
		constants.BackupImage,
		true,
		credsMap,
	)
	pvc := getPersistentVolumeClaim(galera, pod)

	fakeClient := &fake.Clientset{}
	/*
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	gr := schema.GroupResource{
		Group:    pvc.GroupVersionKind().Group,
		Resource: "persistentvolumeclaims",
	}
	pvcIndexer := fakeIndexer{
		indexer,
		apierrors.NewNotFound(gr, pvc.Name),
	}
	pvcIndexer.Add(pvc)
	*/

	pvcIndexer := fakeIndexer{
		cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}),
		apierrors.NewInternalError(errors.New("API server down")),
	}
	pvcIndexer.Add(pvc)

	pvcLister := corelisters.NewPersistentVolumeClaimLister(pvcIndexer)
	control := NewRealGaleraPodControl(fakeClient, nil, nil, pvcLister, recorder)
	fakeClient.AddReactor("create", "pods", func(action core.Action) (bool, runtime.Object, error) {
		create := action.(core.CreateAction)
		return true, create.GetObject(), nil
	})
	if err := control.CreatePodAndClaim(galera, pod, apigalera.RoleWriter); err == nil {
		t.Error("Failed to produce error on PVC creation failure")
	} else {
		logrus.Infof("erreur : %s", err)
	}

	events := collectEvents(recorder.Events)
	if eventCount := len(events); eventCount != 1 {
		t.Errorf("PVC create failure: got %d events, but want 1", eventCount)
	}
	for i := range events {
		if !strings.Contains(events[i], v1.EventTypeWarning) {
			t.Errorf("Found unexpected non-warning event %s", events[i])
		}
	}
}


func TestGaleraPodControl_CreatePodAndClaim_CreatePodFailed(t *testing.T) {
	recorder := record.NewFakeRecorder(10)
	name := "test-gal"
	revision := "59889f974c"
	creds := newSecretForGalera(name)
	config := newConfigMapForGalera(name)
	galera := newGalera(name, creds.Name, config.Name,3)
	credsMap := make(map[string]string, len(creds.Data))
	for k, v := range creds.Data {
		credsMap[k] = string(v)
	}
	pod := newGaleraPod(
		galera,
		revision,
		createUniquePodName(galera.Name, []string{}, []string{}),
		apigalera.RoleWriter,
		apigalera.StateCluster,
		"",
		constants.BootstrapImage,
		constants.BackupImage,
		true,
		credsMap,
	)

	fakeClient := &fake.Clientset{}
	pvcIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	pvcLister := corelisters.NewPersistentVolumeClaimLister(pvcIndexer)
	control := NewRealGaleraPodControl(fakeClient, nil, nil, pvcLister, recorder)
	fakeClient.AddReactor("create", "pods", func(action core.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewInternalError(errors.New("API server down"))
	})
	if err := control.CreatePodAndClaim(galera, pod, apigalera.RoleWriter); err == nil {
		t.Error("Failed to produce error on Pod creation failure")
	}
	events := collectEvents(recorder.Events)
	if eventCount := len(events); eventCount != 2 {
		t.Errorf("Pod create failed: got %d events, but want 2", eventCount)
	} else if !strings.Contains(events[0], v1.EventTypeNormal) {
		t.Errorf("Found unexpected non-normal event %s", events[0])
	} else if !strings.Contains(events[1], v1.EventTypeWarning) {
		t.Errorf("Found unexpected non-warning event %s", events[1])

	}
}

func TestGaleraPodControl_PatchPodLabels(t *testing.T) {
	recorder := record.NewFakeRecorder(10)
	name := "test-gal"
	revision := "59889f974c"
	creds := newSecretForGalera(name)
	config := newConfigMapForGalera(name)
	galera := newGalera(name, creds.Name, config.Name,3)
	credsMap := make(map[string]string, len(creds.Data))
	for k, v := range creds.Data {
		credsMap[k] = string(v)
	}
	pod := newGaleraPod(
		galera,
		revision,
		createUniquePodName(galera.Name, []string{}, []string{}),
		apigalera.RoleWriter,
		apigalera.StateCluster,
		"",
		constants.BootstrapImage,
		constants.BackupImage,
		true,
		credsMap,
	)

	fakeClient := &fake.Clientset{}
	control := NewRealGaleraPodControl(fakeClient, nil, nil, nil, recorder)
	var patch []byte
	fakeClient.PrependReactor("patch", "pods", func(action core.Action) (bool, runtime.Object, error) {
		patch = action.(core.PatchAction).GetPatch()
		return true, nil, nil
	})
	if err := control.PatchPodLabels(galera, pod, RemoveRole()); err != nil {
		t.Errorf("Failed to patch Pod error: %s", err)
	}
	events := collectEvents(recorder.Events)
	if eventCount := len(events); eventCount != 1 {
		t.Errorf("Patching Pod Labels: got %d events, but want 1", eventCount)
		for i := range events {
			t.Log(events[i])
		}
	} else {
		expected := fmt.Sprintf("[{\"op\":\"replace\",\"path\":\"/metadata/labels\",\"value\":{\"foo\":\"bar\",\"galera.v1beta2.sql.databases/controller-revision-hash\":\"%s\",\"galera.v1beta2.sql.databases/galera-name\":\"%s\",\"galera.v1beta2.sql.databases/galera-namespace\":\"%s\",\"galera.v1beta2.sql.databases/reader\":\"false\",\"galera.v1beta2.sql.databases/state\":\"cluster\"}}]", revision, galera.Name, galera.Namespace)
		if expected != string(patch) {
			t.Errorf("The expected pod patch is not correct")
		}
		event := fmt.Sprintf("Normal SuccessfulPatch patch Pod %s in Galera %s/%s successful", pod.Name, galera.Namespace, galera.Name)
		if event != events[0] {
			t.Errorf("Not received the expected event : %s", events[0])
		}
	}
}

func TestGaleraPodControl_PatchClaimLabels(t *testing.T) {
	recorder := record.NewFakeRecorder(10)
	name := "test-gal"
	revision := "59889f974c"
	creds := newSecretForGalera(name)
	config := newConfigMapForGalera(name)
	galera := newGalera(name, creds.Name, config.Name,3)
	credsMap := make(map[string]string, len(creds.Data))
	for k, v := range creds.Data {
		credsMap[k] = string(v)
	}
	pod := newGaleraPod(
		galera,
		revision,
		createUniquePodName(galera.Name, []string{}, []string{}),
		apigalera.RoleWriter,
		apigalera.StateCluster,
		"",
		constants.BootstrapImage,
		constants.BackupImage,
		true,
		credsMap,
	)
	pvc := getPersistentVolumeClaim(galera, pod)

	newRev := "PATCHINGOK"

	fakeClient := &fake.Clientset{}
	pvcIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	pvcIndexer.Add(pvc)
	pvcLister := corelisters.NewPersistentVolumeClaimLister(pvcIndexer)
	control := NewRealGaleraPodControl(fakeClient, nil, nil, pvcLister, recorder)
	var patch []byte
	fakeClient.PrependReactor("patch", "persistentvolumeclaims", func(action core.Action) (bool, runtime.Object, error) {
		patch = action.(core.PatchAction).GetPatch()
		return true, nil, nil
	})
	if err := control.PatchClaimLabels(galera, pod, newRev); err != nil {
		t.Errorf("Failed to patch PVC error: %s", err)
	}
	events := collectEvents(recorder.Events)
	if eventCount := len(events); eventCount != 1 {
		t.Errorf("Patching Claim Labels: got %d events, but want 1", eventCount)
		for i := range events {
			t.Log(events[i])
		}
	} else {
		expected := fmt.Sprintf("[{\"op\":\"replace\",\"path\":\"/metadata/labels\",\"value\":{\"foo\":\"bar\",\"galera.v1beta2.sql.databases/controller-revision-hash\":\"%s\",\"galera.v1beta2.sql.databases/galera-name\":\"%s\",\"galera.v1beta2.sql.databases/galera-namespace\":\"%s\"}}]", newRev, galera.Name, galera.Namespace)
		if expected != string(patch) {
			t.Errorf("The expected pvc patch is not correct")
		}
		event := fmt.Sprintf("Normal SuccessfulPatch patch claim %s for pod %s in galera %s/%s success", pvc.Name, pod.Name, galera.Namespace, galera.Name)
		if event != events[0] {
			t.Errorf("Not received the expected event : %s", events[0])
		}
	}
}

func TestGaleraPodControl_ForceDeletePod(t *testing.T) {
	recorder := record.NewFakeRecorder(10)
	name := "test-gal"
	revision := "59889f974c"
	creds := newSecretForGalera(name)
	config := newConfigMapForGalera(name)
	galera := newGalera(name, creds.Name, config.Name,3)
	credsMap := make(map[string]string, len(creds.Data))
	for k, v := range creds.Data {
		credsMap[k] = string(v)
	}
	pod := newGaleraPod(
		galera,
		revision,
		createUniquePodName(galera.Name, []string{}, []string{}),
		apigalera.RoleWriter,
		apigalera.StateCluster,
		"",
		constants.BootstrapImage,
		constants.BackupImage,
		true,
		credsMap,
	)

	fakeClient := &fake.Clientset{}
	control := NewRealGaleraPodControl(fakeClient, nil, nil, nil, recorder)
	fakeClient.AddReactor("delete", "pods", func(action core.Action) (bool, runtime.Object, error) {
		return true, nil, nil
	})
	if err := control.ForceDeletePod(galera, pod); err != nil {
		t.Errorf("Error returned on successful delete: %s", err)
	}
	events := collectEvents(recorder.Events)
	if eventCount := len(events); eventCount != 1 {
		t.Errorf("delete successful: got %d events, but want 1", eventCount)
	} else if !strings.Contains(events[0], v1.EventTypeNormal) {
		t.Errorf("Found unexpected non-normal event %s", events[0])
	}
}

func TestGaleraPodControl_ForceDeletePod_DeleteFailure(t *testing.T) {
	recorder := record.NewFakeRecorder(10)
	name := "test-gal"
	revision := "59889f974c"
	creds := newSecretForGalera(name)
	config := newConfigMapForGalera(name)
	galera := newGalera(name, creds.Name, config.Name,3)
	credsMap := make(map[string]string, len(creds.Data))
	for k, v := range creds.Data {
		credsMap[k] = string(v)
	}
	pod := newGaleraPod(
		galera,
		revision,
		createUniquePodName(galera.Name, []string{}, []string{}),
		apigalera.RoleWriter,
		apigalera.StateCluster,
		"",
		constants.BootstrapImage,
		constants.BackupImage,
		true,
		credsMap,
	)

	fakeClient := &fake.Clientset{}
	control := NewRealGaleraPodControl(fakeClient, nil, nil, nil, recorder)
	fakeClient.AddReactor("delete", "pods", func(action core.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewInternalError(errors.New("API server down"))
	})
	if err := control.ForceDeletePod(galera, pod); err == nil {
		t.Error("Failed to return error on failed delete")
	}
	events := collectEvents(recorder.Events)
	if eventCount := len(events); eventCount != 1 {
		t.Errorf("delete failed: got %d events, but want 1", eventCount)
	} else if !strings.Contains(events[0], v1.EventTypeWarning) {
		t.Errorf("Found unexpected non-warning event %s", events[0])
	}
}


func TestGaleraPodControl_ForceDeletePodAndClaim(t *testing.T) {
	recorder := record.NewFakeRecorder(10)
	name := "test-gal"
	revision := "59889f974c"
	creds := newSecretForGalera(name)
	config := newConfigMapForGalera(name)
	galera := newGalera(name, creds.Name, config.Name,3)
	credsMap := make(map[string]string, len(creds.Data))
	for k, v := range creds.Data {
		credsMap[k] = string(v)
	}
	pod := newGaleraPod(
		galera,
		revision,
		createUniquePodName(galera.Name, []string{}, []string{}),
		apigalera.RoleWriter,
		apigalera.StateCluster,
		"",
		constants.BootstrapImage,
		constants.BackupImage,
		true,
		credsMap,
	)

	pvc := getPersistentVolumeClaim(galera, pod)

	fakeClient := &fake.Clientset{}
	pvcIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	pvcIndexer.Add(pvc)
	pvcLister := corelisters.NewPersistentVolumeClaimLister(pvcIndexer)
	control := NewRealGaleraPodControl(fakeClient, nil, nil, pvcLister, recorder)
	fakeClient.AddReactor("delete", "pods", func(action core.Action) (bool, runtime.Object, error) {
		return true, nil, nil
	})
	if err := control.ForceDeletePodAndClaim(galera, pod); err != nil {
		t.Errorf("Error returned on successful delete: %s", err)
	}
	events := collectEvents(recorder.Events)
	if eventCount := len(events); eventCount != 2 {
		t.Errorf("delete successful: got %d events, but want 2", eventCount)
	} else {
		if !strings.Contains(events[0], v1.EventTypeNormal) {
			t.Errorf("Found unexpected non-normal event %s", events[0])
		}
		if !strings.Contains(events[1], v1.EventTypeNormal) {
			t.Errorf("Found unexpected non-normal event %s", events[1])
		}
	}
}

func TestGaleraPodControl_GetClaimRevision(t *testing.T) {
	recorder := record.NewFakeRecorder(10)
	name := "test-gal"
	revision := "59889f974c"
	creds := newSecretForGalera(name)
	config := newConfigMapForGalera(name)
	galera := newGalera(name, creds.Name, config.Name, 3)
	credsMap := make(map[string]string, len(creds.Data))
	for k, v := range creds.Data {
		credsMap[k] = string(v)
	}
	pod := newGaleraPod(
		galera,
		revision,
		createUniquePodName(galera.Name, []string{}, []string{}),
		apigalera.RoleWriter,
		apigalera.StateCluster,
		"",
		constants.BootstrapImage,
		constants.BackupImage,
		true,
		credsMap,
	)

	pvc := getPersistentVolumeClaim(galera, pod)

	fakeClient := &fake.Clientset{}
	pvcIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	pvcIndexer.Add(pvc)
	pvcLister := corelisters.NewPersistentVolumeClaimLister(pvcIndexer)
	control := NewRealGaleraPodControl(fakeClient, nil, nil, pvcLister, recorder)
	if exist, rev, err := control.GetClaimRevision(galera.Namespace, pod.Name); err != nil {
		t.Errorf("Error returned on get claim revision: %s", err)
	} else {
		if exist != true {
			t.Errorf("Error returned : claim not found")
		}
		if rev != revision {
			t.Errorf("Error get revision %s instead of %s", rev, revision)
		}
	}
}