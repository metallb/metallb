// SPDX-License-Identifier:Apache-2.0

package frr

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-kit/log"
	frrv1beta1 "github.com/metallb/frr-k8s/api/v1beta1"
	"go.universe.tf/metallb/internal/bgp"
	"go.universe.tf/metallb/internal/bgp/community"
	"go.universe.tf/metallb/internal/logging"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/utils/ptr"
)

const testData = "testdata/"
const testNodeName = "testnodename"
const testNamespace = "testnamespace"

var classCMask = net.IPv4Mask(0xff, 0xff, 0xff, 0)

var update = flag.Bool("update", false, "update .golden files")

func testCompareFiles(t *testing.T, configFile, goldenFile string) {
	var lastError error

	// Try comparing files multiple times because tests can generate more than one configuration
	err := wait.PollUntilContextTimeout(context.TODO(), 10*time.Millisecond, 2*time.Second, true, func(ctx context.Context) (bool, error) {
		lastError = nil
		cmd := exec.Command("diff", configFile, goldenFile)
		output, err := cmd.Output()

		if err != nil {
			lastError = fmt.Errorf("command %s returned error: %s\n%s", cmd.String(), err, output)
			return false, nil
		}

		return true, nil
	})

	// err can only be a ErrWaitTimeout, as the check function always return nil errors.
	// So lastError is always set
	if err != nil {
		t.Fatalf("failed to compare configfiles %s, %s using poll interval\nlast error: %v", configFile, goldenFile, lastError)
	}
}

func testUpdateGoldenFile(t *testing.T, configFile, goldenFile string) {
	t.Log("update golden file")

	// Sleep to be sure the sessionManager has produced all configuration the test
	// has triggered and no config is still waiting in the debouncer() local variables.
	// No other conditions can be checked, so sleeping is our best option.
	time.Sleep(100 * time.Millisecond)

	cmd := exec.Command("cp", "-a", configFile, goldenFile)
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("command %s returned %s and error: %s", cmd.String(), output, err)
	}
}

func testGenerateFileNames(t *testing.T) (string, string) {
	return filepath.Join(testData, filepath.FromSlash(t.Name())), filepath.Join(testData, filepath.FromSlash(t.Name())+".golden")
}

func testCheckConfigFile(t *testing.T) {
	validateGoldenFile(t)
}

func validateGoldenFile(t *testing.T) {
	configFile, goldenFile := testGenerateFileNames(t)

	if *update {
		testUpdateGoldenFile(t, configFile, goldenFile)
	}

	testCompareFiles(t, configFile, goldenFile)
}

func newTestSessionManager(t *testing.T) bgp.SessionManager {
	l := log.NewNopLogger()
	sessionManager := NewSessionManager(l, logging.LevelDebug, testNodeName, testNamespace)
	configFile, _ := testGenerateFileNames(t)
	sessionManager.SetEventCallback(func(config interface{}) {
		frrConfig, ok := config.(frrv1beta1.FRRConfiguration)
		if !ok {
			t.Fatal("passed value is not an frr configuration")
		}
		f, err := os.Create(configFile)
		if err != nil {
			t.Fatalf("failed to create the file %s", configFile)
		}
		defer f.Close()
		toDump, err := json.MarshalIndent(frrConfig, "", "    ")
		if err != nil {
			t.Fatalf("failed to marshal %s", frrConfig.Name)
		}
		_, err = f.Write(toDump)
		if err != nil {
			t.Fatalf("failed to write %s", frrConfig.Name)
		}
	})
	return sessionManager
}
func TestSingleEBGPSessionGracefullRestart(t *testing.T) {
	sessionManager := newTestSessionManager(t)
	l := log.NewNopLogger()

	session, err := sessionManager.NewSession(l,
		bgp.SessionParameters{
			PeerAddress:     "10.2.2.254:179",
			SourceAddress:   net.ParseIP("10.1.1.254"),
			MyASN:           100,
			RouterID:        net.ParseIP("10.1.1.254"),
			PeerASN:         200,
			HoldTime:        ptr.To(time.Second),
			KeepAliveTime:   ptr.To(time.Second),
			Password:        "password",
			CurrentNode:     "hostname",
			GracefulRestart: true,
			SessionName:     "test-peer"})
	if err != nil {
		t.Fatalf("Could not create session: %s", err)
	}
	defer session.Close()

	testCheckConfigFile(t)
}

