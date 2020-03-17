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
	"bytes"
	"encoding/json"
	"fmt"
	apigalera "galera-operator/pkg/apis/apigalera/v1beta2"
	pkggalera "galera-operator/pkg/galera"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1beta1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/client-go/kubernetes/scheme"
	podutil "k8s.io/kubernetes/pkg/api/v1/pod"
	"k8s.io/kubernetes/pkg/controller/history"
	"regexp"
	"strings"
)

// maxUpdateRetries is the maximum number of retries used for update conflict resolution prior to failure
//const maxUpdateRetries = 10

// updateConflictError is the error used to indicate that the maximum number of retries against the API server have
// been attempted and we need to back off
//var updateConflictError = fmt.Errorf("aborting update after %d attempts", maxUpdateRetries)
var patchCodec = scheme.Codecs.LegacyCodec(apigalera.SchemeGroupVersion)

// galeraParentRegex is a regular expression that extracts the parent galera name and suffix from a pod or a claim name
var galeraParentRegex = regexp.MustCompile("(.*)-([0-9a-z]+)$")

// getPodParentNameAndSuffix gets the name of pod's parent galera and pod's suffix as extracted from its Name. If
// the pod was not created by a galera, its parent is considered to be empty string, and its suffix is considered
// to be empty string
func getPodParentNameAndSuffix(pod *corev1.Pod) (string, string) {
	parent := ""
	suffix := ""
	subMatches := galeraParentRegex.FindStringSubmatch(pod.Name)
	if len(subMatches) < 3 {
		return parent, suffix
	}
	parent = subMatches[1]
	suffix = subMatches[2]
	return parent, suffix
}

// getPodParentName gets the name of pod's parent Galera. If pod has not parent, the empty string is returned.
func getPodParentName(pod *corev1.Pod) string {
	parent, _ := getPodParentNameAndSuffix(pod)
	return parent
}

//  getPodSuffix gets pod's suffix. If pod has no suffix, the empty string is returned.
func getPodSuffix(pod *corev1.Pod) string {
	_, suffix := getPodParentNameAndSuffix(pod)
	return suffix
}

// getClaimParentNameAndSuffix gets the name of claim's parent Galera and claim's suffix as extracted from its Name. If
// the claim was not created by a Galera, its parent is considered to be empty string, and its suffix is considered
// to be empty string. The last parameter indicates if the clim is a backup claim or not
func getClaimParentNameAndSuffix(claim *corev1.PersistentVolumeClaim) (string, string, bool) {
	parent := ""
	suffix := ""
	subMatches := galeraParentRegex.FindStringSubmatch(claim.Name)
	if len(subMatches) < 3 {
		return parent, suffix, false
	}
	parent = subMatches[1]
	suffix = subMatches[2]

	if strings.HasPrefix(parent, apigalera.ClaimPrefix) {
		return strings.TrimPrefix(parent, apigalera.ClaimPrefix), suffix, false
	}
	if strings.HasPrefix(parent, apigalera.BackupClaimPrefix) {
		return strings.TrimPrefix(parent, apigalera.BackupClaimPrefix), suffix, true
	}
	return "", "", false
}

// getClaimParentName gets the name of claim's parent Galera. If claim has not parent, the empty string is returned.
func getClaimParentName(claim *corev1.PersistentVolumeClaim) string {
	parent, _, _ := getClaimParentNameAndSuffix(claim)
	return parent
}

// getClaimSuffix gets Claim's suffix. If claim has no suffix, the empty string is returned.
func getClaimSuffix(claim *corev1.PersistentVolumeClaim) string {
	_, suffix, _ := getClaimParentNameAndSuffix(claim)
	return suffix
}

// getClaimSuffixAndBackup gets claim's suffix and backup. If claim has no suffix, the empty string is returned.
func getClaimSuffixAndBackup(claim *corev1.PersistentVolumeClaim) (string, bool) {
	_, suffix, backup := getClaimParentNameAndSuffix(claim)
	return suffix, backup
}

func buildPodName(name, suffix string) string {
	return fmt.Sprintf("%s-%s", name, suffix)
}

// getPodName gets the name of galera's child pod with a suffix
func getPodName(galera *apigalera.Galera, suffix string) string {
	clusterName := galera.Name
	if len(clusterName) > apigalera.MaxNameLength {
		clusterName = clusterName[:apigalera.MaxNameLength]
	}
	return buildPodName(clusterName, suffix)
}

