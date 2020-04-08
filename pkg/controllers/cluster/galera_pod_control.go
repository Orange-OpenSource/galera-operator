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
	"encoding/json"
	"errors"
	"fmt"
	apigalera "galera-operator/pkg/apis/apigalera/v1beta2"
	"galera-operator/pkg/exec"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientset "k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"regexp"
	"strings"
)

// GaleraPodControlInterface defines the interface that GaleraController uses to create, update, and delete Pods,
// and to update the Status of a Galera. It follows the design paradigms used for PodControl, but its
// implementation provides for claim creation, Pod creation, Pod termination, and Pod identity enforcement.
// Like controller.PodControlInterface, it is implemented as an interface to provide for testing fakes.
type GaleraPodControlInterface interface {
	// CreatePodAndClaim create a pod in a galera. Any claims necessary for the pod are created prior to creating
	// the pod. If the returned error is nil the pod and its claims have been created.
	CreatePodAndClaim(galera *apigalera.Galera, pod *corev1.Pod, role string) error
	// PatchPodLabels patchs a pod, used for services mapping and upgrade.
	PatchPodLabels(galera *apigalera.Galera, pod *corev1.Pod, actions ...PatchAction) error
	// PatchClaimLabels patchs data claim, it is used in the process of upgrading a node.
	PatchClaimLabels(galera *apigalera.Galera, pod *corev1.Pod, revision string) error
	// DeleteSyncedPod deletes a pod in a galera if this galera cluster node, ie this pod, is "Synced", ie there is no data not
	// only present on this node. The pod's claims are not deleted. If the delete is successful, the returned error is nil.
	DeleteSyncedPod(galera *apigalera.Galera, pod *corev1.Pod, user, password string) error
	// DeletePodForUpgrade delete only the pod (not claim(s)) only if the galera container running in the pod is "synced",
	// it means that datas on this galera node are replicated on at least an other node.
	DeleteSyncedPodForUpgrade(galera *apigalera.Galera, pod *corev1.Pod, user, password, defaultSCName string) error
	// ForceDeletePod deletes a pod in a galera without any check. The pod's claims are not deleted. If the delete is
	// successful, the returned error is nil.
	ForceDeletePod(galera *apigalera.Galera, pod *corev1.Pod) error
	// ForceDeletePodAndClaim deletes a pod and its claims without any check. If the delete is
	// successful, the returned error is nil.
	ForceDeletePodAndClaim(galera *apigalera.Galera, pod *corev1.Pod) error
	// DeleteClaim deletes the specified claim, it can delete data or backup claim.
	DeleteClaim(galera *apigalera.Galera, claim *corev1.PersistentVolumeClaim) error
	// GetClaimRevision returns the revision of the data claim. An error is returned if the claim does not exist.
	GetClaimRevision(namespace, podName string) (bool, string, error)
	// RunUpgradeGalera runs a mysql_upgrade on previously upgraded pod
	RunUpgradeGalera(galera *apigalera.Galera, pod *corev1.Pod, user, password string) error
	// IsPrimary returns true if the galera container galera status is equal to primary, it means the node is up-to-date
	// in the galera cluster point of view
	IsPrimary(galera *apigalera.Galera, pod *corev1.Pod, user, password string) (error, bool)
}

func NewRealGaleraPodControl(
	client clientset.Interface,
	config *rest.Config,
	podLister corelisters.PodLister,
	claimLister corelisters.PersistentVolumeClaimLister,
	recorder record.EventRecorder,
) GaleraPodControlInterface {
	logger := logrus.WithField("pkg", "controller")
	return &realGaleraPodControl{logger, client, config,podLister, claimLister, recorder}
}

// realGaleraPodControl implements GaleraPodControlInterface using a clientset.Interface to communicate with the
// API server. The struct is package private as the internal details are irrelevant to importing packages.
type realGaleraPodControl struct {
	logger      *logrus.Entry
	client      clientset.Interface
	config      *rest.Config
	podLister   corelisters.PodLister
	claimLister corelisters.PersistentVolumeClaimLister
	recorder    record.EventRecorder
}