func TestSingleEBGPSessionMultiHop(t *testing.T) {
	sessionManager := newTestSessionManager(t)
	l := log.NewNopLogger()

	session, err := sessionManager.NewSession(l,
		bgp.SessionParameters{
			PeerAddress:   "10.2.2.254:179",
			SourceAddress: net.ParseIP("10.1.1.254"),
			MyASN:         100,
			RouterID:      net.ParseIP("10.1.1.254"),
			PeerASN:       200,
			HoldTime:      ptr.To(time.Second),
			KeepAliveTime: ptr.To(time.Second),
			Password:      "password",
			CurrentNode:   "hostname",
			EBGPMultiHop:  true,
			SessionName:   "test-peer"})
	if err != nil {
		t.Fatalf("Could not create session: %s", err)
	}
	defer session.Close()

	testCheckConfigFile(t)
}

func TestSingleEBGPSessionOneHop(t *testing.T) {
	l := log.NewNopLogger()
	sessionManager := newTestSessionManager(t)
	session, err := sessionManager.NewSession(l,
		bgp.SessionParameters{
			PeerAddress:   "127.0.0.2:179",
			SourceAddress: net.ParseIP("10.1.1.254"),
			MyASN:         100,
			RouterID:      net.ParseIP("10.1.1.254"),
			PeerASN:       200,
			HoldTime:      ptr.To(time.Second),
			KeepAliveTime: ptr.To(time.Second),
			Password:      "password",
			CurrentNode:   "hostname",
			EBGPMultiHop:  false,
			SessionName:   "test-peer"})

	if err != nil {
		t.Fatalf("Could not create session: %s", err)
	}
	defer session.Close()

	testCheckConfigFile(t)
}

func TestSingleIPv6EBGPSessionOneHop(t *testing.T) {
	l := log.NewNopLogger()
	sessionManager := newTestSessionManager(t)
	session, err := sessionManager.NewSession(l,
		bgp.SessionParameters{
			PeerAddress:   "[127:0:0::2]:179",
			SourceAddress: net.ParseIP("10:1:1::254"),
			MyASN:         100,
			RouterID:      net.ParseIP("10.1.1.254"),
			PeerASN:       200,
			HoldTime:      ptr.To(time.Second),
			KeepAliveTime: ptr.To(time.Second),
			Password:      "password",
			CurrentNode:   "hostname",
			EBGPMultiHop:  false,
			SessionName:   "test-peer"})

	if err != nil {
		t.Fatalf("Could not create session: %s", err)
	}
	defer session.Close()

	testCheckConfigFile(t)
}

func TestSingleIBGPSession(t *testing.T) {
	l := log.NewNopLogger()
	sessionManager := newTestSessionManager(t)
	session, err := sessionManager.NewSession(l,
		bgp.SessionParameters{
			PeerAddress:   "10.2.2.254:179",
			SourceAddress: net.ParseIP("10.1.1.254"),
			MyASN:         100,
			RouterID:      net.ParseIP("10.1.1.254"),
			PeerASN:       100,
			HoldTime:      ptr.To(time.Second),
			KeepAliveTime: ptr.To(time.Second),
			Password:      "password",
			CurrentNode:   "hostname",
			EBGPMultiHop:  false,
			SessionName:   "test-peer"})

	if err != nil {
		t.Fatalf("Could not create session: %s", err)
	}
	defer session.Close()

	testCheckConfigFile(t)
}