// createUniquePodName returns the name of galera's child pod based on the clusterName. Two slices are used,
// 'existing' that lists all suffixes used by existing pods, and 'available' that lists all suffixes used by
// persistent volume claims and not used by an existing pod
func createUniquePodName(clusterName string, existing, available []string) string {
	var suffix string

	if len(available) > 0 {
		suffix = available[0]
	} else {
		unique := false

		for unique == false {
			suffix = utilrand.String(apigalera.RandomSuffixLength)
			unique = true
			for _, s := range existing {
				if s == suffix {
					unique = false
				}
			}
		}
	}

	if len(clusterName) > apigalera.MaxNameLength {
		clusterName = clusterName[:apigalera.MaxNameLength]
	}
	return buildPodName(clusterName, suffix)
}

func getPersistentVolumeClaimName(galera *apigalera.Galera, suffix string) string {
	return pkggalera.BuildClaimNameForGalera(getPodName(galera, suffix))
}

func getPersistentVolumeClaimNameForPod(podName string) string {
	return pkggalera.BuildClaimNameForGalera(podName)
}

func getBackupPersistentVolumeClaimName(galera *apigalera.Galera, suffix string) string {
	return pkggalera.BuildBackupClaimNameForGalera(getPodName(galera, suffix))
}

// isPodMemberOf tests if pod is a member of galera.
func isPodMemberOf(galera *apigalera.Galera, pod *corev1.Pod) bool {
	return getPodParentName(pod) == galera.Name
}

// isClaimMemberOf tests if this claim is a member of this galera
func isClaimMemberOf(galera *apigalera.Galera, claim *corev1.PersistentVolumeClaim) bool {
	return getClaimParentName(claim) == galera.Name
}

// getPersistentVolumeClaim returns a data claim matching the given galera and pod
func getPersistentVolumeClaim(galera *apigalera.Galera, pod *corev1.Pod) *corev1.PersistentVolumeClaim {
	return pkggalera.CreateGaleraClaim(
		&galera.Spec,
		galera.Labels,
		getPersistentVolumeClaimName(galera, getPodSuffix(pod)),
		galera.Name,
		galera.Namespace,
		getPodRevision(pod),
		galera.AsOwner(),
		)
}

// getBackupPersistentVolumeClaim returns a backup claim matching the given galera and pod
func getBackupPersistentVolumeClaim(galera *apigalera.Galera, pod *corev1.Pod) *corev1.PersistentVolumeClaim {
	return pkggalera.CreateGaleraClaim(
		&galera.Spec,
		galera.Labels,
		getBackupPersistentVolumeClaimName(galera, getPodSuffix(pod)),
		galera.Name,
		galera.Namespace,
		getPodRevision(pod),
		galera.AsOwner(),
		)
}

// isClaimMatching tests if the PVC is matching the claim spec of the galera cluster. The default storage class name
// must be given in case the requested claim does not specify any class and use the default class
func isClaimMatching(galera *apigalera.Galera, claim *corev1.PersistentVolumeClaim, defaultSCName string) bool {
	return pkggalera.CheckClaim(galera, claim, defaultSCName)
}

// isOneContainerTerminated returns true is one container in the pod is terminated
func isOneContainerTerminated(pod *corev1.Pod) bool {
	running := false
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.State.Terminated != nil {
			running = true
		}
	}
	return running
}

// isRunningAndReady returns true if pod is in the PodRunning Phase, if no container is terminated and
// if it has a condition of PodReady.
func isRunningAndReady(pod *corev1.Pod) bool {
	return pod.Status.Phase == corev1.PodRunning && !isOneContainerTerminated(pod) && podutil.IsPodReady(pod)
}

// isCreated returns true if pod has been created and is maintained by the API server
func isCreated(pod *corev1.Pod) bool {
	return pod.Status.Phase != ""
}

// isFailed returns true if pod is in the PodRunning Phase and at least one container is not ready, or true if
// the pod is failed
func isFailed(pod *corev1.Pod) bool {
	if pod.Status.Phase == corev1.PodFailed {
		return true
	}
	if isOneContainerTerminated(pod) {
		return true
	}
	return false
}

// isTerminating returns true if pod's DeletionTimestamp has been set
func isTerminating(pod *corev1.Pod) bool {
	return pod.DeletionTimestamp != nil
}

// isHealthy returns true if pod is running and ready and has not been terminated
func isHealthy(pod *corev1.Pod) bool {
	return isRunningAndReady(pod) && !isTerminating(pod)
}

// isClaimTerminating returns true if pvc's DeletionTimestamp has been set
func isClaimTerminating(claim *corev1.PersistentVolumeClaim) bool {
	return claim.DeletionTimestamp != nil
}

