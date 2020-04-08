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
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/kubernetes/pkg/controller/history"
)

// GaleraControl implements the control logic for upgrading Galeras and their children Pods. It is implemented
// as an interface to allow for extensions that provide different semantics. Currently, there is only one implementation.
type GaleraControlInterface interface {
	// SyncGalera implements the control logic for Galera creation, restore, update, upgrate, and deletion
	// If an implementation returns a non-nil error, the invocation will be retried using a rate-limited strategy.
	// Implementors should sink any errors that they do not wish to trigger a retry, and they may feel free to
	// exit exceptionally at any point provided they wish the update to be re-run at a later point in time.
	SyncGalera(galera *apigalera.Galera, pods []*corev1.Pod, claims []*corev1.PersistentVolumeClaim) error
	// ListRevisions returns a array of the ControllerRevisions that represent the revisions of galera. If the returned
	// error is nil, the returns slice of ControllerRevisions is valid.
	ListRevisions(galera *apigalera.Galera) ([]*appsv1.ControllerRevision, error)
	// AdoptOrphanRevisions adopts any orphaned ControllerRevisions that match galera's Selector. If all adoptions are
	// successful the returned error is nil.
	AdoptOrphanRevisions(galera *apigalera.Galera, revisions []*appsv1.ControllerRevision) error
}

// NewDefaultGaleraControl returns a new instance of the default implementation GaleraControlInterface that
// implements the documented semantics for Galeras. podControl is the PodControlInterface used to create, update,
// and delete Pods and to create PersistentVolumeClaims. statusUpdater is the GaleraStatusUpdaterInterface used
// to update the status of Galeras. You should use an instance returned from NewRealStatefulPodControl() for any
// scenario other than testing.
func NewDefaultGaleraControl(
	podControl GaleraPodControlInterface,
	serviceControl GaleraServiceControlInterface,
	podDBControl GaleraPodDisruptionBudgetControlInterface,
	secretControl GaleraSecretControlInterface,
	storageControl GaleraStorageControlInterface,
	upgradeConfigControl GaleraUpgradeConfigControlInterface,
	restoreControl GaleraRestoreControlInterface,
	statusUpdater GaleraStatusUpdaterInterface,
	controllerHistory history.Interface,
	bootstrapImage string,
	backupImage string,
	recorder record.EventRecorder) GaleraControlInterface {
	logger := logrus.WithField("pkg", "controller")
	return &defaultGaleraControl{
		logger,
		podControl,
		serviceControl,
		podDBControl,
		secretControl,
		storageControl,
		upgradeConfigControl,
		restoreControl,
		statusUpdater,
		controllerHistory,
		bootstrapImage,
		backupImage,
		recorder}
}

type defaultGaleraControl struct {
	logger            		*logrus.Entry
	podControl        		GaleraPodControlInterface
	serviceControl    		GaleraServiceControlInterface
	podDBControl      		GaleraPodDisruptionBudgetControlInterface
	secretControl     		GaleraSecretControlInterface
	storageControl			GaleraStorageControlInterface
	upgradeConfigControl 	GaleraUpgradeConfigControlInterface
	restoreControl    		GaleraRestoreControlInterface
	statusUpdater     		GaleraStatusUpdaterInterface
	controllerHistory 		history.Interface
	bootstrapImage    		string
	backupImage       		string
	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API
	recorder          record.EventRecorder
}

// SyncGalera executes the core logic loop for a galera cluster
func (gc *defaultGaleraControl) SyncGalera(galera *apigalera.Galera, pods []*corev1.Pod, claims []*corev1.PersistentVolumeClaim) error {
	// list all revisions and sort them
	revisions, err := gc.ListRevisions(galera)
	if err != nil {
		return err
	}
	history.SortControllerRevisions(revisions)

	// get the current, and update revisions
	currentRevision, nextRevision, collisionCount, err := gc.getGaleraRevisions(galera, revisions)
	if err != nil {
		return err
	}

	// get the current and next revisions of the galera
	currentGalera, err := ApplyRevision(galera, currentRevision)
	if err != nil {
		return err
	}
	nextGalera, err := ApplyRevision(galera, nextRevision)
	if err != nil {
		return err
	}

	var status *apigalera.GaleraStatus

	// Sync loop is designed to operate the cluster and to reconcile desired state with observed state
	status, err = gc.syncGalera(currentGalera, nextGalera, currentRevision.Name, nextRevision.Name, collisionCount, pods, claims)
	if err != nil {
		return err
	}

	if status.Phase != apigalera.GaleraPhaseRestoring {
		// Create or Update all Services for Galera
		if err := gc.createOrUpdateServices(galera, status); err != nil {
			return err
		}
	}

	// Create or Update PodDisruptionBudget for Galera
	pdbName, err := gc.podDBControl.CreateOrUpdateGaleraPDB(galera)
	if err != nil {
		// If an error occurs during Get/Create, we'll requeue the item so we can
		// attempt processing again later. This could have been caused by a
		// temporary network failure, or any other transient reason.
		return err
	}
	status.PodDisruptionBudgetName = pdbName

	// Update the galera's status
	err = gc.updateGaleraStatus(galera, status)
	if err != nil {
		return err
	}

	var allConditions []string

	for _, condition := range status.Conditions {
		allConditions = append(allConditions, condition.Message)
	}

	gc.logger.Infof("Galera %s/%s revisions : current=%s with %d replicas, next=%s with %d replicas phase=%s condition=%s",
		galera.Namespace,
		galera.Name,
		status.CurrentRevision,
		status.CurrentReplicas,
		status.NextRevision,
		status.NextReplicas,
		status.Phase,
		allConditions)

	if galera.Spec.Special == nil {
		gc.logger.Infof("Galera %s/%s pod status replicas=%d ready=%s unready=%s",
			galera.Namespace,
			galera.Name,
			status.Replicas,
			status.Members.Ready,
			status.Members.Unready)
	} else {
		gc.logger.Infof("Galera %s/%s pod status replicas=%d ready=%s unready=%s special=%s",
			galera.Namespace,
			galera.Name,
			status.Replicas,
			status.Members.Ready,
			status.Members.Unready,
			status.Members.Special)
	}

	// maintain the galera's revision history limit
	return gc.truncateHistory(galera, pods, revisions, currentRevision, nextRevision)
}

func (gc *defaultGaleraControl) ListRevisions(galera *apigalera.Galera) ([]*appsv1.ControllerRevision, error) {
	selector, err := metav1.LabelSelectorAsSelector(metav1.SetAsLabelSelector(galera.ObjectMeta.Labels))
	if err != nil {
		return nil, err
	}
	return gc.controllerHistory.ListControllerRevisions(galera, selector)
}