func TestSingleIPv6IBGPSession(t *testing.T) {
	l := log.NewNopLogger()
	sessionManager := newTestSessionManager(t)
	session, err := sessionManager.NewSession(l,
		bgp.SessionParameters{
			PeerAddress:   "[10:2:2::254]:179",
			SourceAddress: net.ParseIP("10:1:1::254"),
			MyASN:         100,
			RouterID:      net.ParseIP("10.1.1.254"),
			PeerASN:       100,
			HoldTime:      ptr.To(time.Second),
			KeepAliveTime: ptr.To(time.Second),
			Password:      "password",
			CurrentNode:   "hostname",
			EBGPMultiHop:  false,
			SessionName:   "test-peer"})
	if err != nil {
		t.Fatalf("Could not create session: %s", err)
	}
	defer session.Close()

	testCheckConfigFile(t)
}

func TestSingleSessionClose(t *testing.T) {
	l := log.NewNopLogger()
	sessionManager := newTestSessionManager(t)

	session, err := sessionManager.NewSession(l,
		bgp.SessionParameters{
			PeerAddress:   "10.2.2.254:179",
			SourceAddress: net.ParseIP("10.1.1.254"),
			MyASN:         100,
			RouterID:      net.ParseIP("10.1.1.254"),
			PeerASN:       200,
			HoldTime:      ptr.To(time.Second),
			KeepAliveTime: ptr.To(time.Second),
			Password:      "password",
			CurrentNode:   "hostname",
			EBGPMultiHop:  true,
			SessionName:   "test-peer"})

	if err != nil {
		t.Fatalf("Could not create session: %s", err)
	}

	session.Close()
	testCheckConfigFile(t)
}
func TestTwoSessions(t *testing.T) {
	l := log.NewNopLogger()
	sessionManager := newTestSessionManager(t)
	session1, err := sessionManager.NewSession(l,
		bgp.SessionParameters{
			PeerAddress:   "10.2.2.254:179",
			SourceAddress: net.ParseIP("10.1.1.254"),
			MyASN:         100,
			RouterID:      net.ParseIP("10.1.1.254"),
			PeerASN:       200,
			HoldTime:      ptr.To(time.Second),
			KeepAliveTime: ptr.To(time.Second),
			Password:      "password",
			CurrentNode:   "hostname",
			EBGPMultiHop:  true,
			SessionName:   "test-peer1"})

	if err != nil {
		t.Fatalf("Could not create session: %s", err)
	}
	defer session1.Close()
	session2, err := sessionManager.NewSession(l,
		bgp.SessionParameters{
			PeerAddress:   "10.4.4.255:179",
			SourceAddress: net.ParseIP("10.3.3.254"),
			MyASN:         300,
			RouterID:      net.ParseIP("10.3.3.254"),
			PeerASN:       400,
			HoldTime:      ptr.To(time.Second),
			KeepAliveTime: ptr.To(time.Second),
			PasswordRef: corev1.SecretReference{
				Name:      "test-secret",
				Namespace: testNamespace,
			},
			CurrentNode:  "hostname",
			EBGPMultiHop: true,
			SessionName:  "test-peer2"})

	if err != nil {
		t.Fatalf("Could not create session: %s", err)
	}
	defer session2.Close()

	testCheckConfigFile(t)
}

func TestTwoIPv6Sessions(t *testing.T) {
	l := log.NewNopLogger()
	sessionManager := newTestSessionManager(t)

	session1, err := sessionManager.NewSession(l,
		bgp.SessionParameters{
			PeerAddress:   "[10:2:2::254]:179",
			SourceAddress: net.ParseIP("10:1:1::254"),
			MyASN:         100,
			RouterID:      net.ParseIP("10.1.1.254"),
			PeerASN:       200,
			HoldTime:      ptr.To(time.Second),
			KeepAliveTime: ptr.To(time.Second),
			Password:      "password",
			CurrentNode:   "hostname",
			EBGPMultiHop:  false,
			SessionName:   "test-peer1"})
	if err != nil {
		t.Fatalf("Could not create session: %s", err)
	}
	defer session1.Close()
	session2, err := sessionManager.NewSession(l,
		bgp.SessionParameters{
			PeerAddress:   "[10:4:4::255]:179",
			SourceAddress: net.ParseIP("10:3:3::254"),
			MyASN:         300,
			RouterID:      net.ParseIP("10.3.3.254"),
			PeerASN:       400,
			HoldTime:      ptr.To(time.Second),
			KeepAliveTime: ptr.To(time.Second),
			Password:      "password",
			CurrentNode:   "hostname",
			EBGPMultiHop:  false,
			SessionName:   "test-peer2"})

	if err != nil {
		t.Fatalf("Could not create session: %s", err)
	}
	defer session2.Close()
	testCheckConfigFile(t)
}

