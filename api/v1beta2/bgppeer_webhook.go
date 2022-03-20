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

package v1beta2

import (
	"context"
	"fmt"
	"net"

	"time"

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

// log is for logging bgppeer-webhook.
var bgpPeerLog = logf.Log.WithName("bgppeer-webhook")
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

//+kubebuilder:webhook:verbs=create;update,path=/validate-metallb-io-v1beta2-bgppeer,mutating=false,failurePolicy=fail,groups=metallb.io,resources=bgppeers,versions=v1beta2,name=bgppeervalidationwebhook.v1beta2.metallb.io,sideEffects=None,admissionReviewVersions=v1

var _ webhook.Validator = &BGPPeer{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (bgpPeer *BGPPeer) ValidateCreate() error {
	bgpPeerLog.Info("validate BGPPeer create", "name", bgpPeer.Name)
	existingBGPPeers, err := getExistingBGPPeers()
	if err != nil {
		return err
	}
	return bgpPeer.ValidateBGPPeer(existingBGPPeers.Items, BGPFrrMode)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (bgpPeer *BGPPeer) ValidateUpdate(old runtime.Object) error {
	bgpPeerLog.Info("validate BGPPeer update", "name", bgpPeer.Name)
	existingBGPPeers, err := getExistingBGPPeers()
	if err != nil {
		return err
	}
	return bgpPeer.ValidateBGPPeer(existingBGPPeers.Items, BGPFrrMode)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (bgpPeer *BGPPeer) ValidateDelete() error {
	bgpPeerLog.Info("validate BGPPeer delete", "name", bgpPeer.Name)

	return nil
}

func (bgpPeer *BGPPeer) ValidateBGPPeer(existingBGPPeers []BGPPeer, bgpFrrMode bool) error {
	var allErrs field.ErrorList

	if bgpPeer.Spec.MyASN == 0 {
		return errors.New("missing local ASN")
	}
	if bgpPeer.Spec.ASN == 0 {
		return errors.New("missing peer ASN")
	}

	if bgpFrrMode {
		if err := bgpPeer.validateBGPPeersMyASN(existingBGPPeers); err != nil {
			allErrs = append(allErrs, err)
		}
		if err := bgpPeer.validateDuplicateBGPPeer(existingBGPPeers); err != nil {
			allErrs = append(allErrs, err)
		}
		if err := bgpPeer.validateBGPPeerMultiHop(); err != nil {
			allErrs = append(allErrs, err)
		}
	}
	if err := bgpPeer.validateBGPPeersKeepaliveTime(); err != nil {
		allErrs = append(allErrs, err)
	}
	if err := bgpPeer.validateBGPPeersRouterID(existingBGPPeers, bgpFrrMode); err != nil {
		allErrs = append(allErrs, err)
	}
	if err := bgpPeer.validateBGPPeerAddrConfig(); err != nil {
		allErrs = append(allErrs, err)
	}
	if err := bgpPeer.validateBGPPeersHoldTime(existingBGPPeers); err != nil {
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

func (bgpPeer *BGPPeer) validateBGPPeersKeepaliveTime() *field.Error {
	holdTime := bgpPeer.Spec.HoldTime
	keepaliveTime := bgpPeer.Spec.KeepaliveTime
	// Keepalivetime is not set we can't do any validation, return without doing keepalive validation
	if keepaliveTime.Duration == 0 {
		return nil
	}
	// If we come here then user configured KeepaliveTime and we need to make sure holdTime is also configured
	if holdTime.Duration == 0 {
		return field.Invalid(field.NewPath("spec").Child("HoldTime"), holdTime,
			fmt.Sprintf("Missing to configure HoldTime when changing KeepaliveTime to %s", keepaliveTime.String()))
	}
	// keepalive must be lower than holdtime by RFC4271 Keepalive Timer algorithm
	if keepaliveTime.Duration > holdTime.Duration {
		return field.Invalid(field.NewPath("spec").Child("KeepaliveTime"), keepaliveTime,
			fmt.Sprintf("Invalid keepalive time %s higher than holdtime %s", keepaliveTime.String(), holdTime.String()))
	}
	return nil
}

func (bgpPeer *BGPPeer) validateBGPPeersHoldTime(existingBGPPeers []BGPPeer) *field.Error {
	holdTime := bgpPeer.Spec.HoldTime
	if holdTime.Duration != 0 && holdTime.Duration < 3*time.Second {
		return field.Invalid(field.NewPath("spec").Child("HoldTime"), holdTime,
			fmt.Sprintf("Invalid hold time %s must be 0 or >=3s", holdTime.String()))
	}
	return nil
}

func (bgpPeer *BGPPeer) validateBGPPeersRouterID(existingBGPPeers []BGPPeer, bgpFrrMode bool) *field.Error {
	routerID := bgpPeer.Spec.RouterID
	if len(routerID) == 0 {
		return nil
	}
	if net.ParseIP(routerID) == nil {
		return field.Invalid(field.NewPath("spec").Child("RouterID"), routerID,
			fmt.Sprintf("Invalid RouterID %s", routerID))
	}
	if bgpFrrMode {
		for _, existingBGPPeer := range existingBGPPeers {
			if bgpPeer.Name != existingBGPPeer.Name && routerID != existingBGPPeer.Spec.RouterID {
				return field.Invalid(field.NewPath("spec").Child("RouterID"), routerID,
					fmt.Sprintf("BGPPeers with different RouterID not supported in FRR mode, RouterID %s existing routerID %s",
						routerID, existingBGPPeer.Spec.RouterID))
			}
		}
	}
	return nil
}

func (bgpPeer *BGPPeer) validateBGPPeersMyASN(existingBGPPeers []BGPPeer) *field.Error {
	myASN := bgpPeer.Spec.MyASN
	for _, BGPPeer := range existingBGPPeers {
		if bgpPeer.Name != BGPPeer.Name && myASN != BGPPeer.Spec.MyASN {
			return field.Invalid(field.NewPath("spec").Child("MyASN"), myASN,
				fmt.Sprintf("Multiple local ASN not supported in FRR mode, myASN %d existing myASN %d",
					myASN, BGPPeer.Spec.MyASN))
		}
	}

	return nil
}

func (bgpPeer *BGPPeer) validateBGPPeerAddrConfig() *field.Error {
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
	return nil
}

func (bgpPeer *BGPPeer) validateBGPPeerMultiHop() *field.Error {
	myASN := bgpPeer.Spec.MyASN
	remoteASN := bgpPeer.Spec.ASN
	eBGPMultiHop := bgpPeer.Spec.EBGPMultiHop
	if remoteASN == myASN && eBGPMultiHop {
		return field.Invalid(field.NewPath("spec").Child("EBGPMultiHop"), eBGPMultiHop,
			fmt.Sprintf("Invalid EBGPMultiHop parameter set for an ibgp peer %v", eBGPMultiHop))
	}
	return nil
}

func (bgpPeer *BGPPeer) validateDuplicateBGPPeer(existingBGPPeers []BGPPeer) *field.Error {
	address := bgpPeer.Spec.Address
	for _, BGPPeer := range existingBGPPeers {
		if bgpPeer.Name != BGPPeer.Name && address == BGPPeer.Spec.Address {
			return field.Invalid(field.NewPath("spec").Child("Address"), address,
				fmt.Sprintf("Duplicate BGPPeer %s in the same BGP instance not supported in FRR mode",
					address))
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