func (gc *defaultGaleraControl) AdoptOrphanRevisions(
	galera *apigalera.Galera,
	revisions []*appsv1.ControllerRevision) error {
	for i := range revisions {
		adopted, err := gc.controllerHistory.AdoptControllerRevision(galera, controllerKind, revisions[i])
		if err != nil {
			return err
		}
		revisions[i] = adopted
	}
	return nil
}

// truncateHistory truncates any non-live ControllerRevisions in revisions from galera's history. The UpdateRevision and
// CurrentRevision in galera's Status are considered to be live. Any revisions associated with the Pods in pods are also
// considered to be live. Non-live revisions are deleted, starting with the revision with the lowest Revision, until
// only RevisionHistoryLimit revisions remain. If the returned error is nil the operation was successful. This method
// expects that revisions is sorted when supplied.
func (gc *defaultGaleraControl) truncateHistory(
	galera *apigalera.Galera,
	pods []*corev1.Pod,
	revisions []*appsv1.ControllerRevision,
	current *appsv1.ControllerRevision,
	update *appsv1.ControllerRevision) error {
	historySl := make([]*appsv1.ControllerRevision, 0, len(revisions))
	// mark all live revisions
	live := map[string]bool{current.Name: true, update.Name: true}
	for i := range pods {
		live[getPodRevision(pods[i])] = true
	}
	// collect live revisions and historic revisions
	for i := range revisions {
		if !live[revisions[i].Name] {
			historySl = append(historySl, revisions[i])
		}
	}
	historyLen := len(historySl)
	var historyLimit int

	if galera.Spec.RevisionHistoryLimit == nil {
		historyLimit = apigalera.DefaultRevisionHistoryLimit
	} else {
		historyLimit = int(*galera.Spec.RevisionHistoryLimit)
	}

	if historyLen <= historyLimit {
		return nil
	}
	// delete any non-live history to maintain the revision limit.
	historySl = historySl[:(historyLen - historyLimit)]
	for i := 0; i < len(historySl); i++ {
		if err := gc.controllerHistory.DeleteControllerRevision(historySl[i]); err != nil {
			return err
		}
	}
	return nil
}

// getGaleraRevisions returns the current and update ControllerRevisions for galera. It also
// returns a collision count that records the number of name collisions galera saw when creating
// new ControllerRevisions. This count is incremented on every name collision and is used in
// building the ControllerRevision names for name collision avoidance. This method may create
// a new revision, or modify the Revision of an existing revision if an update to galera is detected.
// This method expects that revisions is sorted when supplied.
func (gc *defaultGaleraControl) getGaleraRevisions(
	galera *apigalera.Galera,
	revisions []*appsv1.ControllerRevision) (*appsv1.ControllerRevision, *appsv1.ControllerRevision, int32, error) {
	var currentRevision, nextRevision *appsv1.ControllerRevision

	revisionCount := len(revisions)
	history.SortControllerRevisions(revisions)

	// Use a local copy of galera.Status.CollisionCount to avoid modifying galera.Status directly.
	// This copy is returned so the value gets carried over to galera.Status in updateGalera.
	var collisionCount int32
	if galera.Status.CollisionCount != nil {
		collisionCount = *galera.Status.CollisionCount
	}

	// create a new revision from the current galera
	nextRevision, err := newRevision(galera, newNextRevision(revisions), &collisionCount)
	if err != nil {
		return nil, nil, collisionCount, err
	}

	// find any equivalent revisions
	equalRevisions := history.FindEqualRevisions(revisions, nextRevision)
	equalCount := len(equalRevisions)

	if equalCount > 0 && history.EqualRevision(revisions[revisionCount-1], equalRevisions[equalCount-1]) {
		// if the equivalent revision is immediately prior the update revision has not changed
		nextRevision = revisions[revisionCount-1]
	} else if equalCount > 0 {
		// if the equivalent revision is not immediately prior we will roll back by incrementing the
		// Revision of the equivalent revision
		nextRevision, err = gc.controllerHistory.UpdateControllerRevision(
			equalRevisions[equalCount-1],
			nextRevision.Revision)
		if err != nil {
			return nil, nil, collisionCount, err
		}
	} else {
		//if there is no equivalent revision we create a new one
		nextRevision, err = gc.controllerHistory.CreateControllerRevision(galera, nextRevision, &collisionCount)
		if err != nil {
			return nil, nil, collisionCount, err
		}
	}

	// attempt to find the revision that corresponds to the current revision
	for i := range revisions {
		if revisions[i].Name == galera.Status.CurrentRevision {
			currentRevision = revisions[i]
		}
	}

	// if the current revision is nil we initialize the history by setting it to the next revision
	if currentRevision == nil {
		currentRevision = nextRevision
	}

	return currentRevision, nextRevision, collisionCount, nil
}