func TestTwoSessionsDuplicate(t *testing.T) {
	l := log.NewNopLogger()
	sessionManager := newTestSessionManager(t)
	session1, err := sessionManager.NewSession(l,
		bgp.SessionParameters{
			PeerAddress:   "10.2.2.254:179",
			SourceAddress: net.ParseIP("10.1.1.254"),
			MyASN:         100,
			RouterID:      net.ParseIP("10.1.1.254"),
			PeerASN:       200,
			HoldTime:      ptr.To(time.Second),
			KeepAliveTime: ptr.To(time.Second),
			Password:      "password",
			CurrentNode:   "hostname",
			EBGPMultiHop:  true,
			SessionName:   "test-peer1"})

	if err != nil {
		t.Fatalf("Could not create session: %s", err)
	}
	defer session1.Close()
	session2, err := sessionManager.NewSession(l,
		bgp.SessionParameters{
			PeerAddress:   "10.2.2.254:179",
			SourceAddress: net.ParseIP("10.1.1.254"),
			MyASN:         100,
			RouterID:      net.ParseIP("10.1.1.254"),
			PeerASN:       200,
			HoldTime:      ptr.To(time.Second),
			KeepAliveTime: ptr.To(time.Second),
			Password:      "password",
			CurrentNode:   "hostname",
			EBGPMultiHop:  true,
			SessionName:   "test-peer2"})
	if err != nil {
		t.Fatalf("Could not create session: %s", err)
	}
	defer session2.Close()

	testCheckConfigFile(t)
}

func TestTwoSessionsDuplicateRouter(t *testing.T) {
	l := log.NewNopLogger()
	sessionManager := newTestSessionManager(t)

	session1, err := sessionManager.NewSession(l,
		bgp.SessionParameters{
			PeerAddress:   "10.2.2.254:179",
			SourceAddress: net.ParseIP("10.1.1.254"),
			MyASN:         100,
			RouterID:      net.ParseIP("10.1.1.254"),
			PeerASN:       200,
			HoldTime:      ptr.To(time.Second),
			KeepAliveTime: ptr.To(time.Second),
			Password:      "password",
			CurrentNode:   "hostname",
			EBGPMultiHop:  true,
			SessionName:   "test-peer1"})
	if err != nil {
		t.Fatalf("Could not create session: %s", err)
	}
	defer session1.Close()
	session2, err := sessionManager.NewSession(l,
		bgp.SessionParameters{
			PeerAddress:   "10.4.4.255:179",
			SourceAddress: net.ParseIP("10.1.1.254"),
			MyASN:         100,
			RouterID:      net.ParseIP("10.1.1.254"),
			PeerASN:       400,
			HoldTime:      ptr.To(time.Second),
			KeepAliveTime: ptr.To(time.Second),
			Password:      "password",
			CurrentNode:   "hostname",
			EBGPMultiHop:  true,
			SessionName:   "test-peer2"})

	if err != nil {
		t.Fatalf("Could not create session: %s", err)
	}
	defer session2.Close()

	testCheckConfigFile(t)
}

