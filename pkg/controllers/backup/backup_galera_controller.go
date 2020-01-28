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

package backup

import (
	"fmt"
	apigalera "galera-operator/pkg/apis/apigalera/v1beta2"
	pkgbackup "galera-operator/pkg/backup"
	galeraclientset "galera-operator/pkg/client/clientset/versioned"
	galerascheme "galera-operator/pkg/client/clientset/versioned/scheme"
	informers "galera-operator/pkg/client/informers/externalversions/apigalera/v1beta2"
	listers "galera-operator/pkg/client/listers/apigalera/v1beta2"
	"galera-operator/pkg/controllers/cluster"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	coreinformers "k8s.io/client-go/informers/core/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/kubernetes/pkg/controller"
	"time"
)

const controllerAgentName = "galera-backup-operator"

// GaleraBackupController is the controller implementation for backuping Galera Cluster
type GaleraBackupController struct {
	// specify if the controller is cluster wide or not
	clusterWide bool
	// logrus with fields
	logger *logrus.Entry
	// Abstracted out for testing
	control GaleraBackupControlInterface
	// galeraLister is able to list/get galera sets from a shared informer's store
	galeraLister listers.GaleraLister
	// galeraListerSynced returns true if the galera set shared informer has synced at least once
	galeraListerSynced cache.InformerSynced
	// galeraBackupLister is able to list/get galeraBackup sets from a shared informer's store
	galeraBackupLister listers.GaleraBackupLister
	// galeraBackupListerSynced returns true if the galeraBackup set shared informer has synced at least once
	galeraBackupListerSynced  cache.InformerSynced
	// podListerSynced returns true if the pod shared informer has synced at least once
	podListerSynced cache.InformerSynced
	// secretListerSynced returns true if the secret shared informer has synced at least once
	secretListerSynced cache.InformerSynced
	// workqueue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers
	workqueue workqueue.RateLimitingInterface
}

// NewGaleraBackupController constructs a new GaleraBackupController.
func NewGaleraBackupController(
	kubeClient clientset.Interface,
	config *rest.Config,
	galeraClient galeraclientset.Interface,
	galeraBackupInformer informers.GaleraBackupInformer,
	galeraInformer informers.GaleraInformer,
	podInformer coreinformers.PodInformer,
	secretInformer coreinformers.SecretInformer,
	clusterWide bool,
	namespace string,
	resyncPeriod int,
) *GaleraBackupController {
	logger := logrus.WithField("pkg", "backupcontroller")
	// Create event broadcaster
	// Add galera-backup-operator types to the default Kubernetes Scheme so Events can be
	// logged for galera-backup-operator types
	if err := galerascheme.AddToScheme(scheme.Scheme); err != nil {
		panic("impossible to add GaleraBackup scheme")
	}
	logger.Infof("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(logrus.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeClient.CoreV1().Events(namespace)})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	gbc := &GaleraBackupController{
		clusterWide: clusterWide,
		logger: logger,
//		kubeClient:	kubeClient,
//		config: config,
//		galeraClient: galeraClient,
		control: NewDefaultGaleraBackup(
			kubeClient,
			config,
			galeraClient,
			galeraBackupInformer.Lister(),
			podInformer.Lister(),
			secretInformer.Lister(),
			NewRealGaleraBackupStatusUpdater(galeraClient, galeraBackupInformer.Lister()),
			cluster.NewRealGaleraStatusUpdater(galeraClient, galeraInformer.Lister()),
			recorder,
			),
		workqueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "GaleraBackup"),
	}

	// Set up an event handler for when Galera Backup resources change
	galeraBackupInformer.Informer().AddEventHandlerWithResyncPeriod(
		cache.ResourceEventHandlerFuncs{
			AddFunc:  gbc.enqueueGaleraBackup,
			UpdateFunc: func(oldObj, newObj interface{}) {
				gbc.enqueueGaleraBackup(newObj)
			},
			DeleteFunc: gbc.enqueueGaleraBackup,
		},
		time.Duration(resyncPeriod)*time.Second,
	)

	gbc.galeraLister = galeraInformer.Lister()
	gbc.galeraListerSynced = galeraInformer.Informer().HasSynced

	gbc.galeraBackupLister = galeraBackupInformer.Lister()
	gbc.galeraBackupListerSynced = galeraBackupInformer.Informer().HasSynced

	//	gbc.podLister = podInformer.Lister()
	gbc.podListerSynced = podInformer.Informer().HasSynced

	gbc.secretListerSynced = secretInformer.Informer().HasSynced

	return gbc
}