// syncGalera performs the synchronize function for a Galera. This method creates, updates, and deletes Pods in
// the galera in order to conform the system to the target state for the galera. The target state always contains
// galera.Spec.Replicas Pods with a Ready Condition. If the returned error is nil, the returned GaleraStatus is valid and the
// update must be recorded. If the error is not nil, the method should be retried until successful.
func (gc *defaultGaleraControl) syncGalera(
	currentGalera *apigalera.Galera,
	nextGalera *apigalera.Galera,
	currentRevision string,
	nextRevision string,
	collisionCount int32,
	pods []*corev1.Pod,
	claims []*corev1.PersistentVolumeClaim) (*apigalera.GaleraStatus, error) {

	// Set the generation, and revisions in the returned status
	status := apigalera.GaleraStatus{}
	status.ObservedGeneration = currentGalera.Generation
	status.CurrentRevision = currentRevision
	status.NextRevision = nextRevision
	status.CollisionCount = new(int32)
	*status.CollisionCount = collisionCount

	// If the Galera is being deleted, don't do anything
	if currentGalera.DeletionTimestamp != nil {
		// TODO : not working because galera is deleted before entering this loop
		clustersDeleted.Inc()
		clustersTotal.Dec()
		return &status, nil
	}

	// Get credentials, credentials are used to access galera containers and also to create readiness and liveness probes
	mapCredGalera, err := gc.secretControl.GetGaleraCreds(currentGalera)
	if err != nil {
		// If we cannot have all credentials, we will not be able to create a galera cluster
		status.Phase = apigalera.GaleraPhaseFailed
		return &status, fmt.Errorf(fmt.Sprintf("cluter %s/%s is failed, credentials are incorrect", currentGalera.Namespace, currentGalera.Name))
	}

	// Copy previous Conditions
	status.Conditions = currentGalera.Status.Conditions

	if !status.IsUpgrading() {
		// Check if we can upgrade, if not, use only the current galera
		canUpgrade, err := gc.upgradeConfigControl.CanUpgrade(currentGalera.Spec.Pod.Image, nextGalera.Spec.Pod.Image, nextGalera.Spec.Pod.MycnfConfigMap, currentGalera)
		if err != nil {
			return &status, err
		}
		if !canUpgrade {
			// If we can not upgrade, keep using the current controller revision
			nextGalera = currentGalera
		}
	}

	switch currentGalera.Status.Phase {
	case apigalera.GaleraPhaseNone:
		// If there is no Phase, it means that the cluster is creating or restoring and this is the first time
		// we have it in this sync loop
		if currentGalera.Spec.Restore == nil {
			status.Phase = apigalera.GaleraPhaseCreating
			status.SetScalingUpCondition(0, *currentGalera.Spec.Replicas)
		} else {
			status.Phase = apigalera.GaleraPhaseScheduleRestore
			status.SetRestoringCondition(currentGalera.Spec.Restore.Name)
		}
		clustersCreated.Inc()
		clustersTotal.Inc()
	case apigalera.GaleraPhaseFailed:
		status.Phase = apigalera.GaleraPhaseFailed
		return nil, fmt.Errorf(fmt.Sprintf("cluter %s/%s is failed, no more action will be executed by the operator", currentGalera.Namespace, currentGalera.Name))
	case apigalera.GaleraPhaseScheduleRestore:
		status.Phase = apigalera.GaleraPhaseScheduleRestore
		err := gc.scheduleGaleraRestore(
			currentGalera,
			&status,
			currentRevision,
			pods,
			claims,
			mapCredGalera)
		if err != nil {
			return &status, err
		}
		if int32(len(status.Members.Ready)) == *currentGalera.Spec.Replicas {
			status.Phase = apigalera.GaleraPhaseCopyRestore
			go gc.restoreControl.HandleGaleraRestore(currentGalera, pods, mapCredGalera)
		}
	case apigalera.GaleraPhaseCopyRestore:
		status.Phase = apigalera.GaleraPhaseCopyRestore

		var readyPods []*corev1.Pod
		var unreadyPods []*corev1.Pod

		// Check pods and short them between ready and unready pods
		for _, pod := range pods {
			status.Replicas++

			if isRunningAndReady(pod) {
				readyPods = append(readyPods, pod)
			} else {
				unreadyPods = append(unreadyPods, pod)
			}
			status.CurrentReplicas++
		}

		status.Members.Ready, status.Members.Unready = getMembersName(readyPods, unreadyPods)

		// Check pods and claims. If no claim are found, it means the cluster is failed, if there are claims and
		// no pod, it means restore is complete so we can create a galera cluster using existing claim
		dataClaims, _ := splitClaim(claims)

		if len(dataClaims) == 0 {
			// TODO: if the galera operator fails during the go gc.restoreControl.HandleGaleraRestore, the
			// cluster will be in this phase, perhaps we should try to restore it again
			status.Phase = apigalera.GaleraPhaseFailed
		}
		if len(pods) == 0 {
			status.Phase = apigalera.GaleraPhaseRestoring
		}
	case apigalera.GaleraPhaseCreating:
		fallthrough
	case apigalera.GaleraPhaseBackuping:
		fallthrough
	case apigalera.GaleraPhaseRestoring:
		fallthrough
	case apigalera.GaleraPhaseRunning:
		status.Phase = currentGalera.Status.Phase
		err := gc.runGalera(
			currentGalera,
			nextGalera,
			currentRevision,
			nextRevision,
			&status,
			pods,
			claims,
			mapCredGalera)
		if err != nil {
			return &status, err
		}

	default:
		status.Phase = apigalera.GaleraPhaseFailed
	}

	return &status, nil
}

// splitClaim returns two slices, the first slice contains data claim, one datacalaim is used by every pod,
// the second slice contains backup claim that are only used by restore and backup pods
func splitClaim(claims []*corev1.PersistentVolumeClaim) (dataClaims []*corev1.PersistentVolumeClaim, bkpClaims []*corev1.PersistentVolumeClaim) {
	for _, claim := range claims {
		_, isbkp := getClaimSuffixAndBackup(claim)
		if isbkp == false {
			dataClaims = append(dataClaims, claim)
		} else {
			bkpClaims = append(bkpClaims, claim)
		}
	}
	return
}