func (gpc *realGaleraPodControl) CreatePodAndClaim(galera *apigalera.Galera, pod *corev1.Pod, role string) error {
	// Create the pod's claims prior to creating the Pod
	if err := gpc.createPersistentVolumeClaim(galera, pod); err != nil {
		return err
	}

	if role == apigalera.Backup {
		if err := gpc.createBackupPersistentVolumeClaim(galera, pod); err != nil {
			return err
		}
	}

	// If we created the claims, attempt to create the pod
	_, err := gpc.client.CoreV1().Pods(galera.Namespace).Create(pod)
	// sink already exists errors
	if apierrors.IsAlreadyExists(err) {
		return err
	}
	gpc.recordPodEvent("create", galera, pod, err)

	return err
}

//  patchStringValue specifies a patch operation for a string.
type patchStringValue struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value map[string]string `json:"value"`
}

type PatchAction func(map[string]string) map[string]string

func AddReader() PatchAction {
	return func(m map[string]string) map[string]string {
		m[apigalera.GaleraReaderLabel] = "true"
		return m
	}
}

func RemoveReader() PatchAction {
	return func(m map[string]string) map[string]string {
		m[apigalera.GaleraReaderLabel] = "false"
		return m
	}
}

func AddWriter() PatchAction {
	return func(m map[string]string) map[string]string {
		m[apigalera.GaleraRoleLabel] = apigalera.RoleWriter
		return m
	}
}

func AddBackupWriter() PatchAction {
	return func(m map[string]string) map[string]string {
		m[apigalera.GaleraRoleLabel] = apigalera.RoleBackupWriter
		return m
	}
}

func RemoveRole() PatchAction {
	return func(m map[string]string) map[string]string {
		delete(m, apigalera.GaleraRoleLabel)
		return m
	}
}

func (gpc *realGaleraPodControl) PatchPodLabels(galera *apigalera.Galera, pod *corev1.Pod, actions ...PatchAction) error {
	m := pod.Labels

	for _, action := range actions {
		m = action(m)
	}

	labelsPayload := []patchStringValue{{
		Op:    "replace",
		Path:  "/metadata/labels",
		Value: m,
	}}
	labelsPayloadBytes, _ := json.Marshal(labelsPayload)

	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		// commit the update, retrying on conflicts
		_, updateErr := gpc.client.CoreV1().Pods(galera.Namespace).Patch(pod.Name, types.JSONPatchType, labelsPayloadBytes)
		if updateErr == nil {
			return nil
		}
		if updated, err := gpc.podLister.Pods(galera.Namespace).Get(pod.Name); err == nil {
			// make a copy so we don't mutate the shared cache
			pod = updated.DeepCopy()
		} else {
			utilruntime.HandleError(fmt.Errorf("error getting updated Pod %s/%s from lister: %v", galera.Namespace, pod.Name, err))
		}

		return updateErr
	})

	gpc.recordPodEvent("patch", galera, pod, err)

	return err
}

func (gpc *realGaleraPodControl) PatchClaimLabels(galera *apigalera.Galera, pod *corev1.Pod, revision string) error {
	claim, err := gpc.claimLister.PersistentVolumeClaims(galera.Namespace).Get(getPersistentVolumeClaimNameForPod(pod.Name))
	if err != nil {
		return err
	}

	labelsPayload := []patchStringValue{{
		Op:    "replace",
		Path:  "/metadata/labels",
		Value: claimLabelsForGalera(galera, revision),
	}}
	labelsPayloadBytes, _ := json.Marshal(labelsPayload)

	// Patch data claim
	err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		// commit the update, retrying on conflicts
		_, updateErr := gpc.client.CoreV1().PersistentVolumeClaims(galera.Namespace).Patch(claim.Name, types.JSONPatchType, labelsPayloadBytes)
		if updateErr == nil {
			return nil
		}
		if updated, err := gpc.claimLister.PersistentVolumeClaims(galera.Namespace).Get(claim.Name); err == nil {
			// make a copy so we don't mutate the shared cache
			claim = updated.DeepCopy()
		} else {
			utilruntime.HandleError(fmt.Errorf("error getting updated claim %s/%s from lister: %v", galera.Namespace, claim.Name, err))
		}

		return updateErr
	})

	// No need to patch backup claim

	gpc.recordClaimEvent("patch", galera, claim, err)

	return err
}

func (gpc *realGaleraPodControl) DeleteSyncedPod(galera *apigalera.Galera, pod *corev1.Pod, user, password string) error {
	funcDeletePod := func() error { return gpc.client.CoreV1().Pods(galera.Namespace).Delete(pod.Name, nil) }
	return gpc.deleteSynced(galera, pod, user, password, funcDeletePod)
}

