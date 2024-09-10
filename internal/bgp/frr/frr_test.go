// SPDX-License-Identifier:Apache-2.0

package frr

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/go-kit/log"
	"go.universe.tf/metallb/internal/bgp"
	"go.universe.tf/metallb/internal/bgp/community"
	"go.universe.tf/metallb/internal/ipfamily"
	"go.universe.tf/metallb/internal/logging"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/utils/ptr"
)

const testData = "testdata/"

var classCMask = net.IPv4Mask(0xff, 0xff, 0xff, 0)

var update = flag.Bool("update", false, "update .golden files")

func testOsHostname() (string, error) {
	return "dummyhostname", nil
}

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

func testSetup(t *testing.T) {
	configFile, _ := testGenerateFileNames(t)
	os.Setenv("FRR_CONFIG_FILE", configFile)
	_ = os.Remove(configFile) // removing leftovers from previous runs
	osHostname = testOsHostname
}

func testCheckConfigFile(t *testing.T) {
	validateGoldenFile(t)
	validateAgainstFRR(t)
}

func validateGoldenFile(t *testing.T) {
	configFile, goldenFile := testGenerateFileNames(t)

	if *update {
		testUpdateGoldenFile(t, configFile, goldenFile)
	}

	testCompareFiles(t, configFile, goldenFile)
}

func validateAgainstFRR(t *testing.T) {
	configFile, _ := testGenerateFileNames(t)
	if !strings.Contains(configFile, "Invalid") {
		err := testFileIsValid(configFile)
		if err != nil {
			t.Fatalf("Failed to verify the file %s", err)
		}
	}
}

