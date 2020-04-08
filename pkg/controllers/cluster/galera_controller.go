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
	apigalera "galera-operator/pkg/apis/apigalera/v1beta2"
	galeraclientset "galera-operator/pkg/client/clientset/versioned"
	galerascheme "galera-operator/pkg/client/clientset/versioned/scheme"
	informers "galera-operator/pkg/client/informers/externalversions/apigalera/v1beta2"
	listers "galera-operator/pkg/client/listers/apigalera/v1beta2"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	appsinformers "k8s.io/client-go/informers/apps/v1"
	coreinformers "k8s.io/client-go/informers/core/v1"
	policyinformers "k8s.io/client-go/informers/policy/v1beta1"
	storageinformers "k8s.io/client-go/informers/storage/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/kubernetes/pkg/controller"
	"k8s.io/kubernetes/pkg/controller/history"
	"reflect"
	"time"
)

const controllerAgentName = "galera-operator"

// controllerKind contains the schema.GroupVersionKind for this controller type.
var controllerKind = apigalera.SchemeGroupVersion.WithKind("Galera")

// GaleraController is the controller implementation for Galera Cluster resources
type GaleraController struct {
	// specify if the controller is cluster wide or not
	clusterWide bool
	// logrus with fields
	logger *logrus.Entry
	// galeraClient is a clientset for our own API group
	galeraClient galeraclientset.Interface
	// Abstracted out for testing
	control GaleraControlInterface
	// podControl is used for patching pods
	podControl controller.PodControlInterface
	// claimControl is used for patching persistent volume claims
	claimControl ClaimControlInterface
	// podLister is able to list/get pods from a shared informer's store
	podLister corelisters.PodLister
	// podListerSynced returns true if the pod shared informer has synced at least once
	podListerSynced cache.InformerSynced
	// galeraLister is able to list/get galera sets from a shared informer's store
	galeraLister listers.GaleraLister
	// galeraListerSynced returns true if the galera set shared informer has synced at least once
	galeraListerSynced cache.InformerSynced
	// serviceListerSynced returns true if the service shared informer has synced at least once
	serviceListerSynced cache.InformerSynced
	// pdbListerSynced returns true if the podDisruptionBudget shared informer has synced at least once
	pdbListerSynced cache.InformerSynced
	// secretListerSynced returns true if the secret shared informer has synced at least once
	secretListerSynced cache.InformerSynced
	// scListerSynced returns true if the storage class shared informer has synced at least once
	scListerSynced cache.InformerSynced
	// upgradeConfigListerSynced returns true if the upgradeConfig shared informer has synced at least once
	upgradeConfigListerSynced cache.InformerSynced
	// upgradeRuleListerSyncec returns true if the upgradeRule shared informer has synced at least once
	upgradeRuleListerSyncec cache.InformerSynced
	// cmListerSynced returns true if configMap shared informer has synced at least once
	cmListerSynced cache.InformerSynced
	//claimLister is able to list/get persistent volume claims from a shared informer's store
	claimLister corelisters.PersistentVolumeClaimLister
	// claimListerSynced returns true if the claim shared informer has synced at least once
	claimListerSynced cache.InformerSynced
	// revListerSynced returns true if the rev shared informer has synced at least once
	revListerSynced cache.InformerSynced
	// workqueue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers
	workqueue workqueue.RateLimitingInterface
}