func (gpc *realGaleraPodControl) DeleteSyncedPodForUpgrade(galera *apigalera.Galera, pod *corev1.Pod, user, password, defaultSCName string) error {
	funcDeleteForUpdatePodAndClaims := func() error {
		role, exist := pod.Labels[apigalera.GaleraBackupLabel]
		if exist {
			if role == apigalera.Backup {
				// get backup claim
				bkpClaim, err := gpc.claimLister.PersistentVolumeClaims(galera.Namespace).Get(getBackupPersistentVolumeClaimName(galera, getPodSuffix(pod)))
				if err != nil {
					return err
				}
				// check backup claim
				if !isClaimMatching(galera, bkpClaim, defaultSCName) {
					// delete backup claim
					err = gpc.DeleteClaim(galera, bkpClaim)
					gpc.recordClaimEvent("delete", galera, bkpClaim, err)
					if err != nil {
						return err
					}
				}
			}
		}

		// get data claim
		claim, err := gpc.claimLister.PersistentVolumeClaims(galera.Namespace).Get(getPersistentVolumeClaimName(galera, getPodSuffix(pod)))
		if err != nil {
			return err
		}

		// check data claim
		if !isClaimMatching(galera, claim, defaultSCName) {
			// delete data claim
			err = gpc.DeleteClaim(galera, claim)
			gpc.recordClaimEvent("delete", galera, claim, err)
			if err != nil {
				return err
			}
		}
		// delete pod
		return gpc.client.CoreV1().Pods(galera.Namespace).Delete(pod.Name, nil)
	}
	return gpc.deleteSynced(galera, pod, user, password, funcDeleteForUpdatePodAndClaims)
}

// wsrepLocalStateComment is a regular expression that extracts the wsrep_local_state_comment value form a string
var wsrepLocalStateComment = regexp.MustCompile("(?im)^(.*)wsrep_local_state_comment(.*)$")

// isSynced is a regular expression that extracts if the value for a parameter is Synced
var isSynced = regexp.MustCompile("Synced")

func (gpc *realGaleraPodControl) deleteSynced(galera *apigalera.Galera, pod *corev1.Pod, user, password string, f func() error) error {
	// run the following command to check whether the node is up to date:
	// mysqladmin -u{user} -p{password} extended-status | grep wsrep_local_state_comment
	cmd := []string{"mysqladmin", "extended-status"}
	cmd = append(cmd, fmt.Sprintf("-u%s", user))
	cmd = append(cmd, fmt.Sprintf("-p%s", password))

	stdout, stderr, err := exec.ExecCmd(gpc.client, gpc.config, galera.Namespace, pod, cmd)

	if err != nil {
		gpc.logger.Infof("error (%e) executing mysql command : %s", err, stderr)
	} else {
		result := wsrepLocalStateComment.Find([]byte(stdout))

		if isSynced.Match(result) {
			// The galera container is synced, so data are not only on this pod, we can delete it
			err = f()
		} else {
			stgerr := fmt.Sprintf("galera pod (%s/%s) not synced", pod.Namespace, pod.Name)
			gpc.logger.Infof(stgerr)
			err = errors.New(stgerr)
		}
	}

	gpc.recordPodEvent("delete", galera, pod, err)
	return err
}


func (gpc *realGaleraPodControl) ForceDeletePod(galera *apigalera.Galera, pod *corev1.Pod) error {
	err := gpc.client.CoreV1().Pods(galera.Namespace).Delete(pod.Name, nil)
	gpc.recordPodEvent("delete", galera, pod, err)
	return err
}

func (gpc *realGaleraPodControl) ForceDeletePodAndClaim(galera *apigalera.Galera, pod *corev1.Pod) error {
	role, exist := pod.Labels[apigalera.GaleraRoleLabel]
	if exist {
		if role == apigalera.Restore || role == apigalera.Backup {
			// get backup claim
			bkpClaim, err := gpc.claimLister.PersistentVolumeClaims(galera.Namespace).Get(getBackupPersistentVolumeClaimName(galera, getPodSuffix(pod)))
			if err != nil {
				return err
			}
			// delete backup claim
			err = gpc.DeleteClaim(galera, bkpClaim)
			gpc.recordClaimEvent("delete", galera, bkpClaim, err)
			if  err != nil {
				return err
			}
		}
	}
	// get data claim
	claim, err := gpc.claimLister.PersistentVolumeClaims(galera.Namespace).Get(getPersistentVolumeClaimName(galera, getPodSuffix(pod)))
	if err != nil {
		return err
	}
	// delete data claim
	err = gpc.DeleteClaim(galera, claim)
	if err != nil {
		return err
	}
	// delete pod
	return gpc.ForceDeletePod(galera, pod)
}

