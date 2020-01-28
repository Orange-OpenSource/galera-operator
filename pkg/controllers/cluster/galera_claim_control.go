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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	clientset "k8s.io/client-go/kubernetes"
	pkgcontroller "k8s.io/kubernetes/pkg/controller"
)

// ClaimControlInterface is an interface that knows how to patch claims
// created as an interface to allow testing.
type ClaimControlInterface interface {
	PatchClaim(namespace, name string, data []byte) error
}

var _ ClaimControlInterface = &RealClaimControl{}

// RealClaimControl is the default implementation of ClaimControlInterface.
type RealClaimControl struct {
	KubeClient clientset.Interface
	//	Recorder   record.EventRecorder
}

func (r RealClaimControl) PatchClaim(namespace, name string, data []byte) error {
	_, err := r.KubeClient.CoreV1().PersistentVolumeClaims(namespace).Patch(name, types.StrategicMergePatchType, data)
	return err
}


type ClaimControllerRefManager struct {
	pkgcontroller.BaseControllerRefManager
	controllerKind schema.GroupVersionKind
	claimControl   ClaimControlInterface
}

// NewClaimControllerRefManager returns a ClaimControllerRefManager that exposes
// methods to manage the controllerRef of claims.
//
// The CanAdopt() function can be used to perform a potentially expensive check
// (such as a live GET from the API server) prior to the first adoption.
// It will only be called (at most once) if an adoption is actually attempted.
// If CanAdopt() returns a non-nil error, all adoptions will fail.
//
// NOTE: Once CanAdopt() is called, it will not be called again by the same
//       ClaimControllerRefManager instance. Create a new instance if it makes
//       sense to check CanAdopt() again (e.g. in a different sync pass).
func NewClaimControllerRefManager(
	claimControl ClaimControlInterface,
	controller metav1.Object,
	selector labels.Selector,
	controllerKind schema.GroupVersionKind,
	canAdopt func() error,
) *ClaimControllerRefManager {
	return &ClaimControllerRefManager{
		BaseControllerRefManager: pkgcontroller.BaseControllerRefManager{
			Controller:   controller,
			Selector:     selector,
			CanAdoptFunc: canAdopt,
		},
		controllerKind: controllerKind,
		claimControl:   claimControl,
	}
}

// ClaimClaims tries to take ownership of a list of claims.
//
// It will reconcile the following:
//   * Adopt orphans if the selector matches.
//   * Release owned objects if the selector no longer matches.
//
// Optional: If one or more filters are specified, a claim will only be claimed if
// all filters return true.
//
// A non-nil error is returned if some form of reconciliation was attempted and
// failed. Usually, controllers should try again later in case reconciliation
// is still needed.
//
// If the error is nil, either the reconciliation succeeded, or no
// reconciliation was necessary. The list of claims that you now own is returned.
func (m *ClaimControllerRefManager) ClaimClaims(claims []*corev1.PersistentVolumeClaim, filters ...func(claim *corev1.PersistentVolumeClaim) bool) ([]*corev1.PersistentVolumeClaim, error) {
	var claimed []*corev1.PersistentVolumeClaim
	var errlist []error

	match := func(obj metav1.Object) bool {
		claim := obj.(*corev1.PersistentVolumeClaim)
		// Check selector first so filters only run on potentially matching claims.
		if !m.Selector.Matches(labels.Set(claim.Labels)) {
			return false
		}
		for _, filter := range filters {
			if !filter(claim) {
				return false
			}
		}
		return true
	}
	adopt := func(obj metav1.Object) error {
		return m.AdoptClaim(obj.(*corev1.PersistentVolumeClaim))
	}
	release := func(obj metav1.Object) error {
		return m.ReleaseClaim(obj.(*corev1.PersistentVolumeClaim))
	}

	for _, claim := range claims {
		ok, err := m.ClaimObject(claim, match, adopt, release)
		if err != nil {
			errlist = append(errlist, err)
			continue
		}
		if ok {
			claimed = append(claimed, claim)
		}
	}
	return claimed, utilerrors.NewAggregate(errlist)
}

// AdoptClaim sends a patch to take control of the claim. It returns the error if
// the patching fails.
func (m *ClaimControllerRefManager) AdoptClaim(claim *corev1.PersistentVolumeClaim) error {
	if err := m.CanAdopt(); err != nil {
		return fmt.Errorf("can't adopt claim %v/%v (%v): %v", claim.Namespace, claim.Name, claim.UID, err)
	}
	// Note that ValidateOwnerReferences() will reject this patch if another
	// OwnerReference exists with controller=true.
	addControllerPatch := fmt.Sprintf(
		`{"metadata":{"ownerReferences":[{"apiVersion":"%s","kind":"%s","name":"%s","uid":"%s","controller":true,"blockOwnerDeletion":true}],"uid":"%s"}}`,
		m.controllerKind.GroupVersion(), m.controllerKind.Kind,
		m.Controller.GetName(), m.Controller.GetUID(), claim.UID)
	return m.claimControl.PatchClaim(claim.Namespace, claim.Name, []byte(addControllerPatch))
}

// ReleaseClaim sends a patch to free the claim from the control of the controller.
// It returns the error if the patching fails. 404 and 422 errors are ignored.
func (m *ClaimControllerRefManager) ReleaseClaim(claim *corev1.PersistentVolumeClaim) error {
	//	glog.V(2).Infof("patching claim %s_%s to remove its controllerRef to %s/%s:%s",
	//		claim.Namespace, claim.Name, m.controllerKind.GroupVersion(), m.controllerKind.Kind, m.Controller.GetName())
	deleteOwnerRefPatch := fmt.Sprintf(`{"metadata":{"ownerReferences":[{"$patch":"delete","uid":"%s"}],"uid":"%s"}}`, m.Controller.GetUID(), claim.UID)
	err := m.claimControl.PatchClaim(claim.Namespace, claim.Name, []byte(deleteOwnerRefPatch))
	if err != nil {
		if errors.IsNotFound(err) {
			// If the claim no longer exists, ignore it.
			return nil
		}
		if errors.IsInvalid(err) {
			// Invalid error will be returned in two cases: 1. the claim
			// has no owner reference, 2. the uid of the claim doesn't
			// match, which means the claim is deleted and then recreated.
			// In both cases, the error can be ignored.

			// TODO: If the claim has owner references, but none of them
			// has the owner.UID, server will silently ignore the patch.
			// Investigate why.
			return nil
		}
	}
	return err
}