func TestSingleAdvertisement(t *testing.T) {
	l := log.NewNopLogger()
	sessionManager := newTestSessionManager(t)
	session, err := sessionManager.NewSession(l,
		bgp.SessionParameters{
			PeerAddress:   "10.2.2.254:179",
			SourceAddress: net.ParseIP("10.1.1.254"),
			MyASN:         100,
			RouterID:      net.ParseIP("10.1.1.254"),
			PeerASN:       200,
			HoldTime:      ptr.To(time.Second),
			KeepAliveTime: ptr.To(time.Second),
			Password:      "password",
			CurrentNode:   "hostname",
			EBGPMultiHop:  true,
			SessionName:   "test-peer"})
	if err != nil {
		t.Fatalf("Could not create session: %s", err)
	}
	defer session.Close()

	prefix := &net.IPNet{
		IP:   net.ParseIP("172.16.1.10"),
		Mask: classCMask,
	}
	communities := []community.BGPCommunity{}
	community1, _ := community.New("1111:2222")
	communities = append(communities, community1)
	community2, _ := community.New("3333:4444")
	communities = append(communities, community2)
	adv := &bgp.Advertisement{
		Prefix:      prefix,
		Communities: communities,
		LocalPref:   300,
	}

	err = session.Set(adv)
	if err != nil {
		t.Fatalf("Could not advertise prefix: %s", err)
	}

	testCheckConfigFile(t)
}

func TestTwoAdvertisementsWithSamePrefix(t *testing.T) {
	l := log.NewNopLogger()
	sessionManager := newTestSessionManager(t)
	session, err := sessionManager.NewSession(l,
		bgp.SessionParameters{
			PeerAddress:   "10.2.2.254:179",
			SourceAddress: net.ParseIP("10.1.1.254"),
			MyASN:         100,
			RouterID:      net.ParseIP("10.1.1.254"),
			PeerASN:       200,
			HoldTime:      ptr.To(time.Second),
			KeepAliveTime: ptr.To(time.Second),
			Password:      "password",
			CurrentNode:   "hostname",
			EBGPMultiHop:  true,
			SessionName:   "test-peer"})
	if err != nil {
		t.Fatalf("Could not create session: %s", err)
	}
	defer session.Close()

	prefix := &net.IPNet{
		IP:   net.ParseIP("172.16.1.10"),
		Mask: classCMask,
	}
	communities := []community.BGPCommunity{}
	community1, _ := community.New("1111:2222")
	communities = append(communities, community1)
	community2, _ := community.New("3333:4444")
	communities = append(communities, community2)
	adv := &bgp.Advertisement{
		Prefix:      prefix,
		Communities: communities,
		LocalPref:   300,
		Peers:       []string{"peer1"},
	}
	adv2 := &bgp.Advertisement{
		Prefix:      prefix,
		Communities: communities,
		LocalPref:   300,
	}
	advs := []*bgp.Advertisement{adv, adv2}
	rand.Shuffle(len(advs), func(i, j int) {
		advs[i], advs[j] = advs[j], advs[i]
	})
	err = session.Set(advs[0], advs[1])
	if err != nil {
		t.Fatalf("Could not advertise prefix: %s", err)
	}

	testCheckConfigFile(t)
}

func TestSingleAdvertisementNoRouterID(t *testing.T) {
	l := log.NewNopLogger()
	sessionManager := newTestSessionManager(t)
	session, err := sessionManager.NewSession(l,
		bgp.SessionParameters{
			PeerAddress:   "10.2.2.254:179",
			SourceAddress: net.ParseIP("10.1.1.254"),
			MyASN:         100,
			RouterID:      nil,
			PeerASN:       200,
			HoldTime:      ptr.To(time.Second),
			KeepAliveTime: ptr.To(time.Second),
			Password:      "password",
			CurrentNode:   "hostname",
			EBGPMultiHop:  true,
			SessionName:   "test-peer"})

	if err != nil {
		t.Fatalf("Could not create session: %s", err)
	}
	defer session.Close()

	prefix := &net.IPNet{
		IP:   net.ParseIP("172.16.1.10"),
		Mask: classCMask,
	}

	adv := &bgp.Advertisement{
		Prefix: prefix,
	}

	err = session.Set(adv)
	if err != nil {
		t.Fatalf("Could not advertise prefix: %s", err)
	}

	testCheckConfigFile(t)
}

