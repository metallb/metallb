// SPDX-License-Identifier:Apache-2.0

package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mikioh/ipaddr"
	"go.universe.tf/metallb/api/v1beta1"
	"go.universe.tf/metallb/api/v1beta2"
	"go.universe.tf/metallb/internal/config"
	"go.universe.tf/metallb/internal/version"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"sigs.k8s.io/yaml"
)

const (
	separator      = "---\n"
	autoGenComment = "# This was autogenerated by MetalLB's custom resource generator.\n"
	outPutFileName = "resources.yaml"
)

var (
	resourcesNameSpace = "metallb-system"
	inputDirPath       = "/var/input"
)

func main() {
	source := flag.String("source", "./config.yaml", "name of the configmap file to convert")
	flag.Parse()
	log.Printf("MetalLB generator starting. commit: %s branch: %s goversion: %s",
		version.CommitHash(), version.Branch(), version.GoString())

	f, err := os.Create(filepath.Join(inputDirPath, outPutFileName))
	if err != nil {
		log.Fatalf("failed to create output file: %s", err)
	}
	defer f.Close()

	err = generate(f, *source)
	if err != nil {
		log.Fatalf("failed to generate resources: %s", err)
	}
	log.Println("Generator finished successfully!")
}

// generate gets a name of a metallb configmap file, converts it to
// the matching metallb custom resources yamls, and returns it as a string.
func generate(w io.Writer, origin string) error {
	log.Println("Reading configmap")
	raw, err := readConfig(origin)
	if err != nil {
		return err
	}

	log.Println("Decoding configmap")
	cf, err := decodeConfigFile(raw)
	if err != nil {
		return err
	}

	log.Println("Creating custom resources")
	resources, err := resourcesFor(cf)
	if err != nil {
		return err
	}

	log.Println("Validating the custom resources")
	_, err = config.For(resources, config.DontValidate)
	if err != nil {
		return err
	}

	log.Println("Creating the output YAML")
	_, err = w.Write([]byte(autoGenComment))
	if err != nil {
		return err
	}
	err = createResourcesYAMLs(w, resources)
	if err != nil {
		return err
	}

	return nil
}

func readConfig(origin string) ([]byte, error) {
	fp := filepath.Join(inputDirPath, origin)
	f, err := os.Open(fp)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %s: %v", fp, err)
	}
	defer f.Close()

	raw, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %v", origin, err)
	}

	return raw, nil
}

// decodeConfigFile gets metallb configmap raw bytes and decodes it into
// a configFile object.
func decodeConfigFile(raw []byte) (*configFile, error) {
	scheme, err := initSchema()
	if err != nil {
		return nil, err
	}

	factory := serializer.NewCodecFactory(scheme)
	decoder := factory.UniversalDeserializer()
	obj, _, err := decoder.Decode(raw, nil, nil)
	if err != nil {
		return nil, err
	}

	config, ok := obj.(*corev1.ConfigMap)
	if !ok {
		return nil, fmt.Errorf("not a configmap")
	}

	// set the namespace for the resources
	if config.Namespace != "" {
		resourcesNameSpace = config.Namespace
	}

	cf := &configFile{}
	err = yaml.Unmarshal([]byte(config.Data["config"]), cf)
	if err != nil {
		return nil, err
	}

	return cf, nil
}

func resourcesFor(cf *configFile) (config.ClusterResources, error) {
	var r config.ClusterResources
	var err error

	r.BFDProfiles = bfdProfileFor(cf)
	r.Communities = communitiesFor(cf)
	r.Peers, err = peersFor(cf)
	if err != nil {
		return config.ClusterResources{}, err
	}

	r.Pools, err = ipAddressPoolsFor(cf)
	if err != nil {
		return config.ClusterResources{}, err
	}
	r.BGPAdvs = bgpAdvertisementsFor(cf)
	r.L2Advs = l2AdvertisementsFor(cf)

	return r, nil
}

func bfdProfileFor(c *configFile) []v1beta1.BFDProfile {
	ret := make([]v1beta1.BFDProfile, len(c.BFDProfiles))

	for i, bfd := range c.BFDProfiles {
		b := v1beta1.BFDProfile{
			ObjectMeta: metav1.ObjectMeta{
				Name:      bfd.Name,
				Namespace: resourcesNameSpace,
			},
			Spec: v1beta1.BFDProfileSpec{
				ReceiveInterval:  bfd.ReceiveInterval,
				TransmitInterval: bfd.TransmitInterval,
				DetectMultiplier: bfd.DetectMultiplier,
				EchoInterval:     bfd.EchoInterval,
				EchoMode:         &bfd.EchoMode,
				PassiveMode:      &bfd.PassiveMode,
				MinimumTTL:       bfd.MinimumTTL,
			},
		}
		ret[i] = b
	}
	return ret
}

