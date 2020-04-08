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
	listers "galera-operator/pkg/client/listers/apigalera/v1beta2"
	logrus "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/record"
	"regexp"
)

// GaleraUpgradeConfigControlInterface defines the interface that GaleraController uses to access UpgradeConfig
// used by Galera Operator. It is implemented as an interface to provide for testing fakes.
type GaleraUpgradeConfigControlInterface interface{
	// CanUpgrade determines if the strategy will be an update or an upgrade based on the
	// versions provided in the image names
	// The versions must follow the [semver](http://semver.org) format, for example "10.4.5"
	CanUpgrade(currentImage, nextImage string, config *corev1.LocalObjectReference, galera *apigalera.Galera) (bool, error)
}

// realGaleraUpgradeConfigControl implements GaleraUpgradeConfigControlInterface. The struct is package private
// as the internal details are irrelevant to importing packages.
func NewRealGaleraUpgradeConfigControl(
	upgradeConfigLister listers.UpgradeConfigLister,
	upgradeRuleLister listers.UpgradeRuleLister,
	configMapLister corelisters.ConfigMapLister,
	upgradeConfigName string,
	namespace string,
	recorder record.EventRecorder,
	) GaleraUpgradeConfigControlInterface {
	logger := logrus.WithField("pkg", "controller")
	return &realGaleraUpgradeConfigControl{logger, upgradeConfigLister, upgradeRuleLister, configMapLister, upgradeConfigName, namespace, recorder}
}

type realGaleraUpgradeConfigControl struct {
	logger              *logrus.Entry
	upgradeConfigLister listers.UpgradeConfigLister
	upgradeRuleLister   listers.UpgradeRuleLister
	configMapLister     corelisters.ConfigMapLister
	upgradeConfigName   string
	namespace           string
	recorder    		record.EventRecorder
}

// CanUpgrade determines if galer can be upgraded.
// 1. The nextImage version is greater than the currentImage version. The versions must follow the
// [semver](http://semver.org) format, for example "10.4.5"
// 2. Follow the rules provided by the upgrade config file if present
func (gucc *realGaleraUpgradeConfigControl) CanUpgrade(
	currentImage, nextImage string,
	config *corev1.LocalObjectReference,
	galera *apigalera.Galera) (bool, error) {

	// debug
	nextImage = "sebs42/mariadb:10.5.1-bionic"

	logrus.Infof("SEB: current : %s", currentImage)
	logrus.Infof("SEB: next : %s", nextImage)

	if currentImage == nextImage {
		return true, nil
	}

	// If no upgrade config is configured, just check if the next version is greater thant the current one
	if !isImageGreater(currentImage, nextImage) {
		return false, nil
	}

	if gucc.upgradeConfigName == "" {
		return true, nil
	}

	err := gucc.canUpgrade(currentImage, nextImage, config)

	gucc.recordUpgradeEvent(galera, err)

	if err == nil {
		return true, nil
	} else {
		return false, err
	}
}

func (gucc *realGaleraUpgradeConfigControl) canUpgrade(
	currentImage, nextImage string,
	config *corev1.LocalObjectReference) error {

	upgradeConfig, err := gucc.upgradeConfigLister.UpgradeConfigs(gucc.namespace).Get(gucc.upgradeConfigName)
	if err != nil {
		return fmt.Errorf("unable to retrieve operator upgrade config %s/%s : %v", gucc.namespace, gucc.upgradeConfigName, err)
	}

	configMap, err := gucc.configMapLister.ConfigMaps(gucc.namespace).Get(config.Name)
	if err != nil {
		return fmt.Errorf("unable to retrieve configMap %s/%s : %v", gucc.namespace, config.Name, err)
	}

	return gucc.validateConfig(upgradeConfig, configMap, nextImage)
}