// runGalera performs the run for a Galera, ie a cluster is created. This method creates, updates and deletes Pods.
func (gc *defaultGaleraControl) runGalera(
	currentGalera *apigalera.Galera,
	nextGalera *apigalera.Galera,
	currentRevision string,
	nextRevision string,
	status *apigalera.GaleraStatus,
	pods []*corev1.Pod,
	claims []*corev1.PersistentVolumeClaim,
	mapCredGalera map[string]string) error {

	var readyPods []*corev1.Pod
	var unreadyPods []*corev1.Pod
	var specialPod *corev1.Pod
	var podSuffixes []string
	var bkpSuffixes []string
	var addPod *corev1.Pod

	// Delete pods that don't have theirs claims (data and backup)
	for _, pod := range pods {

		if isHealthy(pod) {
			deletePod := true
			podSuffix := getPodSuffix(pod)

			if backup, exist := pod.Labels[apigalera.GaleraBackupLabel]; exist && backup == "true" {
				// Don't delete backup pod if a data and a backup claims match
				findData := false
				findBackup := false

				for _, claim := range claims {
					claimSuffix, isBackup := getClaimSuffixAndBackup(claim)
					if claimSuffix == podSuffix {
						if isBackup {
							findBackup = true
						} else {
							findData = true
						}
					}
				}

				if findBackup && findData {
					deletePod = false
				}

			} else {
				// Don't delete pod, if a data claim is matching
				for _, claim := range claims {
					if podSuffix == getClaimSuffix(claim) {
						deletePod = false
					}
				}
			}

			if deletePod {
				gc.logger.Infof("deleting pod %s from galera %s/%s : no matching claims", pod.Name, currentGalera.Namespace, currentGalera.Name)
				// make a deep copy so we don't mutate the shared cache
				return gc.podControl.ForceDeletePod(currentGalera, pod.DeepCopy())
			}
		}
	}

	// Check pods and sort them between ready and unready pods
	// Ready Pods are Pods that are Running and Ready and that have also the Galera state equals to Primary, meaning
	// they are synced with other members part of the Galera Cluster.
	// Unready Pods are Pods not Ready or Running or also not synced (Primary), and are also Pods started as Standalone
	// Pods. Standalone Pods are used to do parallel restoration and also to manage image minor and major upgrades.
	// Note that Special Pod is not Ready nor Unready
	onePodTerminating := false
	podTerminating := ""

	for _, pod := range pods {
		status.Replicas++
		addPod = nil

		if isTerminating(pod) {
			onePodTerminating = true
			podTerminating = pod.Name
			continue
		}

		// Create two slices, one with running and ready pods and one with unready pods
		if isRunningAndReady(pod) {
			err, isPrimary := gc.podControl.IsPrimary(currentGalera, pod, mapCredGalera["user"], mapCredGalera["password"])
			if err != nil {
				return err
			}
			if isPrimary {
				addPod = nil

				if role, exist := pod.Labels[apigalera.GaleraRoleLabel]; exist {
					switch role {
					case apigalera.RoleWriter:
						addPod = pod
						// if we are more than one Writer, patch the pod's labels
						if status.Members.Writer == "" {
							status.Members.Writer = pod.Name
						} else {
							// make a deep copy so we don't mutate the shared cache
							if err := gc.podControl.PatchPodLabels(currentGalera, pod.DeepCopy(), RemoveRole(), AddReader()); err != nil {
								return err
							}
						}
					case apigalera.RoleBackupWriter:
						addPod = pod
						// if we are more than one BackupWriter, patch the pod's labels
						if status.Members.BackupWriter == "" {
							status.Members.BackupWriter = pod.Name
						} else {
							// make a deep copy so we don't mutate the shared cache
							if err := gc.podControl.PatchPodLabels(currentGalera, pod.DeepCopy(), RemoveRole(), AddReader()); err != nil {
								return err
							}
						}
					case apigalera.RoleSpecial:
						status.Replicas--
						// if we are more than one RoleSpecial, delete the pod
						if status.Members.Special == "" {
							specialPod = pod
							status.Members.Special = pod.Name
						} else {
							// make a deep copy so we don't mutate the shared cache
							return gc.podControl.ForceDeletePodAndClaim(currentGalera, pod.DeepCopy())
						}
					default:
						// delete this kind of pod
						// make a deep copy so we don't mutate the shared cache
						return gc.podControl.ForceDeletePodAndClaim(currentGalera, pod.DeepCopy())
					}
				}

				// check if the pod a a backup
				if backup, exist := pod.Labels[apigalera.GaleraBackupLabel]; exist && backup == "true" {
					// if we are more than one Backup pod, delete the pod
					if status.Members.Backup == "" {
						addPod = pod
						status.Members.Backup = pod.Name
						bkpSuffixes = append(bkpSuffixes, getPodSuffix(pod))
					} else {
						// make a deep copy so we don't mutate the shared cache
						return gc.podControl.ForceDeletePodAndClaim(currentGalera, pod.DeepCopy())
					}
				}

				// check if the pod is just a reader
				if addPod == nil {
					if reader, exist := pod.Labels[apigalera.GaleraReaderLabel]; exist && reader == "true" {
						addPod = pod
					}
				}

				if addPod != nil {
					readyPods = append(readyPods, addPod)
				}
			} else {
				addPod = pod
				value, exist := pod.Labels[apigalera.GaleraRoleLabel]
				if exist {
					if value == apigalera.RoleSpecial {
						addPod = nil
						specialPod = pod
					}
				}
				if addPod != nil {
					unreadyPods = append(unreadyPods, addPod)
				}

				// check if the pod a a backup
				value, exist = pod.Labels[apigalera.GaleraBackupLabel]
				if exist && value == "true" {
					bkpSuffixes = append(bkpSuffixes, getPodSuffix(pod))
				}
			}
		} else {
			// pod is not running or is not ready
			// add it to slice made of unready pods only if it not a special pod
			addPod = pod
			value, exist := pod.Labels[apigalera.GaleraRoleLabel]
			if exist {
				if value == apigalera.RoleSpecial {
					addPod = nil
					specialPod = pod
				}
			}
			if addPod != nil {
				unreadyPods = append(unreadyPods, addPod)
			}

			// check if the pod a a backup
			value, exist = pod.Labels[apigalera.GaleraBackupLabel]
			if exist && value == "true" {
				bkpSuffixes = append(bkpSuffixes, getPodSuffix(pod))
			}
		}

		// Count the number of current and next revisions
		if isCreated(pod) && !isTerminating(pod) && pod != specialPod {
			if getPodRevision(pod) == currentRevision {
				status.CurrentReplicas++
			}
			if getPodRevision(pod) == nextRevision {
				status.NextReplicas++
			}
		}

		podSuffixes = append(podSuffixes, getPodSuffix(pod))
	}

	status.Members.Ready, status.Members.Unready = getMembersName(readyPods, unreadyPods)

	// Set at least a Writer, a BackupWriter if possible and at least a Reader
	if err := gc.setLabels(currentGalera, readyPods, status); err != nil {
		return err
	}

	// If at lest one pod is terminating, do not go further
	if onePodTerminating {
		gc.logger.Infof("galera %s/%s is waiting for Pod %s to terminate", currentGalera.Namespace, currentGalera.Name, podTerminating)
		return nil
	}

	// Check if a new galera image is going to be deployed
	newImage := false
	if currentGalera.Spec.Pod.Image != nextGalera.Spec.Pod.Image {
		newImage = true
	}

	// Check if galera cluster is failed and set Phase and Conditions
	if err := setStatusPhaseAndConditions(nextGalera, status, int32(len(readyPods)), int32(len(unreadyPods)), newImage); err != nil {
		return err
	}

	defaultSCName, err := gc.storageControl.GetNameDefaultStorageClass()
	if err != nil {
		return err
	}

	// Return a suffixes' list of unused claims.
	// Do not go further, if
	// 1. a claim is terminating
	// 2. a backup claim not mapped by a pod is found : delete this claim
	// 3. a claim is not matching the galera claim spec : delete this claim
	unusedClaimSuffixes, op, err := gc.deleteNoMatchingClaimsAndListUnusedClaims(nextGalera, podSuffixes, bkpSuffixes, claims, defaultSCName)
	if err != nil || op == true {
		return err
	}

	addresses := getBootstrapAddresses(readyPods)

	// Perform mysql_upgrade, if a pod or a claim is modified, stop processing galera for this round
	op, err = gc.checkMysqlUpgrade(nextGalera, unreadyPods, mapCredGalera)
	if err != nil || op == true {
		return err
	}

	if int32(len(readyPods)) < *currentGalera.Spec.Replicas {
		// When restoring, do not proceed any update
		chosenRevision := nextRevision
		if status.Phase == apigalera.GaleraPhaseRestoring {
			chosenRevision = currentRevision
		}

		return gc.checkUnreadyPodBeforeCreatingPod(
			nextGalera,
			status,
			unreadyPods,
			chosenRevision, currentRevision, nextRevision,
			choseRole(status),
			addresses,
			mapCredGalera,
			podSuffixes,
			unusedClaimSuffixes)
	}

	// At this point, all of the current Replicas are Running and Ready, we can consider termination.
	err = gc.considerPodAndClaimTermination(
		nextGalera,
		status,
		readyPods,
		unreadyPods,
		claims,
		unusedClaimSuffixes,
		*nextGalera.Spec.Replicas,
		currentRevision, nextRevision,
		mapCredGalera)
	if err != nil {
		return err
	}

	// Managing the update by deleting a pod that does not match the update revision
	action, err := gc.deletePodForUpgrade(
		nextGalera,
		status,
		readyPods,
		currentRevision,
		nextRevision,
		mapCredGalera,
		defaultSCName)
	if err != nil {
		return err
	}
	if action {
		return nil
	}

	// Manage the special node
	if nextGalera.Spec.Special != nil {
		err = gc.manageSpecialNode(
			nextGalera,
			status,
			currentRevision,
			nextRevision,
			specialPod,
			addresses,
			podSuffixes,
			unusedClaimSuffixes,
			mapCredGalera,
			defaultSCName)
		if err != nil {
			return err
		}
	} else {
		if specialPod != nil {
			// We need to delete this pod because it is no longer needed
			// delete the pod if it not already terminating
			if !isTerminating(specialPod) {
				gc.logger.Infof("Deleting specialPod %s part of Galera %s/%s", specialPod.Name, nextGalera.Namespace, nextGalera.Name)
				return gc.podControl.ForceDeletePodAndClaim(nextGalera, specialPod)
			}
		}
	}

	return nil
}