// Run is the main event loop, it will set up the event handlers for types we are interested
// in, as well as syncing informer caches and starting workers. It will block until channel
// is closed, at which point it will shutdown the workqueue and wait for workers to finish
// processing their current work items
func (gbc *GaleraBackupController) Run(threadiness int, stopCh <-chan struct{}) {
	// don't let panics crash the process
	defer utilruntime.HandleCrash()

	// make sure the work queue is shutdown which will trigger workers to end
	defer gbc.workqueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	gbc.logger.Infof("Starting Backup-Galera controller")
	defer gbc.logger.Infof("Shutting down Backup-Galera controller")

	// Wait for the caches to be synced before starting workers
	gbc.logger.Infof("Waiting for informer caches to sync")
	if !controller.WaitForCacheSync(
		"GaleraBackup",
		stopCh,
		gbc.galeraListerSynced,
		gbc.galeraBackupListerSynced,
		gbc.podListerSynced,
		gbc.secretListerSynced,
	) {
		return
	}

	// start up your worker threads based on threadiness.  Some controllers
	// have multiple kinds of workers
	for i := 0; i < threadiness; i++ {
		// runWorker will loop until "something bad" happens.  The .Until will
		// then rekick the worker after one second
		go wait.Until(gbc.runWorker, time.Second, stopCh)
	}

	gbc.logger.Infof("Started workers")
	defer gbc.logger.Infof("Shutting down workers")
	<-stopCh
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue
func (gbc *GaleraBackupController) runWorker() {
	for gbc.processNextWorkItem() {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler
func (gbc *GaleraBackupController) processNextWorkItem() bool {
	// pull the next work item from queue. It should be a key we use to lookup
	// something in a cache
	obj, quit := gbc.workqueue.Get()

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
		defer gbc.workqueue.Done(obj)

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
			gbc.workqueue.Forget(obj)
			utilruntime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))

			return nil
		}

		// Run the syncHandler, passing it the namespace/name string of the
		// GaleraBackup resource to be synced.
		if err := gbc.syncHandler(key); err != nil {
			return fmt.Errorf("error syncing '%s': %s", key, err.Error())
		}
		// Finally, if no error occurs, tell the queue to stop tracking history for our
		// obj. This will reset things like failure counts for per-item rate limiting
		gbc.workqueue.Forget(obj)

		gbc.logger.Infof("Successfully synced (%s)", key)

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
		gbc.workqueue.AddRateLimited(obj)

		return true
	}

	return true
}

// enqueueGalera takes a GaleraBackup resource and converts it into a
// namespace/name string which is then put onto the work queue. This method
// should *not* be passed resources of any type other than GaleraBackup.
func (gbc *GaleraBackupController) enqueueGaleraBackup(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("could not get key for object %+v:%v", obj, err))
		return
	}
//	gc.workqueue.AddRateLimited(key)
	gbc.workqueue.Add(key)
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the GaleraBackup
// resource with the current status of the resource.
func (gbc *GaleraBackupController) syncHandler(key string) error {
	// Convert the namespace/name string into a distinct namespace and name.
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the GaleraBackup resource with this namespace/name
	backup, err := gbc.galeraBackupLister.GaleraBackups(namespace).Get(name)
	if err != nil {
		// The GaleraBackup resource may no longer exist, in which case we stop processing
		if errors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("galeraBackup '%s' in work queue no longer exists", key))
			return nil
		}
		utilruntime.HandleError(fmt.Errorf("unable to retrieve GaleraBackup %v from store: %v", key, err))
		return err
	}

	// Check backup status
	_, cond := pkgbackup.GetBackupCondition(&backup.Status, apigalera.BackupFailed)
	if cond != nil && cond.Status == corev1.ConditionTrue {
		gbc.logger.Infof("Backup %s/%s failed, skipping.", backup.Namespace, backup.Name)
		return nil
	}

	_, cond = pkgbackup.GetBackupCondition(&backup.Status, apigalera.BackupComplete)
	if cond != nil && cond.Status == corev1.ConditionTrue {
		gbc.logger.Infof("Backup %s/%s is complete, skipping.", backup.Namespace, backup.Name)
		return nil
	}

	_, cond = pkgbackup.GetBackupCondition(&backup.Status, apigalera.BackupRunning)
	if cond != nil && cond.Status == corev1.ConditionTrue {
		gbc.logger.Infof("Backup %s/%s is running on galera cluster", backup.Namespace, backup.Name)
		return nil
	}

	_, cond = pkgbackup.GetBackupCondition(&backup.Status, apigalera.BackupScheduled)
	if cond != nil && cond.Status == corev1.ConditionTrue {
		gbc.logger.Infof("Backup %s/%s is already scheduled on galera cluster", backup.Namespace, backup.Name)
		return nil
	}

	galeraNamespace := backup.Spec.GaleraNamespace
	if galeraNamespace == "" {
		galeraNamespace = backup.Namespace
		gbc.logger.Infof("namespace not provided, using backup.Namespace : %s as namespace for backuping the galera %s", namespace, backup.Spec.GaleraName)
	}

	galera, err := gbc.galeraLister.Galeras(galeraNamespace).Get(backup.Spec.GaleraName)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("unable to retrieve Galera %s from GaleraBackup %s/%s : backup failed", backup.Spec.GaleraName, backup.Namespace, backup.Name))
		return nil
	}

	// Check if the Galera resource is managed by this controller
	if !gbc.managed(galera) {
		gbc.logger.Infof("GaleraBackup %s/%s cannot backup a Galera cluster not managed by this controller", backup.Namespace, backup.Name)
		return nil
	}

	// Check if galera cluster is running
	if !galera.Status.IsRunning()  {
		gbc.logger.Infof("GaleraBackup %s/%s cannot backup Galera %s/%s : galera cluster is not running", backup.Namespace, backup.Name, galera.Namespace, galera.Name)
	}

	return gbc.control.HandleBackup(backup, galera)
}

// managed returns if this GaleraBackupController manages the specified GaleraBackup
func (gbc *GaleraBackupController) managed(galera *apigalera.Galera) bool {
	if value, ok := galera.ObjectMeta.Annotations[apigalera.GaleraAnnotationScope]; ok {
		if gbc.clusterWide {
			return value == apigalera.GaleraAnnotationClusterWide
		}
	} else {
		if !gbc.clusterWide {
			return true
		}
	}
	return false
}