// NewGaleraController creates a GaleraController that can be used to monitor
// Galera resources and create clusters in a target Kubernetes cluster.
// It accepts a list of informers that are then used to monitor the state of the
// target cluster
func NewGaleraController(
	kubeClient clientset.Interface,
	config *rest.Config,
	galeraClient galeraclientset.Interface,
	galeraInformer informers.GaleraInformer,
	podInformer coreinformers.PodInformer,
	serviceInformer coreinformers.ServiceInformer,
	claimInformer coreinformers.PersistentVolumeClaimInformer,
	scInformer storageinformers.StorageClassInformer,
	revInformer appsinformers.ControllerRevisionInformer,
	pdbInformer policyinformers.PodDisruptionBudgetInformer,
	secretInformer coreinformers.SecretInformer,
	upgradeConfigInformer informers.UpgradeConfigInformer,
	upgradeRuleInformer informers.UpgradeRuleInformer,
	cmInformer coreinformers.ConfigMapInformer,
	bootstrapImage string,
	backupImage string,
	upgradeConfig string,
	clusterWide bool,
	namespace string,
	resyncPeriod int,
) *GaleraController {
	logger := logrus.WithField("pkg", "controller")
	// Create event broadcaster
	// Add galera-operator types to the default Kubernetes Scheme so Events can be
	// logged for galera-operator types
	if err := galerascheme.AddToScheme(scheme.Scheme); err != nil {
		panic("impossible to add Galera scheme")
	}

	logger.Infof("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(logger.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeClient.CoreV1().Events(namespace)})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	gc := &GaleraController{
		clusterWide: clusterWide,
		logger: logger,
		galeraClient: galeraClient,
		control: NewDefaultGaleraControl(
			NewRealGaleraPodControl(
				kubeClient,
				config,
				podInformer.Lister(),
				claimInformer.Lister(),
				recorder),
			NewRealGaleraServiceControl(kubeClient, serviceInformer.Lister(), recorder),
			NewRealGaleraPodDisruptionBudgetControl(kubeClient, pdbInformer.Lister(), recorder),
			NewRealGaleraSecretControl(secretInformer.Lister()),
			NewRealGaleraStorageControl(scInformer.Lister()),
			NewRealGaleraUpgradeConfigControl(
				upgradeConfigInformer.Lister(),
				upgradeRuleInformer.Lister(),
				cmInformer.Lister(),
				upgradeConfig,
				namespace,
				recorder),
			NewRealGaleraRestoreControl(
				kubeClient,
				config,
				podInformer.Lister(),
				claimInformer.Lister(),
				secretInformer.Lister(),
				recorder),
			NewRealGaleraStatusUpdater(galeraClient, galeraInformer.Lister()),
			history.NewHistory(kubeClient, revInformer.Lister()),
			bootstrapImage,
			backupImage,
			recorder,
		),
		podControl:   controller.RealPodControl{KubeClient: kubeClient, Recorder: recorder},
		claimControl: RealClaimControl{KubeClient: kubeClient},
		revListerSynced: revInformer.Informer().HasSynced,
		serviceListerSynced: serviceInformer.Informer().HasSynced,
		pdbListerSynced: pdbInformer.Informer().HasSynced,
		secretListerSynced: secretInformer.Informer().HasSynced,
		scListerSynced: scInformer.Informer().HasSynced,
		upgradeConfigListerSynced: upgradeConfigInformer.Informer().HasSynced,
		upgradeRuleListerSyncec: upgradeRuleInformer.Informer().HasSynced,
		cmListerSynced: cmInformer.Informer().HasSynced,
		workqueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Galera"),
	}

	// Set up an event handler for when Pod resources change. This
	// handler will lookup the owner of the given Pod, and if it is
	// owned by a Galera resource will enqueue that Galera resource for
	// processing. This way, we don't need to implement custom logic for
	// handling Pod resources. More info on this pattern:
	// https://github.com/kubernetes/community/blob/8cafef897a22026d42f5e5bb3f104febe7e29830/contributors/devel/controllers.md
	podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		// lookup the Galera and enqueue
		AddFunc: gc.addPod,
		// lookup current and old Galera if labels changed
		UpdateFunc: gc.updatePod,
		// lookup Galera accounting for deletion tombstones
		DeleteFunc: gc.deletePod,
	})
	gc.podLister = podInformer.Lister()
	gc.podListerSynced = podInformer.Informer().HasSynced

	// Set up an event handler for when claim resources change
	claimInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: gc.addClaim,
		UpdateFunc: gc.updateClaim,
		DeleteFunc: gc.deleteClaim,
	})

	gc.claimLister = claimInformer.Lister()
	gc.claimListerSynced = claimInformer.Informer().HasSynced

	// Set up an event handler for when Galera resources change
	galeraInformer.Informer().AddEventHandlerWithResyncPeriod(
		cache.ResourceEventHandlerFuncs{
			AddFunc: gc.enqueueGalera,
			UpdateFunc: func(oldObj, newObj interface{}) {
				oldGalera := oldObj.(*apigalera.Galera)
				newGalera := newObj.(*apigalera.Galera)
				if oldGalera.Status.Replicas != newGalera.Status.Replicas {
					gc.logger.Infof("Observed updated replica count for Galera: %v/%v, %d->%d", newGalera.Namespace, newGalera.Name, oldGalera.Status.Replicas, newGalera.Status.Replicas)
				}
				gc.enqueueGalera(newObj)
			},
			DeleteFunc: gc.enqueueGalera,
		},
		time.Duration(resyncPeriod)*time.Second,
	)

	gc.galeraLister = galeraInformer.Lister()
	gc.galeraListerSynced = galeraInformer.Informer().HasSynced

	return gc
}