// scheduleGaleraRestore is the first part of restoring
func (gc *defaultGaleraControl) scheduleGaleraRestore(
	galera *apigalera.Galera,
	status *apigalera.GaleraStatus,
	revision string,
	pods []*corev1.Pod,
	claims []*corev1.PersistentVolumeClaim,
	mapCredGalera map[string]string) error {
	var readyPods []*corev1.Pod
	var unreadyPods []*corev1.Pod
	var podSuffix []string

	// Check pods and short them between ready and unready pods
	for _, pod := range pods {
		status.Replicas++

		value, exist := pod.Labels[apigalera.GaleraRoleLabel]
		if exist && value == apigalera.Restore && getPodRevision(pod) == revision {
			// Create two slices, one with running and ready pods and one with unready pods
			if isRunningAndReady(pod) {
				readyPods = append(readyPods, pod)
			} else {
				unreadyPods = append(unreadyPods, pod)
			}
			status.CurrentReplicas++
		} else {
			// Delete pod
			return gc.podControl.ForceDeletePodAndClaim(galera, pod)
		}

		podSuffix = append(podSuffix, getPodSuffix(pod))
	}

	status.Members.Ready, status.Members.Unready = getMembersName(readyPods, unreadyPods)

	defaultSCName, err := gc.storageControl.GetNameDefaultStorageClass()
	if err != nil {
		return err
	}

	// Delete backup claim not mapped by a pod and return a suffixes' list of unused claims
	unusedClaimSuffixes, op, err := gc.deleteNoMatchingClaimsAndListUnusedClaims(galera, podSuffix, []string{}, claims, defaultSCName)
	if err != nil || op == true {
		return err
	}

	if int32(len(readyPods)) < *galera.Spec.Replicas {
		return gc.checkUnreadyPodBeforeCreatingPod(
			galera,
			status,
			unreadyPods,
			revision, revision, revision,
			apigalera.Restore,
			"",
			mapCredGalera,
			podSuffix,
			unusedClaimSuffixes)
	}

	// At this point, all of the current Replicas are Running and Ready, we can consider termination.
	return gc.considerPodAndClaimTermination(
		galera,
		status,
		readyPods,
		unreadyPods,
		claims,
		unusedClaimSuffixes,
		*galera.Spec.Replicas,
		revision, revision,
		mapCredGalera)
}

// getMembersName returns two slices containning names of ready and unready pods
func getMembersName(readyPods, unreadyPods []*corev1.Pod) (ready, unready []string) {
	for _, pod := range readyPods {
		ready = append(ready, pod.Name)
	}

	for _, pod := range unreadyPods {
		unready = append(unready, pod.Name)
	}

	return
}

// setLabels is used to patch labels carried by pods in order to match exposed services
func (gc *defaultGaleraControl) setLabels(galera *apigalera.Galera,readyPods []*corev1.Pod, status *apigalera.GaleraStatus) error {
	switch len(readyPods) {
	case 0:
		return nil
	case 1:
		return gc.podControl.PatchPodLabels(galera, readyPods[0].DeepCopy(), AddReader(), AddWriter())
	case 2:
		// Set at least a Writer
		if status.Members.Writer == "" {
			var backup *corev1.Pod

			for _, pod := range readyPods {
				if _, exist := pod.Labels[apigalera.GaleraBackupLabel]; exist {
					backup = pod
				}
			}

			// Chose the Writer that is different from the Backup (if Backup exists)
			if backup == nil {
				status.Members.Writer = readyPods[0].Name
				if err := gc.podControl.PatchPodLabels(galera, readyPods[0].DeepCopy(), RemoveReader(), AddWriter()); err != nil {
					return err
				}
				status.Members.BackupWriter = readyPods[1].Name
				return gc.podControl.PatchPodLabels(galera, readyPods[1].DeepCopy(), AddReader(), AddBackupWriter())
			} else {
				writer := readyPods[0]
				if backup == readyPods[0] {
					writer = readyPods[1]
				}
				status.Members.Writer = writer.Name
				if err := gc.podControl.PatchPodLabels(galera, writer.DeepCopy(), RemoveReader(), AddWriter()); err != nil {
					return err
				}
				status.Members.BackupWriter = backup.Name
				return gc.podControl.PatchPodLabels(galera, backup.DeepCopy(), AddReader(), AddBackupWriter())
			}
		} else {
			if status.Members.BackupWriter == "" {
				for _, pod := range readyPods {
					if pod.Name != status.Members.Writer {
						status.Members.BackupWriter = pod.Name
						if err := gc.podControl.PatchPodLabels(galera, pod.DeepCopy(), AddReader(), AddBackupWriter()); err != nil {
							return err
						}
					}
				}
			}

			for _, pod := range readyPods {
				if pod.Name == status.Members.Writer {
					if pod.Labels[apigalera.GaleraReaderLabel] == "true" {
						return gc.podControl.PatchPodLabels(galera, pod.DeepCopy(), RemoveReader())
					}
				}
			}
		}
	default:
		if status.Members.Writer == "" {
			var candidate *corev1.Pod
			for _, pod := range readyPods {
				if _, exist := pod.Labels[apigalera.GaleraBackupLabel]; exist {
					continue
				}
				if _, exist := pod.Labels[apigalera.GaleraRoleLabel]; exist {
					continue
				}
				candidate = pod
				break
			}

			if candidate == nil {
				return errors.New(fmt.Sprintf("no Writer Role candidate for galera %s/%s ", galera.Namespace, galera.Name))
			}

			status.Members.Writer = candidate.Name
			if err := gc.podControl.PatchPodLabels(galera, candidate.DeepCopy(), RemoveReader(), AddWriter()); err != nil {
				return err
			}
		}

		if status.Members.BackupWriter == "" {
			var candidate *corev1.Pod
			for _, pod := range readyPods {
				if _, exist := pod.Labels[apigalera.GaleraBackupLabel]; exist {
					continue
				}
				if _, exist := pod.Labels[apigalera.GaleraRoleLabel]; exist {
					continue
				}
				// needed because readyPods is not update by previous patching for the Writer
				if status.Members.Writer != pod.Name {
					candidate = pod
					break
				}
			}

			if candidate == nil {
				return errors.New(fmt.Sprintf("no BackupWriter Role candidate for galera %s/%s ", galera.Namespace, galera.Name))
			}

			status.Members.Writer = candidate.Name
			if err := gc.podControl.PatchPodLabels(galera, candidate.DeepCopy(), AddReader(), AddBackupWriter()); err != nil {
				return err
			}
		}

		for _, pod := range readyPods {
			if pod.Name == status.Members.Writer {
				if pod.DeepCopy().Labels[apigalera.GaleraReaderLabel] == "true" {
					return gc.podControl.PatchPodLabels(galera, pod.DeepCopy(), RemoveReader())
				}
			}
		}
	}
	return nil
}