// getBootstrapAddresses returns a string containing addresses used to join galera cluster. An empty string
// means a new galera cluster will be created.
func getBootstrapAddresses(pods []*corev1.Pod) string {
	m := make(map[*corev1.Pod]string)
	for _, pod  := range pods {
		m[pod] = pod.Status.PodIP
	}
	// remove pod with Special role
	if len(m) > 1 {
		for pod := range m {
			value, exist := pod.Labels[apigalera.GaleraRoleLabel]
			if exist && (value == apigalera.RoleSpecial) {
				delete(m, pod)
			}
		}
	}
	// remove pod with Writer role
	if len(m) > 1 {
		for pod := range m {
			value, exist := pod.Labels[apigalera.GaleraRoleLabel]
			if exist && (value == apigalera.RoleWriter) {
				delete(m, pod) 
			}
		}
	}
	// remove pod with BackupWriter role
	if len(m) > 1 {
		for pod := range m {
			value, exist := pod.Labels[apigalera.GaleraRoleLabel]
			if exist && (value == apigalera.RoleBackupWriter) {
				delete(m, pod)
			}
		}
	}
	var podIPs []string
	for _, podIP := range m {
		podIPs = append(podIPs, podIP)
	}
	return strings.Join(podIPs[:], ",")
}

// getPodRevision gets the revision of Pod by inspecting the GaleraRevisionLabel. If pod has no revision the empty
// string is returned.
func getPodRevision(pod *corev1.Pod) string {
	if pod.Labels == nil {
		return ""
	}
	return pod.Labels[apigalera.GaleraRevisionLabel]
}

// getClaimRevision gets the revision of the claim by inspecting the GaleraRevisionLabel. If claim has no revision
// the empty string is returned.
func getClaimRevision(claim *corev1.PersistentVolumeClaim) string {
	if claim.Labels == nil {
		return ""
	}
	return claim.Labels[apigalera.GaleraRevisionLabel]
}

// ChoseRole return a specified Role based on existing role inside the galera cluster
func choseRole(status *apigalera.GaleraStatus) string {
	if status.Members.Writer == "" {
		return apigalera.RoleWriter
	}
	if status.Members.BackupWriter == "" {
		return apigalera.RoleBackupWriter
	}
	if status.Members.Backup == "" {
		return apigalera.Backup
	}
	return apigalera.Reader
}

// newGaleraPod creates a new Pod for a Galera
func newGaleraPod(galera *apigalera.Galera, revision, podName, role, state, addresses, bootstrapImage, backupImage string,
	init bool, credsMap map[string]string) *corev1.Pod {

	return pkggalera.NewGaleraPod(
		&galera.Spec,
		galera.Labels,
		podName,
		galera.Name,
		galera.Namespace,
		role,
		addresses,
		bootstrapImage,
		backupImage,
		revision,
		state,
		init,
		credsMap,
		galera.AsOwner(),
		)
}

// newGaleraPodDisruptionBudget creates a new Pod Disruption Budget for a Galera
func newGaleraPodDisruptionBudget(galera *apigalera.Galera) *policyv1.PodDisruptionBudget {
	return pkggalera.NewGaleraPodDisruptionBudget(
		&galera.Spec,
		galera.Labels,
		galera.Name,
		galera.Namespace,
		getPDBName(galera.Name),
		1,
		galera.AsOwner(),
		)
}

// newGaleraService is used to build services based on the given role
func newGaleraService(galera *apigalera.Galera, serviceName, role string) *corev1.Service {
	return pkggalera.NewGaleraService(
		galera.ObjectMeta.Labels,
		galera.Name,
		galera.Namespace,
		serviceName,
		role,
		galera.AsOwner(),
		)
}

// newGaleraServiceMonitor returns a service used for monitoring Galera clusters
func newGaleraServiceMonitor(galera *apigalera.Galera) *corev1.Service {
	return pkggalera.NewGaleraServiceMonitor(
		galera.ObjectMeta.Labels,
		galera.Name,
		galera.Namespace,
		getServiceName(galera.Name, apigalera.ServiceMonitorSuffix, apigalera.MaxServiceMonitorLength),
		*galera.Spec.Pod.Metric.Port,
		galera.AsOwner(),
		)
}

// newGaleraHeadlessService returns an headless service for Galera
func newGaleraHeadlessService(galera *apigalera.Galera) *corev1.Service {
	return pkggalera.NewGaleraHeadlessService(
		galera.ObjectMeta.Labels,
		galera.Name,
		galera.Namespace,
		getServiceName(galera.Name, apigalera.HeadlessServiceSuffix, apigalera.MaxHeadlessServiceLength),
		galera.AsOwner(),
		)
}