func (gpc *realGaleraPodControl) DeleteClaim(galera *apigalera.Galera, claim *corev1.PersistentVolumeClaim) error {
	err := gpc.client.CoreV1().PersistentVolumeClaims(galera.Namespace).Delete(claim.Name, nil)
	gpc.recordClaimEvent("delete", galera, claim, err)
	return err
}

func (gpc *realGaleraPodControl) GetClaimRevision(namespace, podName string) (bool, string, error) {
	claim, err := gpc.claimLister.PersistentVolumeClaims(namespace).Get(getPersistentVolumeClaimNameForPod(podName))
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, "", nil
		}

		return true, "", err
	}
	return true, getClaimRevision(claim), nil
}

var isKO = regexp.MustCompile("(?s)^(.*)KO$")

func (gpc *realGaleraPodControl) RunUpgradeGalera(galera *apigalera.Galera, pod *corev1.Pod, user, password string) error {
	// build the complete command
	cmd := []string{"mysql_upgrade", "-f"}
	cmd = append(cmd, fmt.Sprintf("-u%s", user))
	cmd = append(cmd, fmt.Sprintf("-p%s", password))

	stdout, stderr, err := exec.ExecCmd(gpc.client, gpc.config, galera.Namespace, pod, cmd)

	if err != nil {
		gpc.logger.Infof("error (%e) executing mysql_upgrade command on galera pod (%s/%s) : %s", err, pod.Namespace, pod.Name, stderr)
	}

	if isKO.Match([]byte(stdout)) {
		err = errors.New(fmt.Sprintf("mysql_upgrade on galera pod (%s/%s) is not successful : %s", pod.Namespace, pod.Name, stdout))
	}

	gpc.recordPodEvent("mysqlUpgrade", galera, pod, err)

	return err
}

// wsrepClusterStatus is a regular expression that extracts the wsrep_cluster_status value form a string
var wsrepClusterStatus = regexp.MustCompile("(?im)^(.*)wsrep_cluster_status(.*)$")

// isPrimary is a regular expression that extracts if the value for a parameter is Primary
var isPrimary = regexp.MustCompile("Primary")

func (gpc *realGaleraPodControl) IsPrimary(galera *apigalera.Galera, pod *corev1.Pod, user, password string) (error, bool) {
	// run the following command to check whether the node is up to date:
	// mysqladmin -u{user} -p{password} extended-status | grep wsrep_cluster_status
	cmd := []string{"mysqladmin", "extended-status"}
	cmd = append(cmd, fmt.Sprintf("-u%s", user))
	cmd = append(cmd, fmt.Sprintf("-p%s", password))

	stdout, stderr, err := exec.ExecCmd(gpc.client, gpc.config, galera.Namespace, pod, cmd)

	if err != nil {
		gpc.logger.Infof("error (%e) executing mysql command : %s", err, stderr)
		return err, false
	}

	result := wsrepClusterStatus.Find([]byte(stdout))

	if isPrimary.Match(result) {
		// The galera container status is PRIMARY
		return nil, true
	}

	return nil, false
}

// recordPodEvent records an event for verb applied to a Pod in a Galera. If err is nil the generated event will
// have a reason of v1.EventTypeNormal. If err is not nil the generated event will have a reason of v1.EventTypeWarning.
func (gpc *realGaleraPodControl) recordPodEvent(verb string, galera *apigalera.Galera, pod *corev1.Pod, err error) {
	if err == nil {
		reason := fmt.Sprintf("Successful%s", strings.Title(verb))
		message := fmt.Sprintf("%s Pod %s in Galera %s/%s successful",
			strings.ToLower(verb), pod.Name, galera.Namespace, galera.Name)
		gpc.recorder.Event(galera, corev1.EventTypeNormal, reason, message)
	} else {
		reason := fmt.Sprintf("Failed%s", strings.Title(verb))
		message := fmt.Sprintf("%s Pod %s in Galera %s/%s failed error: %s",
			strings.ToLower(verb), pod.Name, galera.Namespace, galera.Name, err)
		gpc.recorder.Event(galera, corev1.EventTypeWarning, reason, message)
	}
}