// setStatusPhaseAndConditions sets the Phase and Conditions of examined Galera Cluster
func setStatusPhaseAndConditions(
	galera *apigalera.Galera,
	status *apigalera.GaleraStatus,
	nbReadyPods, nbUnreadyPods int32,
	newImage bool) error {

	// if there is not at least one galera node in PRIMARY state, it means galera cluster is failed
	if nbReadyPods == 0 && nbUnreadyPods > 1 {
		status.Phase = apigalera.GaleraPhaseFailed
		status.SetFailedCondition()
		status.ClearCondition(apigalera.GaleraConditionReady)
		status.ClearCondition(apigalera.GaleraConditionScaling)
		status.ClearCondition(apigalera.GaleraConditionUpgrading)
		clustersFailed.Inc()
		clustersTotal.Dec()
		return fmt.Errorf(fmt.Sprintf("cluter %s/%s is failed, all nodes (%d) are not PRIMARY", galera.Namespace, galera.Name, nbUnreadyPods))
	}

	if status.Replicas != *galera.Spec.Replicas {
		if status.Replicas < *galera.Spec.Replicas {
			status.SetScalingUpCondition(status.Replicas, *galera.Spec.Replicas)
		} else {
			status.SetScalingDownCondition(status.Replicas, *galera.Spec.Replicas)
		}
	} else {
		status.ClearCondition(apigalera.GaleraConditionScaling)
	}
	
	switch status.Phase {
	case apigalera.GaleraPhaseCreating:
		fallthrough
	case apigalera.GaleraPhaseRestoring:
		if status.Replicas == *galera.Spec.Replicas {
			status.ClearCondition(apigalera.GaleraConditionRestoring)
			status.SetReadyCondition()
			status.Phase = apigalera.GaleraPhaseRunning
		}
	case apigalera.GaleraPhaseBackuping:
		fallthrough
	case apigalera.GaleraPhaseRunning:
		if newImage == true {
			status.SetUpgradingCondition(galera.Spec.Pod.Image)

			if !galera.Status.IsUpgrading() {
				clustersModified.Inc()
			}
		} else {
			status.ClearCondition(apigalera.GaleraConditionUpgrading)
		}
	}

	return nil
}

// deleteNoMatchingClaimsAndListUnusedClaims return a list of unused data claims.
// deleteNoMatchingClaimsAndListUnusedClaims delete all backup claims not mapping
// the backup or restore pod suffixes.
// deleteNoMatchingClaimsAndListUnusedClaims delete unused data claims not matching the galera claim spec.
func (gc *defaultGaleraControl) deleteNoMatchingClaimsAndListUnusedClaims(
	galera *apigalera.Galera,
	podSuffix []string,
	backupSuffix []string,
	claims []*corev1.PersistentVolumeClaim,
	defaultSCName string) (claimSuffix []string, operation bool, err error) {

	operation = false

	for _, claim := range claims {
		// if claim is terminating, don't process it, stop this round
		if isClaimTerminating(claim) {
			operation = true
			return
		}

		var exist bool
		// get the suffix
		suffix, backup := getClaimSuffixAndBackup(claim)
		if backup == true {
			// if it is a backup not used by the backup or restore pods, delete the claim
			exist = false
			for _, s := range  backupSuffix {
				if s == suffix {
					exist = true
				}
			}
			if exist == false {
				err = gc.podControl.DeleteClaim(galera, claim)
				operation = true
				return
			}
		} else {
			// check if the data claim is used by a pod
			exist = false
			for _, s := range podSuffix {
				if s == suffix {
					exist = true
				}
			}
			if exist == false {
				// delete claim if not matching the galera claim spec
				if isClaimMatching(galera, claim, defaultSCName) {
					claimSuffix = append(claimSuffix, suffix)
				} else {
					err = gc.podControl.DeleteClaim(galera, claim)
					operation = true
					return
				}
			}
		}
	}
	return
}


// checkMysqlUpgrade performs the mysql_upgrade for nodes being upgraded. Pods and Claims are labelled
// with the revision, when an upgrade occurs (upgrading from n to m), pod n is deleted, pod m is started and claims
// with the same name is attached. As the claim is labelled with version n, we run mysql_upgrade, after this, the
// claim is patched with label m. If an operation on pod or claim is done, return true else return false.
func (gc *defaultGaleraControl) checkMysqlUpgrade(
	galera *apigalera.Galera,
	unreadyPods []*corev1.Pod,
	mapCredGalera map[string]string) (bool, error) {
	for	_, pod := range unreadyPods {
		// Standalone pod are stored in unreadyPods slice
		if isRunningAndReady(pod) {
			podRev := getPodRevision(pod)

			_, claimRev, err := gc.podControl.GetClaimRevision(galera.Namespace, pod.Name)
			if err != nil {
				return true, err
			}

			if podRev != claimRev {
				pod = pod.DeepCopy()

				if err := gc.podControl.RunUpgradeGalera(galera, pod, mapCredGalera["user"], mapCredGalera["password"]); err != nil {
					return true, err
				}

				// Need to delete pod first, if operator crashes, rerun mysql upgrade but do not let a pod in standalone state
				if err := gc.podControl.ForceDeletePod(galera, pod); err != nil {
					return true, err
				}

				if err := gc.podControl.PatchClaimLabels(galera, pod, podRev); err != nil {
					return true, err
				}

				return true, nil
			}
		}
	}

	return false, nil
}