// communitiesFor aggregates all the community aliases into one community resource.
func communitiesFor(cf *configFile) []v1beta1.Community {
	if len(cf.BGPCommunities) == 0 {
		return nil
	}

	communitiesAliases := make([]v1beta1.CommunityAlias, 0)

	for n, v := range cf.BGPCommunities {
		communityAlias := v1beta1.CommunityAlias{
			Name:  n,
			Value: v,
		}
		communitiesAliases = append(communitiesAliases, communityAlias)
	}

	res := v1beta1.Community{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "communities",
			Namespace: resourcesNameSpace,
		},
		Spec: v1beta1.CommunitySpec{
			Communities: communitiesAliases,
		},
	}
	return []v1beta1.Community{res}
}

func peersFor(c *configFile) ([]v1beta2.BGPPeer, error) {
	res := make([]v1beta2.BGPPeer, 0)
	for i, peer := range c.Peers {
		p, err := parsePeer(peer)
		if err != nil {
			return nil, err
		}
		p.Name = fmt.Sprintf("peer%d", i+1)
		p.Namespace = resourcesNameSpace
		res = append(res, *p)
	}
	return res, nil
}

func parsePeer(p peer) (*v1beta2.BGPPeer, error) {
	holdTime, err := parseHoldTime(p.HoldTime)
	if err != nil {
		return nil, err
	}

	nodeSels := make([]metav1.LabelSelector, 0)
	for _, sel := range p.NodeSelectors {
		s := parseNodeSelector(sel)
		nodeSels = append(nodeSels, s)
	}

	res := &v1beta2.BGPPeer{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: resourcesNameSpace,
		},
		Spec: v1beta2.BGPPeerSpec{
			MyASN:         p.MyASN,
			ASN:           p.ASN,
			Address:       p.Addr,
			SrcAddress:    p.SrcAddr,
			Port:          p.Port,
			HoldTime:      metav1.Duration{Duration: holdTime},
			RouterID:      p.RouterID,
			NodeSelectors: nodeSels,
			Password:      p.Password,
			BFDProfile:    p.BFDProfile,
			EBGPMultiHop:  p.EBGPMultiHop,
		},
	}
	if p.KeepaliveTime != "" {
		keepaliveTime, err := parseKeepaliveTime(holdTime, p.KeepaliveTime)
		if err != nil {
			return nil, err
		}
		res.Spec.KeepaliveTime = metav1.Duration{Duration: keepaliveTime}
	}

	return res, nil
}

func parseNodeSelector(sel nodeSelector) metav1.LabelSelector {
	res := metav1.LabelSelector{}

	res.MatchLabels = sel.MatchLabels
	res.MatchExpressions = []metav1.LabelSelectorRequirement{}

	for _, m := range sel.MatchExpressions {
		matchExp := metav1.LabelSelectorRequirement{
			Key:      m.Key,
			Operator: metav1.LabelSelectorOperator(m.Operator),
			Values:   m.Values,
		}
		matchExp.Values = make([]string, len(m.Values))
		copy(matchExp.Values, m.Values)
		res.MatchExpressions = append(res.MatchExpressions, matchExp)
	}
	return res
}

func parseHoldTime(ht string) (time.Duration, error) {
	if ht == "" {
		return 90 * time.Second, nil
	}
	d, err := time.ParseDuration(ht)
	if err != nil {
		return 0, fmt.Errorf("invalid hold time %q: %s", ht, err)
	}
	rounded := time.Duration(int(d.Seconds())) * time.Second
	if rounded != 0 && rounded < 3*time.Second {
		return 0, fmt.Errorf("invalid hold time %q: must be 0 or >=3s", ht)
	}
	return rounded, nil
}

func parseKeepaliveTime(ht time.Duration, ka string) (time.Duration, error) {
	d, err := time.ParseDuration(ka)
	if err != nil {
		return 0, fmt.Errorf("invalid keepalive time %q: %s", ka, err)
	}
	rounded := time.Duration(int(d.Seconds())) * time.Second
	return rounded, nil
}

func ipAddressPoolsFor(c *configFile) ([]v1beta1.IPAddressPool, error) {
	res := make([]v1beta1.IPAddressPool, len(c.Pools))
	for i, addresspool := range c.Pools {
		var ap v1beta1.IPAddressPool
		ap.Name = addresspool.Name
		ap.Namespace = resourcesNameSpace
		ap.Spec.Addresses = make([]string, 0)
		for _, address := range addresspool.Addresses {
			a, err := configRangeToPoolAddresses(address, addresspool.AvoidBuggyIPs)
			if err != nil {
				return nil, err
			}
			ap.Spec.Addresses = append(ap.Spec.Addresses, a)
		}
		ap.Spec.AutoAssign = addresspool.AutoAssign
		res[i] = ap
	}
	return res, nil
}