func (gucc *realGaleraUpgradeConfigControl) validateConfig(upgradeConfig *apigalera.UpgradeConfig, configMap *corev1.ConfigMap, version string) error {

	logrus.Infof("SEB: validateConfig")

//	logrus.Infof("upgConfig := %+v", upgradeConfig)
	mycnf, exist := configMap.Data["my.cnf"]
	if !exist {
		return fmt.Errorf("unable to retrieve my.cnf from configMap %s/%s", configMap.Namespace, configMap.Name)
	}

	cnf := []byte(mycnf)

//	logrus.Infof("exist : %t mycnf : %s ", exist, mycnf)

	upgRules := upgradeConfig.UpgradeRules

	major, minor, patch := getImageVersion(version)

	for _, versionRules := range upgRules {
		semver := semverRegex.Split(versionRules.TargetVersion, 3)
		logrus.Infof("semver : %+v",semver)

		switch len(semver) {
		case 1:
			if major == semver[0] {
				if err := gucc.checkRemovedRules(versionRules.Rules, cnf); err != nil {
					return err
				}
			}
		case 2:
			if major == semver[0] && minor == semver[1] {
				if err := gucc.checkRemovedRules(versionRules.Rules, cnf); err != nil {
					return err
				}
			}
		case 3:
			if major == semver[0] && minor == semver[1] && patch == semver[2] {
				if err := gucc.checkRemovedRules(versionRules.Rules, cnf); err != nil {
					return err
				}
			}
		default:
			return fmt.Errorf("unexpexted semver version : %s", versionRules.TargetVersion)
		}
	}

	return nil
}


func (gucc *realGaleraUpgradeConfigControl) checkRemovedRules(rules []corev1.LocalObjectReference, cnf []byte) error {
	for _, rule := range rules {

		logrus.Infof("check rule: %+v rule.Name %s", rule, rule.Name)

		upgRule, err := gucc.upgradeRuleLister.UpgradeRules(gucc.namespace).Get(rule.Name)
		if err != nil {
			return fmt.Errorf("unable to retrieve operator upgrade rule %s/%s : %v", gucc.namespace, rule.Name, err)
		}

		if upgRule.RemovedOption != nil {
			logrus.Infof("****** removed option : %s", upgRule.RemovedOption.Option)
			rmRegex := regexp.MustCompile("(?im)^" + upgRule.RemovedOption.Option + "(.*)$")
			if rmRegex.Find(cnf) != nil {
				logrus.Infof("removed option found : %s", upgRule.RemovedOption.Option)
				return fmt.Errorf("removed option found : %s", upgRule.RemovedOption.Option)
			}
		}
	}

	return nil
}

// recordUpgradeEvent records an event about upgrade validation. If err is nil the generated event will
// have a reason of v1.EventTypeNormal. If err is not nil the generated event will have a reason of v1.EventTypeWarning.
func (gucc *realGaleraUpgradeConfigControl) recordUpgradeEvent(galera *apigalera.Galera, err error) {
	if err == nil {
		reason := "SuccessfulValidate"
		message := fmt.Sprintf("Validate my.cnf for Galera %s/%s successful", galera.Namespace, galera.Name)
		gucc.recorder.Event(galera, corev1.EventTypeNormal, reason, message)
	} else {
		reason := "FailedValidate"
		message := fmt.Sprintf("Validate my.cnf for Galera %s/%s failed error: %s", galera.Namespace, galera.Name, err)
		gucc.recorder.Event(galera, corev1.EventTypeWarning, reason, message)
	}
}

// isImageGreate compare semver names and returns true if the next image is greate thant the current one
func isImageGreater(current, next string) bool {
	majorCurrent, minorCurrent, patchCurrent := getImageVersion(current)
	majorNext, minorNext, patchNext := getImageVersion(next)

	// compare majors
	if majorNext < majorCurrent {
		return false
	}
	if majorNext > majorCurrent {
		return  true
	}

	// compare minors because majors are equal
	if minorNext < minorCurrent {
		return false
	}
	if minorNext > minorCurrent {
		return true
	}

	// compare patch because majors and minors are equal
	if patchNext < patchCurrent {
		return false
	}

	return true
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