// checkUnreadyPodBeforeCreatingPod creates Pod if there is no unready pod
func (gc *defaultGaleraControl) checkUnreadyPodBeforeCreatingPod(
	galera *apigalera.Galera,
	status *apigalera.GaleraStatus,
	unreadyPods []*corev1.Pod,
	chosenRevision, currentRevision, nextRevision, role, addresses string,
	mapCredGalera map[string]string,
	podSuffixes, unusedClaimSuffixes []string) error {
	state := apigalera.StateCluster
	init := false
	if status.Phase != apigalera.GaleraPhaseRestoring && addresses == "" {
		init = true
	}

	// Examine each unready replica
	for i := range unreadyPods {
		// Delete failed pods
		if isFailed(unreadyPods[i]) {
			gc.recorder.Eventf(galera, corev1.EventTypeWarning, "RecreatingFailedPod",
				"Galera %s/%s is recreating failed Pod %s",
				galera.Namespace,
				galera.Name,
				unreadyPods[i].Name)

			err := gc.podControl.ForceDeletePod(galera, unreadyPods[i])

			if err == nil {
				if getPodRevision(unreadyPods[i]) == currentRevision {
					status.CurrentReplicas--
				}
				if getPodRevision(unreadyPods[i]) == nextRevision {
					status.NextReplicas--
				}
			}

			// pod deleted, no more work possible for this round
			return err
		}
		// If we find a Pod that is currently terminating, we must wait until graceful deletion
		// completes before we continue to make progress.
		if isTerminating(unreadyPods[i]) {
			gc.logger.Infof(
				"Galera %s/%s is waiting for Pod %s to Terminate",
				galera.Namespace,
				galera.Name,
				unreadyPods[i].Name)
			return nil
		}

		// TODO: check if the Pod is pending for too much time (5 minutes ?) and delete it

		// If we have a Pod that has been created but is not running and ready we can not make progress.
		if !isRunningAndReady(unreadyPods[i]) {
			gc.logger.Infof(
				"Galera %s/%s is waiting for Pod %s to be Running with all containers Ready",
				galera.Namespace,
				galera.Name,
				unreadyPods[i].Name)

			return nil
		}
	}

	// Create a new pod and return
	podName := createUniquePodName(galera.Name, podSuffixes, unusedClaimSuffixes)

	exist, rev, err := gc.podControl.GetClaimRevision(galera.Namespace, podName)
	if err != nil {
		return err
	}
	if role == apigalera.Restore {
		state = apigalera.StateStandalone
	} else {
		if exist && rev != chosenRevision {
			state = apigalera.StateStandalone
		}
	}

	pod := newGaleraPod(
		galera,
		chosenRevision,
		podName,
		role,
		state,
		addresses,
		gc.bootstrapImage,
		gc.backupImage,
		init,
		mapCredGalera,
	)

	err = gc.podControl.CreatePodAndClaim(galera, pod, role)
	if err == nil {
		status.Replicas++
		if chosenRevision == currentRevision {
			status.CurrentReplicas++
		} else {
			status.NextReplicas++
		}
	}

	return err
}

// manageSpecialNode is a method in charge of Special node. This method creates and upgrades this kind of node.
func (gc *defaultGaleraControl) manageSpecialNode(
	galera *apigalera.Galera,
	status *apigalera.GaleraStatus,
	currentRevision string,
	nextRevision string,
	specialPod *corev1.Pod,
	addresses string,
	podSuffix []string,
	claimSuffix []string,
	mapCredGalera map[string]string,
	defaultSCName string) error {

	if specialPod == nil {
		podName := createUniquePodName(galera.Name, podSuffix, claimSuffix)

		state := apigalera.StateCluster
		exist, claimRev, err := gc.podControl.GetClaimRevision(galera.Namespace, podName)
		if err != nil {
			return err
		}

		// Creating SpecialPod with nextRevision
		if exist && claimRev != nextRevision {
			state = apigalera.StateStandalone
		}

		pod := newGaleraPod(
			galera,
			nextRevision,
			podName,
			apigalera.RoleSpecial,
			state,
			addresses,
			gc.bootstrapImage,
			gc.backupImage,
			false,
			mapCredGalera,
		)

		return gc.podControl.CreatePodAndClaim(galera, pod, apigalera.RoleSpecial)
	} else {
		// Delete failed Special Pod
		if isFailed(specialPod) {
			gc.recorder.Eventf(galera, corev1.EventTypeWarning, "RecreatingFailedPod",
				"Galera %s/%s is recreating failed Special Pod %s",
				galera.Namespace,
				galera.Name,
				specialPod.Name)
			return gc.podControl.ForceDeletePod(galera, specialPod)
		}

		// if Special Pod is termination, do not do anything
		if isTerminating(specialPod) {
			gc.logger.Infof(
				"Galera %s/%s is waiting for Special Pod %s to Terminate",
				galera.Namespace,
				galera.Name,
				specialPod.Name)
			return nil
		}

		// TODO: check if the Special Pod is pending for too much time (5 minutes ?) and delete it

		// If we have a Special Pod that has been created but is not running and ready we can not make progress.
		if !isRunningAndReady(specialPod) {
			gc.logger.Infof(
				"Galera %s/%s is waiting for Special Pod %s to be Running with all containers Ready",
				galera.Namespace,
				galera.Name,
				specialPod.Name)

			return nil
		}

		// Check if Special Pod need to be upgraded
		podRev := getPodRevision(specialPod)

		_, claimRev, err := gc.podControl.GetClaimRevision(galera.Namespace, specialPod.Name)
		if err != nil {
			return err
		}

		if podRev != claimRev {
			specialPod = specialPod.DeepCopy()
			if err := gc.podControl.RunUpgradeGalera(galera, specialPod, mapCredGalera["user"], mapCredGalera["password"]); err != nil {
				return err
			}
			// Need to delete pod first, if operator crashes, rerun mysql upgrade but do not let a pod in standalone state
			if err := gc.podControl.ForceDeletePod(galera, specialPod); err != nil {
				return err
			}

			return gc.podControl.PatchClaimLabels(galera, specialPod, podRev)
		}

		// Check if upgrade is needed
		if getPodRevision(specialPod) != nextRevision {
			gc.logger.Infof("Galera %s/%s terminating Special Pod %s for upgrade",
				galera.Namespace,
				galera.Name,
				specialPod.Name)
			return gc.podControl.DeleteSyncedPodForUpgrade(galera, specialPod, mapCredGalera["user"], mapCredGalera["password"], defaultSCName)
		}

		return nil
	}
}

