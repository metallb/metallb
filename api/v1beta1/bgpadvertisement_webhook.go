/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1beta1

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging bgpadvertisement-webhook.
var (
	bgpAdvLog    = logf.Log.WithName("bgpadvertisement-webhook")
	bgpAdvClient client.Client
)

func (bgpAdv *BGPAdvertisement) SetupWebhookWithManager(mgr ctrl.Manager) error {
	bgpAdvClient = mgr.GetClient()
	return ctrl.NewWebhookManagedBy(mgr).
		For(bgpAdv).
		Complete()
}

//+kubebuilder:webhook:verbs=create;update,path=/validate-metallb-io-v1beta1-bgpadvertisement,mutating=false,failurePolicy=fail,groups=metallb.io,resources=bgpadvertisements,versions=v1beta1,name=bgpadvertisementvalidationwebhook.metallb.io,sideEffects=None,admissionReviewVersions=v1

var _ webhook.Validator = &BGPAdvertisement{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for BGPAdvertisement.
func (bgpAdv *BGPAdvertisement) ValidateCreate() error {
	bgpAdvLog.Info("validate BGPAdvertisement creation", "name", bgpAdv.Name)

	existingBGPAdvList, err := getExistingBGPAdvs()
	if err != nil {
		return err
	}

	existingIPPoolList, err := getExistingIPPools()
	if err != nil {
		return err
	}

	return bgpAdv.ValidateBGPAdv(true, existingBGPAdvList.Items, existingIPPoolList.Items)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for BGPAdvertisement.
func (bgpAdv *BGPAdvertisement) ValidateUpdate(old runtime.Object) error {
	bgpAdvLog.Info("validate BGPAdvertisement update", "name", bgpAdv.Name)

	existingBGPAdvList, err := getExistingBGPAdvs()
	if err != nil {
		return err
	}

	existingIPPoolList, err := getExistingIPPools()
	if err != nil {
		return err
	}

	return bgpAdv.ValidateBGPAdv(false, existingBGPAdvList.Items, existingIPPoolList.Items)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for BGPAdvertisement.
func (bgpAdv *BGPAdvertisement) ValidateDelete() error {
	bgpAdvLog.Info("validate BGPAdvertisement deletion", "name", bgpAdv.Name)

	return nil
}

func (bgpAdv *BGPAdvertisement) ValidateBGPAdv(isNewBGPAdv bool, existingBGPAdvs []BGPAdvertisement, existingIPPools []IPPool) error {
	if bgpAdv.Name == "" {
		return errors.New("Missing BGPAdvertisement name")
	}

	if isNewBGPAdv {
		err := bgpAdv.validateDuplicateBGPAdv(existingBGPAdvs)
		if err != nil {
			return err
		}
	}

	if bgpAdv.Spec.AggregationLength != nil {
		err := validateAggregationLength(*bgpAdv.Spec.AggregationLength, false)
		if err != nil {
			return err
		}
	}

	if bgpAdv.Spec.AggregationLengthV6 != nil {
		err := validateAggregationLength(*bgpAdv.Spec.AggregationLengthV6, true)
		if err != nil {
			return err
		}
	}

	err := validateDuplicateCommunities(bgpAdv.Spec.Communities)
	if err != nil {
		return err
	}

	for _, community := range bgpAdv.Spec.Communities {
		fs := strings.Split(community, ":")
		if len(fs) != 2 {
			return fmt.Errorf("invalid community string %q", community)
		}

		_, err := strconv.ParseUint(fs[0], 10, 16)
		if err != nil {
			return fmt.Errorf("invalid first section of community %q: %s", fs[0], err)
		}

		_, err = strconv.ParseUint(fs[1], 10, 16)
		if err != nil {
			return fmt.Errorf("invalid second section of community %q: %s", fs[1], err)
		}
	}

	// No pool selector means select all pools
	if len(bgpAdv.Spec.IPPools) == 0 {
		for _, ipPool := range existingIPPools {
			err := validateAggregationLengthPerPool(bgpAdv.Spec.AggregationLength, bgpAdv.Spec.AggregationLengthV6, ipPool.Spec.Addresses, ipPool.Name)
			if err != nil {
				return err
			}
		}
	} else {
		for _, ipPoolName := range bgpAdv.Spec.IPPools {
			for _, ipPool := range existingIPPools {
				if ipPool.Name == ipPoolName {
					err := validateAggregationLengthPerPool(bgpAdv.Spec.AggregationLength, bgpAdv.Spec.AggregationLengthV6, ipPool.Spec.Addresses, ipPool.Name)
					if err != nil {
						return err
					}
				}
			}
		}
	}

	return nil
}

func (bgpAdv *BGPAdvertisement) validateDuplicateBGPAdv(existingBGPAdvs []BGPAdvertisement) error {
	for _, existingAdv := range existingBGPAdvs {
		if reflect.DeepEqual(bgpAdv, existingAdv) {
			return fmt.Errorf("duplicate definition of bgpadvertisements. advertisement name %s", existingAdv.Name)
		}
	}
	return nil
}

func getExistingBGPAdvs() (*BGPAdvertisementList, error) {
	existingBGPAdvList := &BGPAdvertisementList{}
	err := bgpAdvClient.List(context.Background(), existingBGPAdvList)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to get existing BGPAdvertisement objects")
	}
	return existingBGPAdvList, nil
}