func TestSingleAdvertisementInvalidNoPort(t *testing.T) {
	sessionManager := newTestSessionManager(t)

	l := log.NewNopLogger()
	session, err := sessionManager.NewSession(l,
		bgp.SessionParameters{
			PeerAddress:   "10.2.2.254",
			SourceAddress: net.ParseIP("10.1.1.254"),
			MyASN:         100,
			RouterID:      net.ParseIP("10.1.1.254"),
			PeerASN:       200,
			HoldTime:      ptr.To(time.Second),
			KeepAliveTime: ptr.To(time.Second),
			Password:      "password",
			CurrentNode:   "hostname",
			EBGPMultiHop:  true,
			SessionName:   "test-peer"})

	if err == nil {
		session.Close()
		t.Fatalf("Should not be able to create session")
	}

	// Not checking the file since this test won't create it
}

func TestSingleAdvertisementStop(t *testing.T) {
	l := log.NewNopLogger()
	sessionManager := newTestSessionManager(t)

	session, err := sessionManager.NewSession(l,
		bgp.SessionParameters{
			PeerAddress:   "10.2.2.254:179",
			SourceAddress: net.ParseIP("10.1.1.254"),
			MyASN:         100,
			RouterID:      net.ParseIP("10.1.1.254"),
			PeerASN:       200,
			HoldTime:      ptr.To(time.Second),
			KeepAliveTime: ptr.To(time.Second),
			Password:      "password",
			CurrentNode:   "hostname",
			EBGPMultiHop:  true,
			SessionName:   "test-peer"})

	if err != nil {
		t.Fatalf("Could not create session: %s", err)
	}
	defer session.Close()

	prefix := &net.IPNet{
		IP:   net.ParseIP("172.16.1.10"),
		Mask: classCMask,
	}

	adv := &bgp.Advertisement{
		Prefix: prefix,
	}

	err = session.Set(adv)
	if err != nil {
		t.Fatalf("Could not advertise prefix: %s", err)
	}

	err = session.Set()
	if err != nil {
		t.Fatalf("Could not advertise prefix: %s", err)
	}

	testCheckConfigFile(t)
}

func TestSingleAdvertisementChange(t *testing.T) {
	l := log.NewNopLogger()
	sessionManager := newTestSessionManager(t)

	session, err := sessionManager.NewSession(l,
		bgp.SessionParameters{
			PeerAddress:   "10.2.2.254:179",
			SourceAddress: net.ParseIP("10.1.1.254"),
			MyASN:         100,
			RouterID:      net.ParseIP("10.1.1.254"),
			PeerASN:       200,
			HoldTime:      ptr.To(time.Second),
			KeepAliveTime: ptr.To(time.Second),
			Password:      "password",
			CurrentNode:   "hostname",
			EBGPMultiHop:  true,
			SessionName:   "test-peer"})
	if err != nil {
		t.Fatalf("Could not create session: %s", err)
	}
	defer session.Close()

	prefix := &net.IPNet{
		IP:   net.ParseIP("172.16.1.10"),
		Mask: classCMask,
	}

	adv := &bgp.Advertisement{
		Prefix: prefix,
	}

	err = session.Set(adv)
	if err != nil {
		t.Fatalf("Could not advertise prefix: %s", err)
	}

	prefix = &net.IPNet{
		IP:   net.ParseIP("172.16.1.11"),
		Mask: classCMask,
	}

	adv = &bgp.Advertisement{
		Prefix: prefix,
	}

	err = session.Set(adv)
	if err != nil {
		t.Fatalf("Could not advertise prefix: %s", err)
	}

	testCheckConfigFile(t)
}

