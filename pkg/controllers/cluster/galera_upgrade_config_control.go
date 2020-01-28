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
//	"fmt"
	"github.com/sirupsen/logrus"
//	apierrors "k8s.io/apimachinery/pkg/api/errors"
//	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	galeraclientset "galera-operator/pkg/client/clientset/versioned"
	listers "galera-operator/pkg/client/listers/apigalera/v1beta2"
	"regexp"
)


// GaleraUpgradeConfigControlInterface defines the interface that GaleraController uses to access UpgradeConfig
// used by Galera Operator. It is implemented as an interface to provide for testing fakes.
type GaleraUpgradeConfigControlInterface interface{
	// CanUpgrade determines if the strategy will be an update or an upgrade based on the
	// versions provided in the image names
	// The versions must follow the [semver](http://semver.org) format, for example "10.4.5"
	CanUpgrade(currentImage, nextImage string) bool
}

// realGaleraUpgradeConfigControl implements GaleraUpgradeConfigControlInterface using a galeraclientset.Interface
// to communicate with the  API server. The struct is package private as the internal details are irrelevant to
// importing packages.
func NewRealGaleraUpgradeConfigControl(
	client galeraclientset.Interface,
	upgradeConfigLister listers.UpgradeConfigLister,
	upgradeConfigName string,
	namespace string,
	) GaleraUpgradeConfigControlInterface {
	logger := logrus.WithField("pkg", "controller")
	return &realGaleraUpgradeConfigControl{logger, client, upgradeConfigLister, upgradeConfigName, namespace}
}

type realGaleraUpgradeConfigControl struct {
	logger          	*logrus.Entry
	client 				galeraclientset.Interface
	upgradeConfigLister	listers.UpgradeConfigLister
	upgradeConfigName   string
	namespace       	string
}

// CanUpgrade determines if the strategy will be an update or an upgrade based on the
// versions provided in the image names
// The versions must follow the [semver](http://semver.org) format, for example "10.4.5"
func (gucc *realGaleraUpgradeConfigControl) CanUpgrade(currentImage, nextImage string) bool {
	if gucc.upgradeConfigName == "" {
		return true
	}

	// TODO : to implement
	return true

	/*
	if currentImage == nextImage {
		return true
	}

	upgradeConfig, err := gucc.upgradeConfigLister.UpgradeConfigs(gucc.namespace).Get(gucc.upgradeConfigName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("operator can not find upgrade config %s/%s", gucc.namespace, gucc.upgradeConfigName))
			return false
		}
		utilruntime.HandleError(fmt.Errorf("unable to retrieve operator upgrade config %s/%s : %v", gucc.namespace, gucc.upgradeConfigName, err))
		return false
	}

	majorCurrent, minorCurrent, patchCurrent := getImageVersion(currentImage)
	majorNext, minorNext, patchNext := getImageVersion(nextImage)

	currentVersion := majorCurrent + "." + minorCurrent + "." + patchCurrent
	nextVersion := majorNext + "." + minorNext + "." + patchNext

	// Image names can be different but versions can be the same. For example the os have been upgraded.
	if currentVersion == nextVersion {
		return true
	}
	*/
}

var imageVersionRegex = regexp.MustCompile(`(.*):([0-9]*\.[0-9]*\.[0-9]*)(.*)$`)
var semverRegex = regexp.MustCompile(`\.`)

func getImageVersion(name string) (major, minor, patch string) {
	subMatches := imageVersionRegex.FindStringSubmatch(name)
	if len(subMatches) < 4 {
		return major, minor, patch
	}

	semver := semverRegex.Split(subMatches[2], 3)
	if len(subMatches) < 3 {
		return major, minor, patch
	}
	major = semver[0]
	minor = semver[1]
	patch = semver[2]

	return major, minor, patch
}

var _ GaleraUpgradeConfigControlInterface = &realGaleraUpgradeConfigControl{}