// recordClaimEvent records an event for verb applied to the PersistentVolumeClaim of a Pod in a Galera. If err is
// nil the generated event will have a reason of v1.EventTypeNormal. If err is not nil the generated event will have a
// reason of v1.EventTypeWarning.
func (gpc *realGaleraPodControl) recordClaimEvent(verb string, galera *apigalera.Galera, claim *corev1.PersistentVolumeClaim, err error) {
	if err == nil {
		reason := fmt.Sprintf("Successful%s", strings.Title(verb))
		message := fmt.Sprintf("%s claim %s for pod %s in galera %s/%s success",
			strings.ToLower(verb), claim.Name, getPodName(galera, getClaimSuffix(claim)), galera.Namespace, galera.Name)
		gpc.recorder.Event(galera, corev1.EventTypeNormal, reason, message)
	} else {
		reason := fmt.Sprintf("Failed%s", strings.Title(verb))
		message := fmt.Sprintf("%s claim %s for pod %s in galera %s/%s failed error: %s",
			strings.ToLower(verb), claim.Name, getPodName(galera, getClaimSuffix(claim)), galera.Namespace, galera.Name, err)
		gpc.recorder.Event(galera, corev1.EventTypeWarning, reason, message)
	}
}

// createPersistentVolumeClaim creates the required PersistentVolumeClaim for pod, which must be a member of
// Galera. If the claim for pod is successfully created, the returned error is nil. If creation fails, this method
// may be called again until no error is returned, indicating the PersistentVolumeClaim for pod is consistent with
// Galera's Spec.
func (gpc *realGaleraPodControl) createPersistentVolumeClaim(galera *apigalera.Galera, pod *corev1.Pod) error {
	claim := getPersistentVolumeClaim(galera, pod)
	existingClaim, err := gpc.claimLister.PersistentVolumeClaims(claim.Namespace).Get(claim.Name)
//	existingClaim, err := gpc.galeraClient.CoreV1().PersistentVolumeClaims(claim.Namespace).Get(claim.Name, metav1.GetOptions{})

	// If the resource doesn't exist, we'll create it
	if apierrors.IsNotFound(err) {
		gpc.logger.Infof("Creating a new persistent volume claim %s for cluster %s/%s", claim.Name, galera.Namespace, galera.Name)
		_, err = gpc.client.CoreV1().PersistentVolumeClaims(claim.Namespace).Create(claim)
	} else {
		// If the resource is terminating, we return an error
		if err == nil {
			if existingClaim.DeletionTimestamp != nil {
				err = errors.New(fmt.Sprintf("Persistent Volume Claim %s/%s is terminating", galera.Namespace, claim.Name))
			}
		}
	}

	gpc.recordClaimEvent("create", galera, claim, err)
	return err
}

// createBackupPersistentVolumeClaim creates the required PersistentVolumeClaim for pod, which must be a member of
// Galera. If the claim for pod is successfully created, the returned error is nil. If creation fails, this method
// may be called again until no error is returned, indicating the PersistentVolumeClaim for pod is consistent with
// Galera's Spec.
func (gpc *realGaleraPodControl) createBackupPersistentVolumeClaim(galera *apigalera.Galera, pod *corev1.Pod) error {
	claim := getBackupPersistentVolumeClaim(galera, pod)
	existingClaim, err := gpc.claimLister.PersistentVolumeClaims(claim.Namespace).Get(claim.Name)

	// If the resource doesn't exist, we'll create it
	if apierrors.IsNotFound(err) {
		gpc.logger.Infof("Creating a new persistent volume claim %s for cluster %s/%s", claim.Name, galera.Namespace, galera.Name)
		_, err = gpc.client.CoreV1().PersistentVolumeClaims(claim.Namespace).Create(claim)
	} else {
		// If the resource is terminating, we return an error
		if err == nil {
			if existingClaim.DeletionTimestamp != nil {
				err = errors.New(fmt.Sprintf("Backup Persistent Volume Claim %s/%s is terminating", galera.Namespace, claim.Name))
			}
		}
	}

	gpc.recordClaimEvent("create", galera, claim, err)
	return err
}

/*
func (gpc *realGaleraPodControl) SetLogLevel(level logrus.Level) {
	gpc.logger.Level = level
}
*/

var _ GaleraPodControlInterface = &realGaleraPodControl{}