func TestTwoAdvertisements(t *testing.T) {
	l := log.NewNopLogger()
	sessionManager := newTestSessionManager(t)

	session, err := sessionManager.NewSession(l,
		bgp.SessionParameters{
			PeerAddress:   "10.2.2.254:179",
			SourceAddress: net.ParseIP("10.1.1.254"),
			MyASN:         100,
			RouterID:      net.ParseIP("10.1.1.254"),
			PeerASN:       200,
			HoldTime:      ptr.To(time.Second),
			KeepAliveTime: ptr.To(time.Second),
			Password:      "password",
			CurrentNode:   "hostname",
			EBGPMultiHop:  true,
			SessionName:   "test-peer"})
	if err != nil {
		t.Fatalf("Could not create session: %s", err)
	}
	defer session.Close()

	prefix1 := &net.IPNet{
		IP:   net.ParseIP("172.16.1.10"),
		Mask: classCMask,
	}
	communities := []community.BGPCommunity{}
	community, _ := community.New("1111:2222")
	communities = append(communities, community)
	adv1 := &bgp.Advertisement{
		Prefix:      prefix1,
		Communities: communities,
	}

	prefix2 := &net.IPNet{
		IP:   net.ParseIP("172.16.1.11"),
		Mask: classCMask,
	}

	adv2 := &bgp.Advertisement{
		Prefix: prefix2,
	}

	err = session.Set(adv1, adv2)
	if err != nil {
		t.Fatalf("Could not advertise prefix: %s", err)
	}

	testCheckConfigFile(t)
}

func TestTwoAdvertisementsTwoSessions(t *testing.T) {
	l := log.NewNopLogger()
	sessionManager := newTestSessionManager(t)

	for i := 0; i < 100; i++ {
		func() {
			sessionsParameters := []bgp.SessionParameters{
				{
					PeerAddress:   "10.2.2.254:179",
					SourceAddress: net.ParseIP("10.1.1.254"),
					MyASN:         100,
					RouterID:      net.ParseIP("10.1.1.254"),
					PeerASN:       200,
					HoldTime:      ptr.To(time.Second),
					KeepAliveTime: ptr.To(time.Second),
					Password:      "password",
					CurrentNode:   "hostname",
					EBGPMultiHop:  true,
					SessionName:   "test-peer"},
				{
					PeerAddress:   "10.2.2.255:179",
					SourceAddress: net.ParseIP("10.1.1.254"),
					MyASN:         100,
					RouterID:      net.ParseIP("10.1.1.254"),
					PeerASN:       200,
					HoldTime:      ptr.To(time.Second),
					KeepAliveTime: ptr.To(time.Second),
					Password:      "password",
					CurrentNode:   "hostname",
					EBGPMultiHop:  true,
					SessionName:   "test-peer1"},
			}
			seed := time.Now().UnixNano()
			rand.New(rand.NewSource(seed))

			rand.Shuffle(len(sessionsParameters), func(i, j int) {
				sessionsParameters[i], sessionsParameters[j] = sessionsParameters[j], sessionsParameters[i]
			})

			session, err := sessionManager.NewSession(l, sessionsParameters[0])
			if err != nil {
				t.Fatalf("Could not create session: %s", err)
			}
			defer session.Close()

			session1, err := sessionManager.NewSession(l, sessionsParameters[1])
			if err != nil {
				t.Fatalf("Could not create session: %s", err)
			}
			defer session1.Close()

			prefix1 := &net.IPNet{
				IP:   net.ParseIP("172.16.1.10"),
				Mask: classCMask,
			}
			communities := []community.BGPCommunity{}
			community, _ := community.New("1111:2222")
			communities = append(communities, community)
			adv1 := &bgp.Advertisement{
				Prefix:      prefix1,
				Communities: communities,
			}

			prefix2 := &net.IPNet{
				IP:   net.ParseIP("172.16.1.11"),
				Mask: classCMask,
			}

			adv2 := &bgp.Advertisement{
				Prefix:      prefix2,
				Communities: communities,
				LocalPref:   2,
			}
			advs := []*bgp.Advertisement{adv1, adv2}
			rand.Shuffle(len(advs), func(i, j int) {
				advs[i], advs[j] = advs[j], advs[i]
			})
			err = session.Set(advs[0], advs[1])
			if err != nil {
				t.Fatalf("Could not advertise prefix: %s", err)
			}

			rand.Shuffle(len(advs), func(i, j int) {
				advs[i], advs[j] = advs[j], advs[i]
			})
			err = session1.Set(advs[0], advs[1])
			if err != nil {
				t.Fatalf("Could not advertise prefix: %s", err)
			}

			validateGoldenFile(t)
		}()
	}
}

