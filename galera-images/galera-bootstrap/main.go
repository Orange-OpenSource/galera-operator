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
	"flag"
	"fmt"
	"github.com/sirupsen/logrus"
	"io/ioutil"
	"os"
	"regexp"
	"galera-bootstrap/pkg/version"
	"runtime"
)

const (
	EnvPodIP = "POD_IP"
)

// clusterNameRegex is a regular expression that extracts wsrep_node_address
var clusterNameRegex = regexp.MustCompile("(?im)^wsrep_cluster_name(.*)$")
// clusterAddressesRegex is a regular expression that extracts wsrep_cluster_address
var clusterAddressesRegex = regexp.MustCompile("(?im)^wsrep_cluster_address(.*)$")
// nodeNameRegex is a regular expression that extracts wsrep_node_name
var nodeNameRegex = regexp.MustCompile("(?im)^wsrep_node_name(.*)$")
// nodeAddressRegex is a regular expression that extracts wsrep_node_address
var nodeAddressRegex = regexp.MustCompile("(?im)^wsrep_node_address(.*)$")
// clusterRegex is a regular expression that extracts wsrep_on
var clusterRegex = regexp.MustCompile("(?im)^wsrep_on(.*)$")
// accountRegex is a regular expression that extracts wsrep_sst_auth
var accountRegex = regexp.MustCompile("(?im)^wsrep_sst_auth(.*)$")
// dataDirRegex is a regular expression that extracts datadir
var dataDirRegex = regexp.MustCompile("(?im)^datadir(.*)$")

var inputConf, outputConf, clusterName, nodeName, clusterAddresses, user, password string
var clusterEnabled, printVersion   bool

func init() {
	flag.StringVar(&inputConf, "input-conf", "input/my.cnf" ,"galera configuration input file path")
	flag.StringVar(&outputConf, "output-conf", "output/my.cnf" ,"galera configuration output file path")
	flag.StringVar(&clusterName, "cluster-name", "test-cluster", "galera cluster name")
	flag.StringVar(&clusterAddresses, "cluster-addresses", "", "galera cluster addresses, use \"\" to create a new cluster or \"ip1,ip2\" to join an existing cluster")
	flag.StringVar(&nodeName, "node-name", "test-node", "galera node name")
	flag.StringVar(&user, "user", "", "galera user account, need some privileges")
	flag.StringVar(&password, "password", "", "galera user password")
	flag.BoolVar(&clusterEnabled, "cluster", true, "galera cluster on")
	flag.BoolVar(&printVersion, "version", false, "Show version and quit")
}

// addInfo insert a comment to specify that parameters are added to mysql config file
func addInfo(data string, added *bool) string {
	if *added == false {
		*added = true
		return data + "\n\n# Added by Galera Bootstrap\n"
	} else {
		return data
	}
}

// modifyFile modifies or inserts some parameters/values in the mysql config file
func modifyFile() *string {
	nodeAddress := os.Getenv(EnvPodIP)

	if len(nodeAddress) == 0 {
		logrus.Fatalf("must set env (%s)", EnvPodIP)
	}

	bData, err := ioutil.ReadFile(inputConf)
	if err != nil {
		logrus.Fatalf("error opening %s file", inputConf)
	}

	added := false

	// work with string and not byte
	sData := string(bData)

	// Setting wsrep_node_address
	replaceClusterName := fmt.Sprintf(`wsrep_cluster_name=` + clusterName)

	if string(clusterNameRegex.Find([]byte(sData))) != "" {
		sData = clusterNameRegex.ReplaceAllString(sData, replaceClusterName)
	} else {
		sData = addInfo(sData, &added)
		sData = sData + replaceClusterName + "\n"
	}

	// Setting wsrep_cluster_address
	replaceClusterAddresses := fmt.Sprintf(`wsrep_cluster_address="gcomm://` + clusterAddresses + `"`)

	if string(clusterAddressesRegex.Find([]byte(sData))) != "" {
		sData = clusterAddressesRegex.ReplaceAllString(sData, replaceClusterAddresses)
	} else {
		sData = addInfo(sData, &added)
		sData = sData + replaceClusterAddresses + "\n"
	}

	// Setting wsrep_node_name
	replaceNodeName := fmt.Sprintf(`wsrep_node_name=` + nodeName)
	
	if string(nodeNameRegex.Find([]byte(sData))) != "" {
		sData = nodeNameRegex.ReplaceAllString(sData, replaceNodeName)
	} else {
		sData = addInfo(sData, &added)
		sData = sData + replaceNodeName + "\n"
	}

	// Setting  wsrep_node_address
	replaceNodeAddress := fmt.Sprintf(`wsrep_node_address=` + nodeAddress)

	if string(nodeAddressRegex.Find([]byte(sData))) != "" {
		sData = nodeAddressRegex.ReplaceAllString(sData, replaceNodeAddress)
	} else {
		sData = addInfo(sData, &added)
		sData = sData + replaceNodeAddress + "\n"
	}

	// Setting wsrep_sst_auth
	replaceAccount := fmt.Sprintf(`wsrep_sst_auth="` + user + `:` + password + `"`)

	if string(accountRegex.Find([]byte(sData))) != "" {
		sData = accountRegex.ReplaceAllString(sData, replaceAccount)
	} else {
		sData = addInfo(sData, &added)
		sData = sData + replaceAccount + "\n"
	}

	// Setting wsrep_on
	enabled := "OFF"
	if clusterEnabled {
		enabled = "ON"
	}
	replaceWsrep := fmt.Sprintf("wsrep_on=" + enabled)

	if string(clusterRegex.Find([]byte(sData))) != "" {
		sData = clusterRegex.ReplaceAllString(sData, replaceWsrep)
	} else {
		sData = addInfo(sData, &added)
		sData = sData + replaceWsrep + "\n"
	}

	// Setting datadir
	dataDir := `datadir=/var/lib/mysql`

	if string(dataDirRegex.Find([]byte(sData))) != "" {
		sData = dataDirRegex.ReplaceAllString(sData, dataDir)
	} else {
		sData = addInfo(sData, &added)
		sData = sData + dataDir + "\n"
	}

	return &sData
}

func main() {
	// Parse flags the command-line flags from os.Args[1:]
	flag.Parse()

	if printVersion {
		fmt.Println("galera-bootsrap Version:", version.Version)
		fmt.Println("Git SHA:", version.GitSHA)
		fmt.Println("Build Date:", version.Date)
		fmt.Println("Go Version:", runtime.Version())
		fmt.Printf("Go OS/Arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)
		os.Exit(0)
	}

	sData := modifyFile()

	if err := ioutil.WriteFile(outputConf, []byte(*sData), 0777); err != nil {
		logrus.Fatalf("error writing file")
	}
}