func configRangeToPoolAddresses(addresses string, avoidBuggyIPs *bool) (string, error) {
	if avoidBuggyIPs == nil {
		return addresses, nil
	}
	if !*avoidBuggyIPs {
		return addresses, nil
	}
	if strings.Contains(addresses, "-") {
		return addresses, nil
	}
	_, cidr, err := net.ParseCIDR(addresses)
	if err != nil {
		return "", err
	}
	c := ipaddr.NewCursor([]ipaddr.Prefix{*ipaddr.NewPrefix(cidr)})
	second := c.Next()
	if second == nil {
		return "", fmt.Errorf("not enough addresses")
	}
	err = c.Set(c.Last())
	if err != nil {
		return "", err
	}
	secondToLast := c.Prev()
	if secondToLast == nil {
		return "", fmt.Errorf("not enough addresses")
	}
	return fmt.Sprintf("%s-%s", second.IP.String(), secondToLast.IP.String()), nil
}

func bgpAdvertisementsFor(c *configFile) []v1beta1.BGPAdvertisement {
	res := make([]v1beta1.BGPAdvertisement, 0)
	index := 1
	for _, ap := range c.Pools {
		for _, bgpAdv := range ap.BGPAdvertisements {
			var b v1beta1.BGPAdvertisement
			b.Name = fmt.Sprintf("bgpadvertisement%d", index)
			index = index + 1
			b.Namespace = resourcesNameSpace
			b.Spec.Communities = make([]string, len(bgpAdv.Communities))
			copy(b.Spec.Communities, bgpAdv.Communities)
			b.Spec.AggregationLength = bgpAdv.AggregationLength
			b.Spec.AggregationLengthV6 = bgpAdv.AggregationLengthV6
			b.Spec.LocalPref = bgpAdv.LocalPref
			b.Spec.IPAddressPools = []string{ap.Name}
			res = append(res, b)
		}
		if len(ap.BGPAdvertisements) == 0 && ap.Protocol == BGP {
			res = append(res, emptyBGPAdv(ap.Name, index))
			index = index + 1
		}
	}
	return res
}

func emptyBGPAdv(addressPoolName string, index int) v1beta1.BGPAdvertisement {
	return v1beta1.BGPAdvertisement{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("bgpadvertisement%d", index),
			Namespace: resourcesNameSpace,
		},
		Spec: v1beta1.BGPAdvertisementSpec{
			IPAddressPools: []string{addressPoolName},
		},
	}
}

func l2AdvertisementsFor(c *configFile) []v1beta1.L2Advertisement {
	res := make([]v1beta1.L2Advertisement, 0)
	index := 1
	for _, addresspool := range c.Pools {
		if addresspool.Protocol == Layer2 {
			l2Adv := v1beta1.L2Advertisement{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("l2advertisement%d", index),
					Namespace: resourcesNameSpace,
				},
				Spec: v1beta1.L2AdvertisementSpec{
					IPAddressPools: []string{addresspool.Name},
				},
			}
			index = index + 1
			res = append(res, l2Adv)
		}
	}
	return res
}

func createResourcesYAMLs(w io.Writer, resources config.ClusterResources) error {
	objects := resourcesToObjects(resources)
	schema, err := initSchema()
	if err != nil {
		return err
	}

	serializer := json.NewSerializerWithOptions(
		json.DefaultMetaFactory, nil, nil,
		json.SerializerOptions{
			Yaml:   true,
			Pretty: true,
			Strict: true,
		},
	)

	for _, object := range objects {
		gvks, _, err := schema.ObjectKinds(object)
		if err != nil {
			return err
		}
		objectKind := object.GetObjectKind()
		objectKind.SetGroupVersionKind(gvks[0])

		err = serializer.Encode(object, w)
		if err != nil {
			return err
		}
		_, err = w.Write([]byte(separator))
		if err != nil {
			return err
		}
	}

	return nil
}

func resourcesToObjects(resources config.ClusterResources) []runtime.Object {
	objects := make([]runtime.Object, 0)
	for _, peer := range resources.Peers {
		objects = append(objects, peer.DeepCopy())
	}
	for _, p := range resources.Pools {
		objects = append(objects, p.DeepCopy())
	}
	for _, bgpAdv := range resources.BGPAdvs {
		objects = append(objects, bgpAdv.DeepCopy())
	}
	for _, l2Adv := range resources.L2Advs {
		objects = append(objects, l2Adv.DeepCopy())
	}
	for _, b := range resources.BFDProfiles {
		objects = append(objects, b.DeepCopy())
	}
	for _, c := range resources.Communities {
		objects = append(objects, c.DeepCopy())
	}
	return objects
}

func initSchema() (*runtime.Scheme, error) {
	s := runtime.NewScheme()
	err := v1beta1.AddToScheme(s)
	if err != nil {
		return nil, err
	}
	err = v1beta2.AddToScheme(s)
	if err != nil {
		return nil, err
	}
	err = corev1.AddToScheme(s)
	if err != nil {
		return nil, err
	}
	return s, nil
}