// considerPodAndClaimTermination deletes pods if there are too much pods deployed (if the galera cluster is scaling down),
// considerPodAndClaimTermination also deletes orphan claims.
func (gc *defaultGaleraControl) considerPodAndClaimTermination(
	galera *apigalera.Galera,
	status *apigalera.GaleraStatus,
	readyPods []*corev1.Pod,
	unreadyPods []*corev1.Pod,
	claims []*corev1.PersistentVolumeClaim,
	unusedClaimSuffixes []string,
	replicas int32,
	currentRevision, nextRevision string,
	mapCredGalera map[string]string) error {

	// If we have a Special Pod not ready, do not delete orphan claims or unready pod (special pod can be part of unready)
	canProcess := true
	if galera.Spec.Special != nil && status.Members.Special == "" {
		canProcess = false
	}
	if canProcess {
		// Delete orphan claims
		for _, claim := range claims {
			suffix := getClaimSuffix(claim)
			clear := false
			for _, unusedClaimSuffix := range unusedClaimSuffixes {
				if suffix == unusedClaimSuffix {
					clear = true
				}
			}
			if clear == true {
				return gc.podControl.DeleteClaim(galera, claim)
			}
		}

		// Delete unready pods, claim will be deleted next iteration (orphan claim)
		for _, pod := range unreadyPods {
			if err := gc.podControl.ForceDeletePod(galera, pod); err != nil {
				return err
			}
		}
	}

	// Delete ready pod, claim will be deleted next iteration (orphan claim)
	if int32(len(readyPods)) > replicas {
		// Delete a reader pod
		for _, pod := range readyPods {
			_, exist := pod.Labels[apigalera.GaleraRoleLabel]
			if exist {
				continue
			}
			_, exist = pod.Labels[apigalera.GaleraBackupLabel]
			if exist {
				continue
			}
			if err := gc.podControl.DeleteSyncedPod(galera, pod, mapCredGalera["user"], mapCredGalera["password"]); err != nil {
				return err
			}
			if getPodRevision(pod) == currentRevision {
				status.CurrentReplicas--
			}
			if getPodRevision(pod) == nextRevision {
				status.NextReplicas--
			}
			return nil
		}
	}

	return nil
}

// deletePodForUpgrade deletes a pod if the all nodes of the cluster are running and if an upgrade is needed.
// deletePodForUpgrade returns a boolean indicating if an action occurred.
func (gc *defaultGaleraControl) deletePodForUpgrade(
	galera *apigalera.Galera,
	status *apigalera.GaleraStatus,
	pods []*corev1.Pod,
	currentRevision string,
	nextRevision string,
	mapCredGalera map[string]string,
	defaultSCName string) (bool, error) {

	if currentRevision == nextRevision {
		return false, nil
	}

	for	_, pod := range pods {
		// wait for unhealthy Pods on upgrade
		if !isHealthy(pod) {
			gc.logger.Infof("Galera %s/%s is waiting for Pod %s to upgrade",
				galera.Namespace,
				galera.Name,
				pod.Name)
			return true, nil
		}

		// delete the pod if it not already terminating and does not match the next revision
		if getPodRevision(pod) != nextRevision && !isTerminating(pod) {
			gc.logger.Infof("Galera %s/%s terminating Pod %s for upgrade",
				galera.Namespace,
				galera.Name,
				pod.Name)
			err := gc.podControl.DeleteSyncedPodForUpgrade(galera, pod, mapCredGalera["user"], mapCredGalera["password"], defaultSCName)
			status.CurrentReplicas--
			return true, err
		}
	}

	return false, nil
}


// createOrUpdateServices creates or updates Services for Galera
func (gc *defaultGaleraControl) createOrUpdateServices(galera *apigalera.Galera, status *apigalera.GaleraStatus) error {
	switch galera.Status.Phase {
	case apigalera.GaleraPhaseBackuping:
		fallthrough
	case apigalera.GaleraPhaseRunning:
		if galera.Status.IsReady() {
			svcWriterName, err := gc.serviceControl.CreateOrUpdateGaleraServiceWriter(galera)
			if err != nil {
				// If an error occurs during Get/Create, we'll requeue the item so we can
				// attempt processing again later. This could have been caused by a
				// temporary network failure, or any other transient reason.
				return err
			}
			status.ServiceWriter = svcWriterName

			svcWriterBkpName, err := gc.serviceControl.CreateOrUpdateGaleraServiceWriterBackup(galera)
			if err != nil {
				return err
			}
			status.ServiceWriterBackup = svcWriterBkpName

			svcReaderName, err := gc.serviceControl.CreateOrUpdateGaleraServiceReader(galera)
			if err != nil {
				return err
			}
			status.ServiceReader = svcReaderName


			svcSpecialName, err := gc.serviceControl.CreateOrUpdateGaleraServiceSpecial(galera)
			if err != nil {
				return err
			}
			status.ServiceSpecial = svcSpecialName

			if galera.Spec.Pod.Metric != nil {
				svcMonitorName, err := gc.serviceControl.CreateOrUpdateGaleraServiceMonitor(galera)
				if err != nil {
					return err
				}
				status.ServiceMonitor = svcMonitorName

				// TODO: we should consider creating the ServiceMonitor from API monitoring.coreos.com/v1 instead of having a separate yaml
			}

			svcInternal, err := gc.serviceControl.CreateOrUpdateGaleraServiceInternal(galera)
			if err != nil {
				return err
			}
			status.HeadlessService = svcInternal
		}
		return nil
	default:
		// TODO: do we need to delete services if cluster phase is failed ?
		return nil
	}
}

// updateGaleraStatus updates galera's Status to be equal to status. If status indicates a complete update, it is
// mutated to indicate completion. If status is semantically equivalent to galera's Status no update is performed. If the
// returned error is nil, the update is successful.
func (gc *defaultGaleraControl) updateGaleraStatus(
	galera *apigalera.Galera,
	status *apigalera.GaleraStatus) error {

	// complete any in progress rolling update if necessary
	completeRollingUpdate(galera, status)

	// if the status is not inconsistent do not perform an update
	if !inconsistentStatus(galera, status) {
		return nil
	}

	// copy galera and update its status
	galera = galera.DeepCopy()
	if err := gc.statusUpdater.UpdateGaleraStatus(galera, status); err != nil {
		return err
	}

	return nil
}

var _ GaleraControlInterface = &defaultGaleraControl{}
