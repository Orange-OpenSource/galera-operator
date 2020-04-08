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
	listers "galera-operator/pkg/client/listers/apigalera/v1beta2"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/util/retry"
)

// GaleraStatusUpdaterInterface is an interface used to update the GaleraStatus associated with a Galera.
// For any use other than testing, clients should create an instance using NewRealGaleraStatusUpdater.
type GaleraStatusUpdaterInterface interface {
	// UpdateGaleraStatus sets the galera's Status to status. Implementations are required to retry on conflicts,
	// but fail on other errors. If the returned error is nil galera's Status has been successfully set to status.
	UpdateGaleraStatus(galera *apigalera.Galera, status *apigalera.GaleraStatus) error
}

// NewRealGaleraStatusUpdater returns a GaleraStatusUpdaterInterface that updates the Status of a Galera,
// using the supplied galeraClient and galeraLister.
func NewRealGaleraStatusUpdater(
	client galeraclientset.Interface,
	galeraLister listers.GaleraLister) GaleraStatusUpdaterInterface {
	return &realGaleraStatusUpdater{client, galeraLister}
}

type realGaleraStatusUpdater struct {
	client galeraclientset.Interface
	galeraLister listers.GaleraLister
}

func (gsu *realGaleraStatusUpdater) UpdateGaleraStatus(
	galera *apigalera.Galera,
	status *apigalera.GaleraStatus) error {
	// don't wait due to limited number of clients, but backoff after the default number of steps
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		galera.Status = *status
		_, updateErr := gsu.client.SqlV1beta2().Galeras(galera.Namespace).UpdateStatus(galera)
		if updateErr == nil {
			return nil
		}
		if updated, err := gsu.galeraLister.Galeras(galera.Namespace).Get(galera.Name); err == nil {
			// make a copy so we don't mutate the shared cache
			galera = updated.DeepCopy()
		} else {
			utilruntime.HandleError(fmt.Errorf("error getting updated Galera %s/%s from lister: %v", galera.Namespace, galera.Name, err))
		}

		return updateErr
	})
}

var _ GaleraStatusUpdaterInterface = &realGaleraStatusUpdater{}
