// SPDX-License-Identifier:Apache-2.0

package config

import (
	"context"

	frrk8sv1beta1 "github.com/metallb/frr-k8s/api/v1beta1"
	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	metallbv1beta2 "go.universe.tf/metallb/api/v1beta2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type Resources struct {
	Pools           []metallbv1beta1.IPAddressPool    `json:"ipaddresspools"`
	Peers           []metallbv1beta2.BGPPeer          `json:"bgppeers"`
	BFDProfiles     []metallbv1beta1.BFDProfile       `json:"bfdprofiles"`
	BGPAdvs         []metallbv1beta1.BGPAdvertisement `json:"bgpadvertisements"`
	L2Advs          []metallbv1beta1.L2Advertisement  `json:"l2advertisements"`
	Communities     []metallbv1beta1.Community        `json:"communities"`
	PasswordSecrets map[string]corev1.Secret          `json:"passwordsecrets"`
	Nodes           []corev1.Node                     `json:"nodes"`
	Namespaces      []corev1.Namespace                `json:"namespaces"`
	BGPExtras       corev1.ConfigMap                  `json:"bgpextras"`
}

type Updater interface {
	Update(r Resources) error
	Clean() error
	Client() client.Client
	Namespace() string
}

type beta1Updater struct {
	cli       client.Client
	namespace string
}

func UpdaterForCRs(r *rest.Config, ns string) (Updater, error) {
	myScheme := runtime.NewScheme()

	if err := metallbv1beta1.AddToScheme(myScheme); err != nil {
		return nil, err
	}

	if err := metallbv1beta2.AddToScheme(myScheme); err != nil {
		return nil, err
	}

	if err := corev1.AddToScheme(myScheme); err != nil {
		return nil, err
	}

	if err := frrk8sv1beta1.AddToScheme(myScheme); err != nil {
		return nil, err
	}

	cl, err := client.New(r, client.Options{
		Scheme: myScheme,
	})

	if err != nil {
		return nil, err
	}

	return &beta1Updater{
		cli:       cl,
		namespace: ns,
	}, nil
}

func (o beta1Updater) Update(r Resources) error {
	// we fill a map of objects to keep the order we add the resources random, as
	// it would happen by throwing a set of manifests against a cluster, hoping to
	// find corner cases that we would not find by adding them always in the same
	// order.
	objects := map[int]client.Object{}
	oldValues := map[int]client.Object{}
	key := 0
	for _, pool := range r.Pools {
		objects[key] = pool.DeepCopy()
		oldValues[key] = pool.DeepCopy()
		key++
	}

	for _, secret := range r.PasswordSecrets {
		objects[key] = secret.DeepCopy()
		oldValues[key] = secret.DeepCopy()
		key++
	}

	for _, peer := range r.Peers {
		objects[key] = peer.DeepCopy()
		oldValues[key] = peer.DeepCopy()
		key++
	}

	for _, bfdProfile := range r.BFDProfiles {
		objects[key] = bfdProfile.DeepCopy()
		oldValues[key] = bfdProfile.DeepCopy()
		key++
	}

	for _, bgpAdv := range r.BGPAdvs {
		objects[key] = bgpAdv.DeepCopy()
		oldValues[key] = bgpAdv.DeepCopy()
		key++
	}

	for _, l2Adv := range r.L2Advs {
		objects[key] = l2Adv.DeepCopy()
		oldValues[key] = l2Adv.DeepCopy()
		key++
	}

	for _, community := range r.Communities {
		objects[key] = community.DeepCopy()
		oldValues[key] = community.DeepCopy()
		key++
	}

	// Iterating over the map will return the items in a random order.
	for i, obj := range objects {
		obj.SetNamespace(o.namespace)
		_, err := controllerutil.CreateOrPatch(context.Background(), o.cli, obj, func() error {
			// the mutate function is expected to change the object when updating.
			// we always override with the old version, and we change only the spec part.
			switch toChange := obj.(type) {
			case *metallbv1beta1.IPAddressPool:
				old := oldValues[i].(*metallbv1beta1.IPAddressPool)
				toChange.Spec = *old.Spec.DeepCopy()
			case *metallbv1beta1.BFDProfile:
				old := oldValues[i].(*metallbv1beta1.BFDProfile)
				toChange.Spec = *old.Spec.DeepCopy()
			case *metallbv1beta2.BGPPeer:
				old := oldValues[i].(*metallbv1beta2.BGPPeer)
				toChange.Spec = *old.Spec.DeepCopy()
			case *metallbv1beta1.BGPAdvertisement:
				old := oldValues[i].(*metallbv1beta1.BGPAdvertisement)
				toChange.Spec = *old.Spec.DeepCopy()
			case *metallbv1beta1.L2Advertisement:
				old := oldValues[i].(*metallbv1beta1.L2Advertisement)
				toChange.Spec = *old.Spec.DeepCopy()
			case *metallbv1beta1.Community:
				old := oldValues[i].(*metallbv1beta1.Community)
				toChange.Spec = *old.Spec.DeepCopy()
			}

			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (o beta1Updater) Clean() error {
	err := o.cli.DeleteAllOf(context.Background(), &metallbv1beta1.IPAddressPool{}, client.InNamespace(o.namespace))
	if err != nil {
		return err
	}
	err = o.cli.DeleteAllOf(context.Background(), &metallbv1beta2.BGPPeer{}, client.InNamespace(o.namespace))
	if err != nil {
		return err
	}
	err = o.cli.DeleteAllOf(context.Background(), &metallbv1beta1.BFDProfile{}, client.InNamespace(o.namespace))
	if err != nil {
		return err
	}
	err = o.cli.DeleteAllOf(context.Background(), &metallbv1beta1.BGPAdvertisement{}, client.InNamespace(o.namespace))
	if err != nil {
		return err
	}
	err = o.cli.DeleteAllOf(context.Background(), &metallbv1beta1.L2Advertisement{}, client.InNamespace(o.namespace))
	if err != nil {
		return err
	}
	err = o.cli.DeleteAllOf(context.Background(), &metallbv1beta1.Community{}, client.InNamespace(o.namespace))
	if err != nil {
		return err
	}
	return nil
}

func (o beta1Updater) Client() client.Client {
	return o.cli
}

func (o beta1Updater) Namespace() string {
	return o.namespace
}