func TestLargeCommunities(t *testing.T) {
	l := log.NewNopLogger()
	sessionManager := newTestSessionManager(t)

	session, err := sessionManager.NewSession(l,
		bgp.SessionParameters{
			PeerAddress:   "10.2.2.254:179",
			SourceAddress: net.ParseIP("10.1.1.254"),
			MyASN:         100,
			RouterID:      net.ParseIP("10.1.1.254"),
			PeerASN:       200,
			HoldTime:      ptr.To(time.Second),
			KeepAliveTime: ptr.To(time.Second),
			Password:      "password",
			CurrentNode:   "hostname",
			EBGPMultiHop:  true,
			SessionName:   "test-peer"})
	if err != nil {
		t.Fatalf("Could not create session: %s", err)
	}
	defer session.Close()

	prefix := &net.IPNet{
		IP:   net.ParseIP("172.16.1.10"),
		Mask: classCMask,
	}
	communities := []community.BGPCommunity{}
	community1, _ := community.New("large:1111:2222:3333")
	communities = append(communities, community1)
	community2, _ := community.New("large:2222:3333:4444")
	communities = append(communities, community2)
	community3, _ := community.New("3333:4444")
	communities = append(communities, community3)
	adv := &bgp.Advertisement{
		Prefix:      prefix,
		Communities: communities,
		LocalPref:   300,
	}

	err = session.Set(adv)
	if err != nil {
		t.Fatalf("Could not advertise prefix: %s", err)
	}

	testCheckConfigFile(t)
}

func TestSingleSessionWithNoTimers(t *testing.T) {
	l := log.NewNopLogger()
	sessionManager := newTestSessionManager(t)

	session, err := sessionManager.NewSession(l,
		bgp.SessionParameters{
			PeerAddress: "10.2.2.254:179",
			MyASN:       102,
			PeerASN:     100,
			SessionName: "test-peer"})

	if err != nil {
		t.Fatalf("Could not create session: %s", err)
	}
	defer session.Close()

	testCheckConfigFile(t)
}

func TestSingleSessionWithZeroTimers(t *testing.T) {
	l := log.NewNopLogger()
	sessionManager := newTestSessionManager(t)

	session, err := sessionManager.NewSession(l,
		bgp.SessionParameters{
			PeerAddress:   "10.2.2.254:179",
			MyASN:         102,
			HoldTime:      ptr.To(0 * time.Second),
			KeepAliveTime: ptr.To(0 * time.Second),
			PeerASN:       100,
			SessionName:   "test-peer"})

	if err != nil {
		t.Fatalf("Could not create session: %s", err)
	}
	defer session.Close()

	testCheckConfigFile(t)
}

func TestSingleSessionWithInternalASN(t *testing.T) {
	sessionManager := newTestSessionManager(t)
	l := log.NewNopLogger()

	session, err := sessionManager.NewSession(l,
		bgp.SessionParameters{
			PeerAddress: "10.2.2.254:179",
			MyASN:       100,
			RouterID:    net.ParseIP("10.1.1.254"),
			DynamicASN:  "internal",
			CurrentNode: "hostname",
			SessionName: "test-peer"})
	if err != nil {
		t.Fatalf("Could not create session: %s", err)
	}
	defer session.Close()

	testCheckConfigFile(t)
}

func TestSingleSessionWithExternalASN(t *testing.T) {
	sessionManager := newTestSessionManager(t)
	l := log.NewNopLogger()

	session, err := sessionManager.NewSession(l,
		bgp.SessionParameters{
			PeerAddress: "10.2.2.254:179",
			MyASN:       100,
			RouterID:    net.ParseIP("10.1.1.254"),
			DynamicASN:  "external",
			CurrentNode: "hostname",
			SessionName: "test-peer"})
	if err != nil {
		t.Fatalf("Could not create session: %s", err)
	}
	defer session.Close()

	testCheckConfigFile(t)
}
