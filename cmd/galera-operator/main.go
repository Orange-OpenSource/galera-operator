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

package main

import (
	"context"
	"flag"
	"fmt"
	clientset "galera-operator/pkg/client/clientset/versioned"
	informers "galera-operator/pkg/client/informers/externalversions"
	"galera-operator/pkg/controllers/backup"
	"galera-operator/pkg/controllers/cluster"
	"galera-operator/pkg/utils/constants"
	"galera-operator/pkg/utils/probe"
	"galera-operator/pkg/utils/signals"
	"galera-operator/pkg/version"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"net/http"
	"os"
	"runtime"
	"sync"
	"time"
)

var (
	namespace      string
	name           string
	listenAddr     string
	clusterWide    bool
	bootstrapImage string
	backupImage    string
	upgradeConfig  string
	resyncPeriod   int
	runThread      int
	backupThread   int
	printVersion   bool
)

func init() {
	flag.StringVar(&listenAddr, "listen-address", constants.ListenAddress, "The galera-operator address on which the metrics and readiness probe HTTP server will listen to")
	flag.BoolVar(&clusterWide, "cluster-wide", false, "Enable operator to watch clusters in all namespaces")
	flag.IntVar(&resyncPeriod, "resync", constants.ResyncPeriod, "Resync period time between calls to controllers")
	flag.IntVar(&runThread,"run-thread", constants.RunThread, "Specify the number of thread for the run controller" )
	flag.IntVar(&backupThread,"backup-thread", constants.BackupThread, "Specify the number of thread for the backup controllers" )
	flag.StringVar(&bootstrapImage, "bootstrap", constants.BootstrapImage, "Container image used to bootstrap galera clusters (it is not the galera image)")
	flag.StringVar(&backupImage, "backup", constants.BackupImage, "Container image used to backup/restore galera clusters (it is not the galera image)")
	flag.StringVar(&upgradeConfig, "upgrade-config", constants.UpgradeConfig, "Upgrade Config CR used to check upgrades, empty string for no control")
	flag.BoolVar(&printVersion, "version", false, "Show version and quit")
}

func main() {
	// Parse flags the command-line flags from os.Args[1:]
	flag.Parse()

	if printVersion {
		fmt.Println("galera-operator Version:", version.Version)
		fmt.Println("Git SHA:", version.GitSHA)
		fmt.Println("Build Date:", version.Date)
		fmt.Println("Go Version:", runtime.Version())
		fmt.Printf("Go OS/Arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)
		os.Exit(0)
	}

	logrus.Infof("galera-operator Version: %v", version.Version)
	logrus.Infof("Git SHA: %s", version.GitSHA)
	logrus.Infof("Build Date:", version.Date)
	logrus.Infof("Go Version: %s", runtime.Version())
	logrus.Infof("Go OS/Arch: %s/%s", runtime.GOOS, runtime.GOARCH)

	namespace = os.Getenv(constants.EnvOperatorPodNamespace)
	if len(namespace) == 0 {
		logrus.Fatalf("must set env (%s)", constants.EnvOperatorPodNamespace)
	}

	name = os.Getenv(constants.EnvOperatorPodName)
	if len(name) == 0 {
		logrus.Fatalf("must set env (%s)", constants.EnvOperatorPodName)
	}

	http.HandleFunc(probe.HTTPProbeEndpoint, probe.ReadyHandler)
	http.Handle("/metrics", promhttp.Handler())
	go http.ListenAndServe(listenAddr, nil)

	ctx, cancelFunc := context.WithCancel(context.Background())

	// Set up signals so we handle the first shutdown signal gracefully.
	signals.SetupSignalHandlerWithContext(cancelFunc)

	cfg, err := rest.InClusterConfig()
	if err != nil {
		logrus.Fatalf("Error building kubeconfig: %s", err.Error())
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		logrus.Fatalf("Error building clientset: %s", err.Error())
	}

	galeraClient, err := clientset.NewForConfig(cfg)
	if err != nil {
		logrus.Fatalf("Error building galera clientset: %s", err.Error())
	}

	// ClusterWide or namespaced operator
	var operatedNamespace string
	if clusterWide == true {
		operatedNamespace = metav1.NamespaceAll
	} else {
		operatedNamespace = namespace
	}

	// use informers that filter on the namespace where the operator is deployed
	kubeInformerFactory := kubeinformers.NewSharedInformerFactoryWithOptions(kubeClient, time.Duration(resyncPeriod)*time.Second, kubeinformers.WithNamespace(operatedNamespace))
 	galeraOperatorInformerFactory := informers.NewSharedInformerFactoryWithOptions(galeraClient, time.Duration(resyncPeriod)*time.Second, informers.WithNamespace(operatedNamespace))

 	probe.SetReady()

	// Goroutines can add themselves to this to be waited on
	var wg sync.WaitGroup

	// Galera Controller
	galeraCtl := cluster.NewGaleraController(
        kubeClient,
        cfg,
        galeraClient,
		galeraOperatorInformerFactory.Sql().V1beta2().Galeras(),
		kubeInformerFactory.Core().V1().Pods(),
		kubeInformerFactory.Core().V1().Services(),
		kubeInformerFactory.Core().V1().PersistentVolumeClaims(),
		kubeInformerFactory.Storage().V1().StorageClasses(),
		kubeInformerFactory.Apps().V1().ControllerRevisions(),
		kubeInformerFactory.Policy().V1beta1().PodDisruptionBudgets(),
		kubeInformerFactory.Core().V1().Secrets(),
		galeraOperatorInformerFactory.Sql().V1beta2().UpgradeConfigs(),
		galeraOperatorInformerFactory.Sql().V1beta2().UpgradeRules(),
		kubeInformerFactory.Core().V1().ConfigMaps(),
		bootstrapImage,
		backupImage,
		upgradeConfig,
		clusterWide,
		namespace,
		resyncPeriod,
	)

	wg.Add(1)
	go func() {
		defer wg.Done()
		galeraCtl.Run(runThread, ctx.Done())
	}()

	// Galera Backup controller
	galeraBkpCtl := backup.NewGaleraBackupController(
		kubeClient,
		cfg,
		galeraClient,
		galeraOperatorInformerFactory.Sql().V1beta2().GaleraBackups(),
		galeraOperatorInformerFactory.Sql().V1beta2().Galeras(),
		kubeInformerFactory.Core().V1().Pods(),
		kubeInformerFactory.Core().V1().Secrets(),
		clusterWide,
		namespace,
		resyncPeriod,
	)

	wg.Add(1)
	go func() {
		defer wg.Done()
		galeraBkpCtl.Run(backupThread, ctx.Done())
	}()

	// Shared informers have to be started after ALL controllers.
	go kubeInformerFactory.Start(ctx.Done())
	go galeraOperatorInformerFactory.Start(ctx.Done())

	<-ctx.Done()

	logrus.Info("Waiting for all controllers to shut down gracefully")
	wg.Wait()
}