// Run is the main event loop, it will set up the event handlers for types we are interested
// in, as well as syncing informer caches and starting workers. It will block until channel
// is closed, at which point it will shutdown the workqueue and wait for workers to finish
// processing their current work items
func (gc *GaleraController) Run(threadiness int, stopCh <-chan struct{}){
	// don't let panics crash the process
	defer utilruntime.HandleCrash()

	// make sure the work queue is shutdown which will trigger workers to end
	defer gc.workqueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	gc.logger.Infof("Starting Galera controller")
	defer gc.logger.Infof("Shutting down Galera controller")

	// Wait for the caches to be synced before starting workers
	gc.logger.Infof("Waiting for informer caches to sync")
	if !controller.WaitForCacheSync(
		"Galera",
		stopCh,
		gc.podListerSynced,
		gc.galeraListerSynced,
		gc.claimListerSynced,
		gc.revListerSynced,
		gc.serviceListerSynced,
		gc.pdbListerSynced,
		gc.secretListerSynced,
		gc.scListerSynced,
		gc.upgradeConfigListerSynced,
		gc.upgradeRuleListerSyncec,
		gc.cmListerSynced,
	) {
		return
	}

	gc.logger.Infof("Starting Galera controller workers")

	// start up your worker threads based on threadiness.  Some controllers
	// have multiple kinds of workers
	for i := 0; i < threadiness; i++ {
		// runWorker will loop until "something bad" happens.  The .Until will
		// then rekick the worker after one second
		go wait.Until(gc.runWorker, time.Second, stopCh)
	}

	gc.logger.Infof("Started workers")
	defer gc.logger.Infof("Shutting down workers")
	<-stopCh
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue
func (gc *GaleraController) runWorker() {
	for gc.processNextWorkItem() {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler
func (gc *GaleraController) processNextWorkItem() bool {
	// pull the next work item from queue. It should be a key we use to lookup
	// something in a cache
	obj, quit := gc.workqueue.Get()

	if quit {
		return false
	}

	// We wrap this block in a func so we can defer c.workqueue.Done.
	err := func(obj interface{}) error {
		// We call Done here so the workqueue knows we have finished
		// processing this item. We also must remember to call Forget if we
		// do not want this work item being re-queued. For example, we do
		// not call Forget if a transient error occurs, instead the item is
		// put back on the workqueue and attempted again after a back-off
		// period
		defer gc.workqueue.Done(obj)

		var key string
		var ok bool
		// We expect strings to come off the workqueue. These are of the
		// form namespace/name. We do this as the delayed nature of the
		// workqueue means the items in the informer cache may actually be
		// more up to date that when the item was initially put onto the
		// workqueue
		if key, ok = obj.(string); !ok {
			// As the item in the workqueue is actually invalid, we call
			// Forget here else we'd go into a loop of attempting to
			// process a work item that is invalid
			gc.workqueue.Forget(obj)
			utilruntime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))

			return nil
		}

		// Run the syncHandler, passing it the namespace/name string of the
		// Galera resource to be synced.
		if err := gc.syncHandler(key); err != nil {
			return fmt.Errorf("error syncing '%s': %s", key, err.Error())
		}
		// Finally, if no error occurs, tell the queue to stop tracking history for our
		// obj. This will reset things like failure counts for per-item rate limiting
		gc.workqueue.Forget(obj)

		gc.logger.Infof("Successfully synced (%s), remove obj (%#v) from queue", key, obj)

		return nil
	}(obj)

	if err != nil {
		// there was a failure so be sure to report it.  This method allows for
		// pluggable error handling which can be used for things like
		// cluster-monitoring
		utilruntime.HandleError(fmt.Errorf("%v failed with : %v", obj, err))

		// since we failed, we should requeue the item to work on later.  This
		// method will add a backoff to avoid hotlooping on particular items
		// (they're probably still not going to work right away) and overall
		// controller protection (everything I've done is broken, this controller
		// needs to calm down or it can starve other useful work) cases
		gc.workqueue.AddRateLimited(obj)

		return true
	}

	return true
}

// addPod adds the Galera for the pod to the sync queue
func (gc *GaleraController) addPod(obj interface{}) {
	pod := obj.(*corev1.Pod)

	if pod.DeletionTimestamp != nil {
		// on a restart of the controller manager, it's possible a new pod shows up in a state that
		// is already pending deletion. Prevent the pod from being a creation observation
		gc.deletePod(pod)
		return
	}

	// If it has a ControllerRef, that's all that matters.
	if controllerRef := metav1.GetControllerOf(pod); controllerRef != nil {
		galera := gc.resolveControllerRef(pod.Namespace, controllerRef)
		if galera == nil {
			return
		}
		gc.logger.Infof("Pod %s created, labels: %+v", pod.Name, pod.Labels)
		gc.enqueueGalera(galera)
		return
	}

	// Otherwise, it's an orphan. Get a list of all matching controllers and sync
	// them to see if anyone wants to adopt it
	galeras := gc.getGalerasForPod(pod)
	if len(galeras) == 0 {
		return
	}
	gc.logger.Infof("Orphan Pod %s created, labels: %+v", pod.Name, pod.Labels)
	for _, galera := range galeras {
		gc.enqueueGalera(galera)
	}
}

// updatePod adds the Galera for the current and old pods to the sync queue.
func (gc *GaleraController) updatePod(old, cur interface{}) {
	oldPod := old.(*corev1.Pod)
	curPod := cur.(*corev1.Pod)

	if curPod.ResourceVersion == oldPod.ResourceVersion {
		// Periodic resync will send update events for all known pods
		// Two different versions of the same pod will always have different RVs
		return
	}

	labelChanged := !reflect.DeepEqual(curPod.Labels, oldPod.Labels)

	curControllerRef := metav1.GetControllerOf(curPod)
	oldControllerRef := metav1.GetControllerOf(oldPod)
	controllerRefChanged := !reflect.DeepEqual(curControllerRef, oldControllerRef)
	if controllerRefChanged && oldControllerRef != nil {
		// The ControllerRef was changed. Sync the old controller, if any
		if galera := gc.resolveControllerRef(oldPod.Namespace, oldControllerRef); galera != nil {
			gc.enqueueGalera(galera)
		}
	}

	// If it has a ControllerRef, that's all that matters
	if curControllerRef != nil {
		galera := gc.resolveControllerRef(curPod.Namespace, curControllerRef)
		if galera == nil {
			return
		}
		gc.logger.Infof("Pod %s updated, objectMeta %+v -> %+v.", curPod.Name, oldPod.ObjectMeta, curPod.ObjectMeta)
		gc.enqueueGalera(galera)
		return
	}

	// Otherwise, it's an orphan. If anything changed, sync matching controllers
	// to see if anyone wants to adopt it now
	if labelChanged || controllerRefChanged {
		galeras := gc.getGalerasForPod(curPod)
		if len(galeras) == 0 {
			return
		}
		gc.logger.Infof("Orphan Pod %s updated, objectMeta %+v -> %+v.", curPod.Name, oldPod.ObjectMeta, curPod.ObjectMeta)
		for _, galera := range galeras {
			gc.enqueueGalera(galera)
		}
	}
}

// deletePod enqueues the Galera for the pod accounting for deletion tombstones
func (gc *GaleraController) deletePod(obj interface{}) {
	pod, ok := obj.(*corev1.Pod)

	// When a delete is dropped, the relist will notice a pod in the store not
	// in the list, leading to the insertion of a tombstone object which contains
	// the deleted key/value. Note that this value might be stale. If the pod
	// changed labels the new Galera will not be woken up till the periodic resync
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("couldn't get object from tombstone %+v", obj))
			return
		}
		pod, ok = tombstone.Obj.(*corev1.Pod)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("tombstone contained object that is not a pod %+v", obj))
			return
		}
	}

	controllerRef := metav1.GetControllerOf(pod)
	if controllerRef == nil {
		// No controller should care about orphans being deleted
		return
	}
	galera := gc.resolveControllerRef(pod.Namespace, controllerRef)
	if galera == nil {
		return
	}
	gc.logger.Infof("Pod %s/%s deleted through %v.", pod.Namespace, pod.Name, utilruntime.GetCaller())
	gc.enqueueGalera(galera)
}

