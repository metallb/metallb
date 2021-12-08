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
	"net"

	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging bgppeer-webhook
var bgppeerlog = logf.Log.WithName("bgppeer-webhook")
var bgpClient client.Client

var BGPFrrMode = false

func (bgpPeer *BGPPeer) SetupWebhookWithManager(mgr ctrl.Manager, bgpType string) error {
	if bgpType == "frr" {
		BGPFrrMode = true
	}
	bgpClient = mgr.GetClient()
	return ctrl.NewWebhookManagedBy(mgr).
		For(bgpPeer).
		Complete()
}

//+kubebuilder:webhook:verbs=create;update,path=/validate-metallb-io-v1beta1-bgppeer,mutating=false,failurePolicy=fail,groups=metallb.io,resources=bgppeers,versions=v1beta1,name=bgppeervalidationwebhook.metallb.io,sideEffects=None,admissionReviewVersions=v1

var _ webhook.Validator = &BGPPeer{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (bgpPeer *BGPPeer) ValidateCreate() error {
	bgppeerlog.Info("validate create", "name", bgpPeer.Name)
	existingBGPPeersList, err := getExistingBGPPeers()
	if err != nil {
		return err
	}
	return bgpPeer.validateBGPPeer(existingBGPPeersList, BGPFrrMode)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (bgpPeer *BGPPeer) ValidateUpdate(old runtime.Object) error {
	bgppeerlog.Info("validate update", "name", bgpPeer.Name)
	existingBGPPeersList, err := getExistingBGPPeers()
	if err != nil {
		return err
	}
	return bgpPeer.validateBGPPeer(existingBGPPeersList, BGPFrrMode)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (bgpPeer *BGPPeer) ValidateDelete() error {
	bgppeerlog.Info("validate delete", "name", bgpPeer.Name)

	return nil
}

func (bgpPeer *BGPPeer) validateBGPPeer(existingBGPPeersList *BGPPeerList, bgpFrrMode bool) error {
	var allErrs field.ErrorList

	if err := bgpPeer.validateBGPPeersRouterID(existingBGPPeersList); err != nil {
		allErrs = append(allErrs, err)
	}
	if bgpFrrMode {
		if err := bgpPeer.validateBGPPeersMyASN(existingBGPPeersList); err != nil {
			allErrs = append(allErrs, err)
		}
	}
	if err := bgpPeer.validateBGPPeerConfig(existingBGPPeersList); err != nil {
		allErrs = append(allErrs, err)
	}
	if err := bgpPeer.validateBGPPeersKeepaliveTime(existingBGPPeersList); err != nil {
		allErrs = append(allErrs, err)
	}
	if len(allErrs) == 0 {
		return nil
	}

	err := apierrors.NewInvalid(
		schema.GroupKind{Group: "metallb.io", Kind: "BGPPeer"},
		bgpPeer.Name, allErrs)
	return err
}

func (bgpPeer *BGPPeer) validateBGPPeersKeepaliveTime(existingBGPPeersList *BGPPeerList) *field.Error {
	holdTime := bgpPeer.Spec.HoldTime
	keepaliveTime := bgpPeer.Spec.KeepaliveTime

	// Keepalivetime is not set we can't do any validation, return without doing keepalive validation
	if keepaliveTime == 0 {
		return nil
	}
	// If we come here then user configured KeepaliveTime and we need to make sure holdTime is also configured
	if holdTime == 0 {
		return field.Invalid(field.NewPath("spec").Child("HoldTime"), holdTime,
			fmt.Sprintf("Missing to configure HoldTime when changing KeepaliveTime to %s", keepaliveTime))
	}
	// keepalive must be lower than holdtime by RFC4271 Keepalive Timer algorithm
	if keepaliveTime > holdTime {
		return field.Invalid(field.NewPath("spec").Child("KeepaliveTime"), keepaliveTime,
			fmt.Sprintf("Invalid keepalive time %s higher than holdtime %s", keepaliveTime, holdTime))
	}
	return nil
}

func (bgpPeer *BGPPeer) validateBGPPeersRouterID(existingBGPPeersList *BGPPeerList) *field.Error {
	routerID := bgpPeer.Spec.RouterID

	if len(routerID) == 0 {
		return nil
	}
	if net.ParseIP(routerID) == nil {
		return field.Invalid(field.NewPath("spec").Child("RouterID"), routerID,
			fmt.Sprintf("Invalid RouterID %s", routerID))
	}
	return nil
}

func (bgpPeer *BGPPeer) validateBGPPeersMyASN(existingBGPPeersList *BGPPeerList) *field.Error {
	myASN := bgpPeer.Spec.MyASN
	for _, BGPPeer := range existingBGPPeersList.Items {
		if myASN != BGPPeer.Spec.MyASN {
			return field.Invalid(field.NewPath("spec").Child("MyASN"), myASN,
				fmt.Sprintf("Multiple local ASN not supported in FRR mode, myASN %d existing myASN %d",
					myASN, BGPPeer.Spec.MyASN))
		}
	}

	return nil
}

func (bgpPeer *BGPPeer) validateBGPPeerConfig(existingBGPPeersList *BGPPeerList) *field.Error {
	remoteASN := bgpPeer.Spec.ASN
	myASN := bgpPeer.Spec.MyASN
	address := bgpPeer.Spec.Address
	srcAddr := bgpPeer.Spec.SrcAddress

	if net.ParseIP(address) == nil {
		return field.Invalid(field.NewPath("spec").Child("Address"), address,
			fmt.Sprintf("Invalid BGPPeer address %s", address))
	}

	if len(srcAddr) != 0 && net.ParseIP(srcAddr) == nil {
		return field.Invalid(field.NewPath("spec").Child("SrcAddress"), srcAddr,
			fmt.Sprintf("Invalid BGPPeer source address %s", srcAddr))
	}

	for _, BGPPeer := range existingBGPPeersList.Items {
		if remoteASN == BGPPeer.Spec.ASN && address == BGPPeer.Spec.Address && myASN == BGPPeer.Spec.MyASN {
			return field.Invalid(field.NewPath("spec").Child("Address"), address,
				fmt.Sprintf("Duplicate BGPPeer %s ASN %d in the same BGP instance",
					address, remoteASN))
		}
	}
	return nil
}

func getExistingBGPPeers() (*BGPPeerList, error) {
	existingBGPPeerslList := &BGPPeerList{}
	err := bgpClient.List(context.Background(), existingBGPPeerslList)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to get existing BGPPeer objects")
	}
	return existingBGPPeerslList, nil
}