// selectorForGalera returns a selector build with Galera provided labels , Galera.Name and Galera.Namespace
func selectorForGalera(galera *apigalera.Galera) (labels.Selector, error) {
	return pkggalera.SelectorForGalera(galera.Labels, galera.Name, galera.Namespace)
}

// func claimLabelsForGalera returns a map with all labels needed for a Galera, it is used to patch labels with the
// desired revsision
func claimLabelsForGalera(galera *apigalera.Galera, revision string) map[string]string {
	return pkggalera.ClaimLabelsForGalera(galera.Labels, galera.Name, galera.Namespace, revision)
}

// Match check if the given Galera's template matches the template stored in the given history.
func Match(galera *apigalera.Galera, history *appsv1.ControllerRevision) (bool, error) {
	patch, err := getPatch(galera)
	if err != nil {
		return false, err
	}
	return bytes.Equal(patch, history.Data.Raw), nil
}

// getPatch returns a strategic merge patch that can be applied to restore a Galera to a
// previous version. If the returned error is nil the patch is valid. The current state that we save is just the
// PodSpecTemplate. We can modify this later to encompass more state (or less) and remain compatible with previously
// recorded patches.
func getPatch(galera *apigalera.Galera) ([]byte, error) {
	str, err := runtime.Encode(patchCodec, galera)
	if err != nil {
		logrus.Infof("3")
		return nil, err
	}
	var raw map[string]interface{}
	err = json.Unmarshal([]byte(str), &raw)
	if err != nil {
		logrus.Infof("4")
		return nil, err
	}
	objCopy := make(map[string]interface{})
	specCopy := make(map[string]interface{})
	spec := raw["spec"].(map[string]interface{})
	pod := spec["pod"].(map[string]interface{})
	specCopy["pod"] = pod
	pod["$patch"] = "replace"
	claim := spec["persistentVolumeClaimSpec"].(map[string]interface{})
	specCopy["persistentVolumeClaimSpec"] = claim
	claim["$patch"] = "replace"
	objCopy["spec"] = specCopy
	patch, err := json.Marshal(objCopy)

	return patch, err
}

// newRevision creates a new ControllerRevision containing a patch that reapplies the target state of galera.
// The Revision of the returned ControllerRevision is set to revision. If the returned error is nil, the returned
// ControllerRevision is valid. Galera revisions are stored as patches that re-apply the current state of galera
// to a new Galera using a strategic merge patch to replace the saved state of the new Galera.
func newRevision(galera *apigalera.Galera, revision int64, collisionCount *int32) (*appsv1.ControllerRevision, error) {
	patch, err := getPatch(galera)
	if err != nil {
		return nil, err
	}
	cr, err := history.NewControllerRevision(galera,
		controllerKind,
		galera.ObjectMeta.Labels,
		runtime.RawExtension{Raw: patch},
		revision,
		collisionCount)
	if err != nil {
		return nil, err
	}
	if cr.ObjectMeta.Annotations == nil {
		cr.ObjectMeta.Annotations = make(map[string]string)
	}
	for key, value := range galera.Annotations {
		cr.ObjectMeta.Annotations[key] = value
	}
	return cr, nil
}

// ApplyRevision returns a new Galera constructed by restoring the state in revision to galera. If the returned error
// is nil, the returned Galera is valid.
func ApplyRevision(galera *apigalera.Galera, revision *appsv1.ControllerRevision) (*apigalera.Galera, error) {
	clone := galera.DeepCopy()
	patched, err := strategicpatch.StrategicMergePatch([]byte(runtime.EncodeOrDie(patchCodec, clone)), revision.Data.Raw, clone)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(patched, clone)
	if err != nil {
		return nil, err
	}
	return clone, nil
}

// newNextRevision finds the next valid revision number based on revisions. If the length of revisions
// is 0 this is 1. Otherwise, it is 1 greater than the largest revision's Revision. This method
// assumes that revisions has been sorted by Revision.
func newNextRevision(revisions []*appsv1.ControllerRevision) int64 {
	count := len(revisions)
	if count <= 0 {
		return 1
	}
	return revisions[count-1].Revision + 1
}

// completeRollingUpdate completes a rolling update when all of galera's replica Pods have been updated
// to the updateRevision. status's currentRevision is galera to updateRevision and its' updateRevision
// is galera to the empty string. status's currentReplicas is set to updateReplicas and its updateReplicas
// are set to 0.
func completeRollingUpdate(status *apigalera.GaleraStatus) {
	if 	status.NextReplicas == status.Replicas &&
		int32(len(status.Members.Ready)) == status.Replicas {
			status.CurrentReplicas = status.NextReplicas
			status.CurrentRevision = status.NextRevision
	}
}