// addClaim adds the Galera for the claim to the sync queue
func (gc *GaleraController) addClaim(obj interface{}) {
	claim := obj.(*corev1.PersistentVolumeClaim)

	if claim.DeletionTimestamp != nil {
		// on a restart of the controller manager, it's possible a new claim shows up in a state that
		// is already pending deletion. Prevent the claim from being a creation observation
		gc.deleteClaim(claim)
		return
	}

	// If it has a ControllerRef, that's all that matters.
	if controllerRef := metav1.GetControllerOf(claim); controllerRef != nil {
		galera := gc.resolveControllerRef(claim.Namespace, controllerRef)
		if galera == nil {
			return
		}
		gc.logger.Infof("claim %s created, labels: %+v", claim.Name, claim.Labels)
		gc.enqueueGalera(galera)
		return
	}

	// Otherwise, it's an orphan. Get a list of all matching controllers and sync
	// them to see if anyone wants to adopt it
	galeras := gc.getGalerasForClaim(claim)
	if len(galeras) == 0 {
		return
	}
	gc.logger.Infof("Orphan claim %s created, labels: %+v", claim.Name, claim.Labels)
	for _, galera := range galeras {
		gc.enqueueGalera(galera)
	}
}

// updateClaim adds the galera for the current and old claims to the sync queue.
func (gc *GaleraController) updateClaim(old, cur interface{}) {
	oldClaim := old.(*corev1.PersistentVolumeClaim)
	curClaim := cur.(*corev1.PersistentVolumeClaim)

	if curClaim.ResourceVersion == oldClaim.ResourceVersion {
		// Periodic resync will send update events for all known claims
		// Two different versions of the same claim will always have different RVs
		return
	}

	labelChanged := !reflect.DeepEqual(curClaim.Labels, oldClaim.Labels)

	curControllerRef := metav1.GetControllerOf(curClaim)
	oldControllerRef := metav1.GetControllerOf(oldClaim)
	controllerRefChanged := !reflect.DeepEqual(curControllerRef, oldControllerRef)
	if controllerRefChanged && oldControllerRef != nil {
		// The ControllerRef was changed. Sync the old controller, if any
		if galera := gc.resolveControllerRef(oldClaim.Namespace, oldControllerRef); galera != nil {
			gc.enqueueGalera(galera)
		}
	}

	// If it has a ControllerRef, that's all that matters
	if curControllerRef != nil {
		galera := gc.resolveControllerRef(curClaim.Namespace, curControllerRef)
		if galera == nil {
			return
		}
		gc.logger.Infof("claim %s updated, objectMeta %+v -> %+v.", curClaim.Name, oldClaim.ObjectMeta, curClaim.ObjectMeta)
		gc.enqueueGalera(galera)
		return
	}

	// Otherwise, it's an orphan. If anything changed, sync matching controllers
	// to see if anyone wants to adopt it now
	if labelChanged || controllerRefChanged {
		galeras := gc.getGalerasForClaim(curClaim)
		if len(galeras) == 0 {
			return
		}
		gc.logger.Infof("Orphan claim %s updated, objectMeta %+v -> %+v.", curClaim.Name, oldClaim.ObjectMeta, curClaim.ObjectMeta)
		for _, galera := range galeras {
			gc.enqueueGalera(galera)
		}
	}
}