func TestSingleEBGPSessionMultiHop(t *testing.T) {
	testSetup(t)

	l := log.NewNopLogger()
	sessionManager := mockNewSessionManager(l, logging.LevelInfo)
	defer close(sessionManager.reloadConfig)
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
	testSetup(t)

	l := log.NewNopLogger()
	sessionManager := mockNewSessionManager(l, logging.LevelInfo)
	defer close(sessionManager.reloadConfig)
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
	testSetup(t)

	l := log.NewNopLogger()
	sessionManager := mockNewSessionManager(l, logging.LevelInfo)
	defer close(sessionManager.reloadConfig)
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
	testSetup(t)

	l := log.NewNopLogger()
	sessionManager := mockNewSessionManager(l, logging.LevelInfo)
	defer close(sessionManager.reloadConfig)
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
	testSetup(t)

	l := log.NewNopLogger()
	sessionManager := mockNewSessionManager(l, logging.LevelInfo)
	defer close(sessionManager.reloadConfig)
	session, err := sessionManager.NewSession(l,
		bgp.SessionParameters{
			PeerAddress:   "[10:2:2::254]:179",
			SourceAddress: net.ParseIP("10:1:1::254"),
			MyASN:         100,
			RouterID:      net.ParseIP("10.1.1.254"),
			PeerASN:       100,
			HoldTime:      ptr.To(time.Second),
			KeepAliveTime: ptr.To(time.Second),
			ConnectTime:   ptr.To(time.Second),
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
	testSetup(t)

	l := log.NewNopLogger()
	sessionManager := mockNewSessionManager(l, logging.LevelInfo)
	defer close(sessionManager.reloadConfig)

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

func TestSingleSessionWithGracefulRestart(t *testing.T) {
	testSetup(t)

	l := log.NewNopLogger()
	sessionManager := mockNewSessionManager(l, logging.LevelInfo)
	defer close(sessionManager.reloadConfig)
	session, err := sessionManager.NewSession(l,
		bgp.SessionParameters{
			PeerAddress:     "10.2.2.254:179",
			SourceAddress:   net.ParseIP("10.1.1.254"),
			MyASN:           102,
			RouterID:        net.ParseIP("10.1.1.254"),
			PeerASN:         100,
			GracefulRestart: true,
			SessionName:     "test-peer"})

	if err != nil {
		t.Fatalf("Could not create session: %s", err)
	}
	defer session.Close()

	testCheckConfigFile(t)
}

func TestTwoSessions(t *testing.T) {
	testSetup(t)

	l := log.NewNopLogger()
	sessionManager := mockNewSessionManager(l, logging.LevelInfo)
	defer close(sessionManager.reloadConfig)
	session1, err := sessionManager.NewSession(l,
		bgp.SessionParameters{
			PeerAddress:   "10.2.2.254:179",
			SourceAddress: net.ParseIP("10.1.1.254"),
			MyASN:         100,
			RouterID:      net.ParseIP("10.1.1.254"),
			PeerASN:       200,
			HoldTime:      ptr.To(time.Second),
			KeepAliveTime: ptr.To(time.Second),
			ConnectTime:   ptr.To(time.Second),
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
			ConnectTime:   ptr.To(time.Second),
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

func TestTwoIPv6Sessions(t *testing.T) {
	testSetup(t)

	l := log.NewNopLogger()
	sessionManager := mockNewSessionManager(l, logging.LevelInfo)
	defer close(sessionManager.reloadConfig)

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

func TestIPv4AndIPv6SessionsDisableMP(t *testing.T) {
	testSetup(t)

	l := log.NewNopLogger()
	sessionManager := mockNewSessionManager(l, logging.LevelInfo)
	defer close(sessionManager.reloadConfig)

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
			SessionName:   "test-peer1",
			DisableMP:     true},
	)
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
			Password:      "password",
			CurrentNode:   "hostname",
			EBGPMultiHop:  true,
			SessionName:   "test-peer2",
			DisableMP:     true})

	if err != nil {
		t.Fatalf("Could not create session: %s", err)
	}
	defer session2.Close()
	testCheckConfigFile(t)
}

func TestTwoSessionsDuplicate(t *testing.T) {
	testSetup(t)

	l := log.NewNopLogger()
	sessionManager := mockNewSessionManager(l, logging.LevelInfo)
	defer close(sessionManager.reloadConfig)
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
	testSetup(t)

	l := log.NewNopLogger()
	sessionManager := mockNewSessionManager(l, logging.LevelInfo)
	defer close(sessionManager.reloadConfig)

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
	testSetup(t)

	l := log.NewNopLogger()
	sessionManager := mockNewSessionManager(l, logging.LevelInfo)
	defer close(sessionManager.reloadConfig)
	session, err := sessionManager.NewSession(l,
		bgp.SessionParameters{
			PeerAddress:   "10.2.2.254:179",
			SourceAddress: net.ParseIP("10.1.1.254"),
			MyASN:         100,
			RouterID:      net.ParseIP("10.1.1.254"),
			PeerASN:       200,
			HoldTime:      ptr.To(time.Second),
			KeepAliveTime: ptr.To(time.Second),
			ConnectTime:   ptr.To(time.Second),
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

func TestSingleAdvertisementNoRouterID(t *testing.T) {
	testSetup(t)

	l := log.NewNopLogger()
	sessionManager := mockNewSessionManager(l, logging.LevelInfo)
	defer close(sessionManager.reloadConfig)
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
	testSetup(t)

	l := log.NewNopLogger()
	sessionManager := mockNewSessionManager(l, logging.LevelInfo)
	defer close(sessionManager.reloadConfig)
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
	testSetup(t)

	l := log.NewNopLogger()
	sessionManager := mockNewSessionManager(l, logging.LevelInfo)
	defer close(sessionManager.reloadConfig)
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
	testSetup(t)

	l := log.NewNopLogger()
	sessionManager := mockNewSessionManager(l, logging.LevelInfo)
	defer close(sessionManager.reloadConfig)
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
	testSetup(t)

	l := log.NewNopLogger()
	sessionManager := mockNewSessionManager(l, logging.LevelInfo)
	defer close(sessionManager.reloadConfig)
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

func TestTwoAdvertisementsDuplicate(t *testing.T) {
	testSetup(t)

	l := log.NewNopLogger()
	sessionManager := mockNewSessionManager(l, logging.LevelInfo)
	defer close(sessionManager.reloadConfig)
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
	adv1 := &bgp.Advertisement{
		Prefix: prefix1,
	}

	adv2 := &bgp.Advertisement{
		Prefix: prefix1,
	}

	err = session.Set(adv1, adv2)
	if err != nil {
		t.Fatalf("Could not advertise prefix: %s", err)
	}

	testCheckConfigFile(t)
}

func TestTwoAdvertisementsTwoSessions(t *testing.T) {
	testSetup(t)

	l := log.NewNopLogger()
	sessionManager := mockNewSessionManager(l, logging.LevelInfo)
	defer close(sessionManager.reloadConfig)

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
					ConnectTime:   ptr.To(time.Second),
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
					ConnectTime:   ptr.To(time.Second),
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
	validateAgainstFRR(t)
}

func TestSingleSessionExtras(t *testing.T) {
	testSetup(t)

	l := log.NewNopLogger()
	sessionManager := mockNewSessionManager(l, logging.LevelInfo)
	defer close(sessionManager.reloadConfig)
	err := sessionManager.SyncExtraInfo("# hello")
	if err != nil {
		t.Fatalf("Could not sync extra info")
	}
	session, err := sessionManager.NewSession(l,
		bgp.SessionParameters{
			PeerAddress:   "127.0.0.2:179",
			SourceAddress: net.ParseIP("10.1.1.254"),
			MyASN:         100,
			RouterID:      net.ParseIP("10.1.1.254"),
			PeerASN:       200,
			HoldTime:      ptr.To(time.Second),
			KeepAliveTime: ptr.To(time.Second),
			ConnectTime:   ptr.To(time.Second),
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

func TestLoggingConfiguration(t *testing.T) {
	testSetup(t)

	l := log.NewNopLogger()
	sessionManager := mockNewSessionManager(l, logging.LevelWarn)
	defer close(sessionManager.reloadConfig)

	config, err := sessionManager.createConfig()
	if err != nil {
		t.Fatalf("Error while creating configuration: %s", err)
	}

	sessionManager.reloadConfig <- reloadEvent{config: config}
	testCheckConfigFile(t)
}

func TestLoggingConfigurationDebug(t *testing.T) {
	testSetup(t)

	l := log.NewNopLogger()
	sessionManager := mockNewSessionManager(l, logging.LevelDebug)
	defer close(sessionManager.reloadConfig)

	config, err := sessionManager.createConfig()
	if err != nil {
		t.Fatalf("Error while creating configuration: %s", err)
	}

	sessionManager.reloadConfig <- reloadEvent{config: config}
	testCheckConfigFile(t)
}

func TestLoggingConfigurationOverrideByEnvironmentVar(t *testing.T) {
	testSetup(t)

	orig := os.Getenv("FRR_LOGGING_LEVEL")
	os.Setenv("FRR_LOGGING_LEVEL", "alerts")
	t.Cleanup(func() { os.Setenv("FRR_LOGGING_LEVEL", orig) })

	l := log.NewNopLogger()
	sessionManager := mockNewSessionManager(l, logging.LevelDebug)
	defer close(sessionManager.reloadConfig)

	config, err := sessionManager.createConfig()
	if err != nil {
		t.Fatalf("Error while creating configuration: %s", err)
	}

	sessionManager.reloadConfig <- reloadEvent{config: config}
	testCheckConfigFile(t)
}

func TestLargeCommunities(t *testing.T) {
	testSetup(t)

	l := log.NewNopLogger()
	sessionManager := mockNewSessionManager(l, logging.LevelInfo)
	defer close(sessionManager.reloadConfig)
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

func TestManyAdvertisementsSameCommunity(t *testing.T) {
	testSetup(t)

	l := log.NewNopLogger()
	sessionManager := mockNewSessionManager(l, logging.LevelInfo)
	defer close(sessionManager.reloadConfig)
	session, err := sessionManager.NewSession(l,
		bgp.SessionParameters{
			PeerAddress:   "10.2.2.254:179",
			SourceAddress: net.ParseIP("10.1.1.254"),
			MyASN:         100,
			RouterID:      net.ParseIP("10.1.1.254"),
			PeerASN:       200,
			HoldTime:      ptr.To(time.Second),
			KeepAliveTime: ptr.To(time.Second),
			ConnectTime:   ptr.To(time.Second),
			Password:      "password",
			CurrentNode:   "hostname",
			EBGPMultiHop:  true,
			SessionName:   "test-peer"})
	if err != nil {
		t.Fatalf("Could not create session: %s", err)
	}
	defer session.Close()

	community1, _ := community.New("1111:2222")
	communities := []community.BGPCommunity{community1}
	advs := []*bgp.Advertisement{}
	for i := 0; i < 10; i++ {
		prefix := &net.IPNet{
			IP:   net.ParseIP(fmt.Sprintf("172.16.1.%d", i)),
			Mask: classCMask,
		}
		adv := &bgp.Advertisement{
			Prefix:      prefix,
			Communities: communities,
		}
		advs = append(advs, adv)
	}

	err = session.Set(advs...)
	if err != nil {
		t.Fatalf("Could not advertise prefix: %s", err)
	}

	testCheckConfigFile(t)
}

func TestSingleSessionWithNoTimers(t *testing.T) {
	testSetup(t)

	l := log.NewNopLogger()
	sessionManager := mockNewSessionManager(l, logging.LevelInfo)
	defer close(sessionManager.reloadConfig)
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
	testSetup(t)

	l := log.NewNopLogger()
	sessionManager := mockNewSessionManager(l, logging.LevelInfo)
	defer close(sessionManager.reloadConfig)
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

func TestAddToAdvertisements(t *testing.T) {
	tests := []struct {
		name      string
		current   []*advertisementConfig
		toAdd     *advertisementConfig
		expected  []*advertisementConfig
		shouldErr bool
	}{
		{
			name:    "starting empty",
			current: []*advertisementConfig{},
			toAdd: &advertisementConfig{
				Prefix:   "192.168.1.1/32",
				IPFamily: ipfamily.IPv4,
			},
			expected: []*advertisementConfig{
				{
					Prefix:   "192.168.1.1/32",
					IPFamily: ipfamily.IPv4,
				},
			},
		},
		{
			name: "mismatch localpref",
			current: []*advertisementConfig{{
				Prefix:    "192.168.1.1/32",
				IPFamily:  ipfamily.IPv4,
				LocalPref: uint32(12),
			}},
			toAdd: &advertisementConfig{
				Prefix:    "192.168.1.1/32",
				IPFamily:  ipfamily.IPv4,
				LocalPref: uint32(13),
			},
			shouldErr: true,
		},
		{
			name: "adding to back",
			current: []*advertisementConfig{
				{
					Prefix:   "192.168.1.1/32",
					IPFamily: ipfamily.IPv4,
				},
			},
			toAdd: &advertisementConfig{
				Prefix:   "192.168.1.2/32",
				IPFamily: ipfamily.IPv4,
			},
			expected: []*advertisementConfig{
				{
					Prefix:   "192.168.1.1/32",
					IPFamily: ipfamily.IPv4,
				},
				{
					Prefix:   "192.168.1.2/32",
					IPFamily: ipfamily.IPv4,
				},
			},
		},
		{
			name: "adding to head",
			current: []*advertisementConfig{
				{
					Prefix:   "192.168.1.2/32",
					IPFamily: ipfamily.IPv4,
				},
			},
			toAdd: &advertisementConfig{
				Prefix:   "192.168.1.1/32",
				IPFamily: ipfamily.IPv4,
			},
			expected: []*advertisementConfig{
				{
					Prefix:   "192.168.1.1/32",
					IPFamily: ipfamily.IPv4,
				},
				{
					Prefix:   "192.168.1.2/32",
					IPFamily: ipfamily.IPv4,
				},
			},
		},
		{
			name: "adding in the middle",
			current: []*advertisementConfig{
				{
					Prefix:   "192.168.1.1/32",
					IPFamily: ipfamily.IPv4,
				},
				{
					Prefix:   "192.168.1.3/32",
					IPFamily: ipfamily.IPv4,
				},
			},
			toAdd: &advertisementConfig{
				Prefix:   "192.168.1.2/32",
				IPFamily: ipfamily.IPv4,
			},
			expected: []*advertisementConfig{
				{
					Prefix:   "192.168.1.1/32",
					IPFamily: ipfamily.IPv4,
				},
				{
					Prefix:   "192.168.1.2/32",
					IPFamily: ipfamily.IPv4,
				},
				{
					Prefix:   "192.168.1.3/32",
					IPFamily: ipfamily.IPv4,
				},
			},
		},
		{
			name: "should add communities",
			current: []*advertisementConfig{
				{
					Prefix:   "192.168.1.1/32",
					IPFamily: ipfamily.IPv4,
				},
				{
					Prefix:   "192.168.1.3/32",
					IPFamily: ipfamily.IPv4,
					Communities: []string{
						"1111:2222",
						"3333:4444",
					},
				},
			},
			toAdd: &advertisementConfig{
				Prefix:   "192.168.1.3/32",
				IPFamily: ipfamily.IPv4,
				Communities: []string{
					"1111:2222",
					"5555:6666",
				},
				LargeCommunities: []string{
					"3333:4444:5555",
					"6666:7777:8888",
				},
			},
			expected: []*advertisementConfig{
				{
					Prefix:   "192.168.1.1/32",
					IPFamily: ipfamily.IPv4,
				},
				{
					Prefix:   "192.168.1.3/32",
					IPFamily: ipfamily.IPv4,
					Communities: []string{
						"1111:2222",
						"3333:4444",
						"5555:6666",
					},
					LargeCommunities: []string{
						"3333:4444:5555",
						"6666:7777:8888",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := addToAdvertisements(tt.current, tt.toAdd)
			if err != nil && !tt.shouldErr {
				t.Fatalf("unexpected error: %s", err)
			}
			if err == nil && tt.shouldErr {
				t.Fatalf("expecting error")
			}
			if !reflect.DeepEqual(res, tt.expected) {
				t.Fatalf("expecting %s got %s", spew.Sdump(tt.expected), spew.Sdump(res))
			}
		})
	}
}

func TestSingleSessionWithInternalASN(t *testing.T) {
	testSetup(t)

	l := log.NewNopLogger()
	sessionManager := mockNewSessionManager(l, logging.LevelInfo)
	defer close(sessionManager.reloadConfig)
	session, err := sessionManager.NewSession(l,
		bgp.SessionParameters{
			PeerAddress:     "10.2.2.254:179",
			SourceAddress:   net.ParseIP("10.1.1.254"),
			MyASN:           102,
			RouterID:        net.ParseIP("10.1.1.254"),
			DynamicASN:      "internal",
			GracefulRestart: true,
			SessionName:     "test-peer"})

	if err != nil {
		t.Fatalf("Could not create session: %s", err)
	}
	defer session.Close()

	testCheckConfigFile(t)
}

func TestSingleSessionWithExternalASN(t *testing.T) {
	testSetup(t)

	l := log.NewNopLogger()
	sessionManager := mockNewSessionManager(l, logging.LevelInfo)
	defer close(sessionManager.reloadConfig)
	session, err := sessionManager.NewSession(l,
		bgp.SessionParameters{
			PeerAddress:     "10.2.2.254:179",
			SourceAddress:   net.ParseIP("10.1.1.254"),
			MyASN:           102,
			RouterID:        net.ParseIP("10.1.1.254"),
			DynamicASN:      "external",
			GracefulRestart: true,
			SessionName:     "test-peer"})

	if err != nil {
		t.Fatalf("Could not create session: %s", err)
	}
	defer session.Close()

	testCheckConfigFile(t)
}