// deleteClaim enqueues the galera for the claim accounting for deletion tombstones
func (gc *GaleraController) deleteClaim(obj interface{}) {
	claim, ok := obj.(*corev1.PersistentVolumeClaim)

	// When a delete is dropped, the relist will notice a claim in the store not
	// in the list, leading to the insertion of a tombstone object which contains
	// the deleted key/value. Note that this value might be stale. If the claim
	// changed labels the new galera will not be woken up till the periodic resync
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("couldn't get object from tombstone %+v", obj))
			return
		}
		claim, ok = tombstone.Obj.(*corev1.PersistentVolumeClaim)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("tombstone contained object that is not a claim %+v", obj))
			return
		}
	}

	controllerRef := metav1.GetControllerOf(claim)
	if controllerRef == nil {
		// No controller should care about orphans being deleted
		return
	}
	galera := gc.resolveControllerRef(claim.Namespace, controllerRef)
	if galera == nil {
		return
	}
	gc.logger.Infof("claim %s/%s deleted through %v.", claim.Namespace, claim.Name, utilruntime.GetCaller())
	gc.enqueueGalera(galera)
}

// getClaimsForGalera returns the claims that a given galera should manage
//
// NOTE: Returned claims are pointers to objects from the cache
//       If you need to modify one, you need to copy it first
func (gc *GaleraController) getClaimsForGalera(galera *apigalera.Galera, selector labels.Selector) ([]*corev1.PersistentVolumeClaim, error) {
	// List all claims to include the claims that don't match the selector anymore but
	// has a ControllerRef pointing to this galera
	claims, err := gc.claimLister.PersistentVolumeClaims(galera.Namespace).List(labels.Everything())
	if err != nil {
		return nil, err
	}

	filter := func(claim *corev1.PersistentVolumeClaim) bool {
		// Only claim if it matches our StatefulSet name. Otherwise release/ignore
		return isClaimMemberOf(galera, claim)
	}

	// If any adoptions are attempted, we should first recheck for deletion with
	// an uncached quorum read sometime after listing claims
	canAdoptFunc := controller.RecheckDeletionTimestamp(func() (metav1.Object, error) {
		fresh, err := gc.galeraClient.SqlV1beta2().Galeras(galera.Namespace).Get(galera.Name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		if fresh.UID != galera.UID {
			return nil, fmt.Errorf("original Galera %v/%v is gone: got uid %v, wanted %v", galera.Namespace, galera.Name, fresh.UID, galera.UID)
		}
		return fresh, nil
	})

	claimcm := NewClaimControllerRefManager(gc.claimControl, galera, selector, controllerKind, canAdoptFunc)
	return claimcm.ClaimClaims(claims, filter)
}


// getPodsForGalera returns the Pods that a given Galera should manage
// It also reconciles ControllerRef by adopting/orphaning.
//
// NOTE: Returned Pods are pointers to objects from the cache
//       If you need to modify one, you need to copy it first
func (gc *GaleraController) getPodsForGalera(galera *apigalera.Galera, selector labels.Selector) ([]*corev1.Pod, error) {
	// List all pods to include the pods that don't match the selector anymore but
	// has a ControllerRef pointing to this Galera
	pods, err := gc.podLister.Pods(galera.Namespace).List(labels.Everything())
	if err != nil {
		return nil, err
	}

	filter := func(pod *corev1.Pod) bool {
		// Only claim if it matches our StatefulSet name. Otherwise release/ignore
		return isPodMemberOf(galera, pod)
	}

	// If any adoptions are attempted, we should first recheck for deletion with
	// an uncached quorum read sometime after listing Pods (see #42639)
	canAdoptFunc := controller.RecheckDeletionTimestamp(func() (metav1.Object, error) {
		fresh, err := gc.galeraClient.SqlV1beta2().Galeras(galera.Namespace).Get(galera.Name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		if fresh.UID != galera.UID {
			return nil, fmt.Errorf("original Galera %v/%v is gone: got uid %v, wanted %v", galera.Namespace, galera.Name, fresh.UID, galera.UID)
		}
		return fresh, nil
	})

	pcm := controller.NewPodControllerRefManager(gc.podControl, galera, selector, controllerKind, canAdoptFunc)
	return pcm.ClaimPods(pods, filter)
}

// adoptOrphanRevisions adopts any orphaned ControllerRevisions matched by Galera's Selector
func (gc *GaleraController) adoptOrphanRevisions(galera *apigalera.Galera) error {
	revisions, err := gc.control.ListRevisions(galera)
	if err != nil {
		return err
	}
	hasOrphans := false
	for i := range revisions {
		if metav1.GetControllerOf(revisions[i]) == nil {
			hasOrphans = true
			break
		}
	}
	if hasOrphans {
		fresh, err := gc.galeraClient.SqlV1beta2().Galeras(galera.Namespace).Get(galera.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if fresh.UID != galera.UID {
			return fmt.Errorf("original Galera %v/%v is gone: got uid %v, wanted %v", galera.Namespace, galera.Name, fresh.UID, galera.UID)
		}
		return gc.control.AdoptOrphanRevisions(galera, revisions)
	}
	return nil
}

// getGalerasForPod returns a list of Galeras that potentially match a given pod
func (gc *GaleraController) getGalerasForPod(pod *corev1.Pod) []*apigalera.Galera {
	galeras, err := gc.galeraLister.GetPodGaleras(pod)
	if err != nil {
		return nil
	}
	// More than one set is selecting the same Pod
	if len(galeras) > 1 {
		// ControllerRef will ensure we don't do anything crazy, but more than one
		// item in this list nevertheless constitutes user error.
		utilruntime.HandleError(
			fmt.Errorf(
				"user error: more than one Galera is selecting pods with labels: %+v",
				pod.Labels))
	}
	return galeras
}

// getGalerasForClaim returns a list of Galeras that potentially match a given claim
func (gc *GaleraController) getGalerasForClaim(claim *corev1.PersistentVolumeClaim) []*apigalera.Galera {
	galeras, err := gc.galeraLister.GetClaimGaleras(claim)
	if err != nil {
		return nil
	}
	// More than one set is selecting the same claim
	if len(galeras) > 1 {
		// ControllerRef will ensure we don't do anything crazy, but more than one
		// item in this list nevertheless constitutes user error.
		utilruntime.HandleError(
			fmt.Errorf(
				"user error: more than one galera is selecting claim with labels: %+v",
				claim.Labels))
	}
	return galeras
}


// resolveControllerRef returns the controller referenced by a ControllerRef,
// or nil if the ControllerRef could not be resolved to a matching controller
// of the correct Kind.
func (gc *GaleraController) resolveControllerRef(namespace string, controllerRef *metav1.OwnerReference) *apigalera.Galera {
	// We can't look up by UID, so look up by Name and then verify UID.
	// Don't even try to look up by Name if it's the wrong Kind.
	if controllerRef.Kind != controllerKind.Kind {
		return nil
	}
	galera, err := gc.galeraLister.Galeras(namespace).Get(controllerRef.Name)
	if err != nil {
		return nil
	}
	if galera.UID != controllerRef.UID {
		// The controller we found with this Name is not the same one that the
		// ControllerRef points to.
		return nil
	}
	return galera
}

// enqueueGalera takes a Galera resource and converts it into a
// namespace/name string which is then put onto the work queue. This method
// should *not* be passed resources of any type other than Galera.
func (gc *GaleraController) enqueueGalera(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)

	if err != nil {
		utilruntime.HandleError(fmt.Errorf("could not get key for object %+v:%v", obj, err))
		return
	}
//	gc.workqueue.AddRateLimited(key)
	gc.workqueue.Add(key)
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Galera
// resource with the current status of the resource.
func (gc *GaleraController) syncHandler(key string) error {
	startTime := time.Now()
	defer func() {
		gc.logger.Infof("Finished syncing Galera %q (%v)", key, time.Since(startTime))
	}()

	// Convert the namespace/name string into a distinct namespace and name.
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the Galera resource with this namespace/name
	galera, err := gc.galeraLister.Galeras(namespace).Get(name)
	if err != nil {
		// The Galera resource may no longer exist, in which case we stop processing
		if errors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("galera '%s' in work queue no longer exists", key))
			return nil
		}
		utilruntime.HandleError(fmt.Errorf("unable to retrieve Galera %v from store: %v", key, err))
		return err
	}

	// Galera are added to the queue in 3 conditions
	// - a galera cluster is created
	// - a galera cluster is modified
	// - the operator is resurrected
	// idempotent actions can be safely run over already existing Galera
	// non idempotent can't

	// Check if the Galera resource is managed by this controller
	if !gc.managed(galera) {
		gc.logger.Infof("Galera %s/%s not managed by this controller", galera.Namespace, galera.Name)
		return nil
	}

	// Galera cluster failed : do nothing, ie do not delete failed pods, if CR is deleted, failed pods will
	// be deleted by garbage collector. It let a possibility to owner of the Galera CR do do manual actions
	// on pods.
	// Return nil to remove from workqueue and to not process again this Galera
	if galera.Status.IsFailed() {
		utilruntime.HandleError(fmt.Errorf("ignore failed Galera %s/%s. Please delete its CR", galera.Namespace, galera.Name))
		return nil
	}

	if err := gc.adoptOrphanRevisions(galera); err != nil {
		return err
	}

	selector, err := selectorForGalera(galera)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("error converting Galera %s/%s selector: %v", galera.Namespace, galera.Name, err))
		// This is a non-transient error, so don't retry.
		return nil
	}

	// list pods part of this galera cluster
	pods, err := gc.getPodsForGalera(galera, selector)
	if err != nil {
		gc.logger.Infof("error listing pods for Galera %s/%s: %v", galera.Namespace, galera.Name, err)
		return err
	}

	// list claims part of this galera cluster
	claims, err := gc.getClaimsForGalera(galera, selector)
	if err != nil {
		gc.logger.Infof("error listing claims for Galera %s/%s: %v", galera.Namespace, galera.Name, err)
		return err
	}

	return gc.syncGalera(galera, pods, claims)
}

// createOrUpdateGalera syncs a tuple of (galera, []*corev1.Pod, []*corev1.PersistentVolumeClaim)
func (gc *GaleraController) syncGalera(galera *apigalera.Galera, pods []*corev1.Pod, claims []*corev1.PersistentVolumeClaim) error {
	gc.logger.Infof("Syncing Galera %v/%v with %d pods and %d claims", galera.Namespace, galera.Name, len(pods), len(claims))
	// TODO: investigate where we mutate the galera cluster during the update as it is not obvious.
	if err := gc.control.SyncGalera(galera.DeepCopy(), pods, claims); err != nil {
		gc.logger.Infof("Error when syncing Galera %s/%s", galera.Namespace, galera.Name)
		return err
	}
	return nil
}

// managed returns if this GaleraController manages the specified Galera
func (gc *GaleraController) managed(galera *apigalera.Galera) bool {
	if value, ok := galera.ObjectMeta.Annotations[apigalera.GaleraAnnotationScope]; ok {
		if gc.clusterWide {
			return value == apigalera.GaleraAnnotationClusterWide
		}
	} else {
		if !gc.clusterWide {
			return true
		}
	}
	return false
}
