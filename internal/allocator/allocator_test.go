package allocator

import (
	"fmt"
	"net"
	"testing"

	"go.universe.tf/metallb/internal/config"
)

func TestAssignment(t *testing.T) {
	alloc := New()
	if err := alloc.SetPools(pools(
		pool("test", false, true, "1.2.3.4/31"),
		pool("test2", true, true, "1.2.4.0/24"))); err != nil {
		t.Fatalf("SetPools: %s", err)
	}

	tests := []struct {
		desc    string
		svc     string
		ip      string
		wantErr bool
	}{
		{
			desc: "assign s1",
			svc:  "s1",
			ip:   "1.2.3.4",
		},
		{
			desc: "s1 idempotent reassign",
			svc:  "s1",
			ip:   "1.2.3.4",
		},
		{
			desc:    "s2 can't grab s1's IP",
			svc:     "s2",
			ip:      "1.2.3.4",
			wantErr: true,
		},
		{
			desc: "s2 can get the other IP",
			svc:  "s2",
			ip:   "1.2.3.5",
		},
		{
			desc:    "s1 now can't grab s2's IP",
			svc:     "s1",
			ip:      "1.2.3.5",
			wantErr: true,
		},
		{
			desc: "s1 frees its IP",
			svc:  "s1",
			ip:   "",
		},
		{
			desc: "s2 can grab s1's former IP",
			svc:  "s2",
			ip:   "1.2.3.4",
		},
		{
			desc: "s1 can now grab s2's former IP",
			svc:  "s1",
			ip:   "1.2.3.5",
		},
		{
			desc:    "s3 cannot grab a 0 buggy IP",
			svc:     "s3",
			ip:      "1.2.4.0",
			wantErr: true,
		},
		{
			desc:    "s3 cannot grab a 255 buggy IP",
			svc:     "s3",
			ip:      "1.2.4.255",
			wantErr: true,
		},
		{
			desc: "s3 can grab another IP in that pool",
			svc:  "s3",
			ip:   "1.2.4.254",
		},
	}

	for _, test := range tests {
		if test.ip == "" {
			alloc.Unassign(test.svc)
			continue
		}
		ip := net.ParseIP(test.ip)
		if ip == nil {
			t.Fatalf("invalid IP %q in test %q", test.ip, test.desc)
		}
		err := alloc.Assign(test.svc, ip)
		if test.wantErr {
			if err == nil {
				t.Errorf("%q should have caused an error, but did not", test.desc)
			} else if a := assigned(alloc, test.svc); a == test.ip {
				t.Errorf("%q: Assign(%q, %q) failed, but allocator did record allocation", test.desc, test.svc, test.ip)
			}

			continue
		}

		if err != nil {
			t.Errorf("%q: Assign(%q, %q): %s", test.desc, test.svc, test.ip, err)
		}
		if a := assigned(alloc, test.svc); a != test.ip {
			t.Errorf("%q: ran Assign(%q, %q), but allocator has recorded allocation of %q", test.desc, test.svc, test.ip, a)
		}
	}
}

func TestPoolAllocation(t *testing.T) {
	alloc := New()
	// This test only allocates from the "test" pool, so it will run
	// out of IPs quickly even though there are tons available in
	// other pools.
	if err := alloc.SetPools(pools(
		pool("not_this_one", false, true, "192.168.0.0/16"),
		pool("test", false, true, "1.2.3.4/31", "1.2.3.10/31"),
		pool("test2", false, true, "10.20.30.0/24"))); err != nil {
		t.Fatalf("SetPools: %s", err)
	}

	validIPs := map[string]bool{
		"1.2.3.4":  true,
		"1.2.3.5":  true,
		"1.2.3.10": true,
		"1.2.3.11": true,
	}

	tests := []struct {
		svc      string
		unassign bool
		wantErr  bool
	}{
		{svc: "s1"},
		{svc: "s2"},
		{svc: "s3"},
		{svc: "s4"},
		{
			svc:     "s5",
			wantErr: true,
		},
		{
			svc:     "s6",
			wantErr: true,
		},
		{
			svc:      "s1",
			unassign: true,
		},
		{svc: "s5"},
		{
			svc:     "s6",
			wantErr: true,
		},
	}

	for i, test := range tests {
		if test.unassign {
			alloc.Unassign(test.svc)
			continue
		}
		ip, err := alloc.AllocateFromPool(test.svc, "test")
		if test.wantErr {
			if err == nil {
				t.Errorf("#%d should have caused an error, but did not", i+1)

			}
			continue
		}
		if err != nil {
			t.Errorf("#%d AllocateFromPool(%q, \"test\"): %s", i+1, test.svc, err)
		}
		if !validIPs[ip.String()] {
			t.Errorf("#%d allocated unexpected IP %q", i+1, ip)
		}
	}

	alloc.Unassign("s5")
	if _, err := alloc.AllocateFromPool("s5", "nonexistentpool"); err == nil {
		t.Error("Allocating from non-existent pool succeeded")
	}
}

func TestAllocation(t *testing.T) {
	alloc := New()
	if err := alloc.SetPools(pools(
		pool("test1", false, true, "1.2.3.4/31"),
		pool("test2", false, true, "1.2.3.10/31"))); err != nil {
		t.Fatalf("SetPools: %s", err)
	}

	validIPs := map[string]bool{
		"1.2.3.4":  true,
		"1.2.3.5":  true,
		"1.2.3.10": true,
		"1.2.3.11": true,
	}

	tests := []struct {
		svc      string
		unassign bool
		wantErr  bool
	}{
		{svc: "s1"},
		{svc: "s2"},
		{svc: "s3"},
		{svc: "s4"},
		{
			svc:     "s5",
			wantErr: true,
		},
		{
			svc:     "s6",
			wantErr: true,
		},
		{
			svc:      "s1",
			unassign: true,
		},
		{svc: "s5"},
		{
			svc:     "s6",
			wantErr: true,
		},
	}

	for i, test := range tests {
		if test.unassign {
			alloc.Unassign(test.svc)
			continue
		}
		ip, err := alloc.Allocate(test.svc)
		if test.wantErr {
			if err == nil {
				t.Errorf("#%d should have caused an error, but did not", i+1)

			}
			continue
		}
		if err != nil {
			t.Errorf("#%d Allocate(%q, \"test\"): %s", i+1, test.svc, err)
		}
		if !validIPs[ip.String()] {
			t.Errorf("#%d allocated unexpected IP %q", i+1, ip)
		}
	}
}

func TestBuggyIPs(t *testing.T) {
	alloc := New()
	if err := alloc.SetPools(pools(
		pool("test", false, true, "1.2.3.0/31"),
		pool("test2", false, true, "1.2.3.254/31"),
		pool("test3", true, true, "1.2.4.0/31"),
		pool("test4", true, true, "1.2.4.254/31"))); err != nil {
		t.Fatalf("SetPools: %s", err)
	}

	validIPs := map[string]bool{
		"1.2.3.0":   true,
		"1.2.3.1":   true,
		"1.2.3.254": true,
		"1.2.3.255": true,
		"1.2.4.1":   true,
		"1.2.4.254": true,
	}

	tests := []struct {
		svc     string
		wantErr bool
	}{
		{svc: "s1"},
		{svc: "s2"},
		{svc: "s3"},
		{svc: "s4"},
		{svc: "s5"},
		{svc: "s6"},
		{
			svc:     "s7",
			wantErr: true,
		},
	}

	for i, test := range tests {
		ip, err := alloc.Allocate(test.svc)
		if test.wantErr {
			if err == nil {
				t.Errorf("#%d should have caused an error, but did not", i+1)

			}
			continue
		}
		if err != nil {
			t.Errorf("#%d Allocate(%q, \"test\"): %s", i+1, test.svc, err)
		}
		if !validIPs[ip.String()] {
			t.Errorf("#%d allocated unexpected IP %q", i+1, ip)
		}
	}

}

func TestNextIP(t *testing.T) {
	tests := []struct {
		in, out string
	}{
		{"1.2.3.4", "1.2.3.5"},
		{"0.0.0.0", "0.0.0.1"},
		{"1.2.3.255", "1.2.4.0"},
		{"1.2.255.255", "1.3.0.0"},
		{"1.255.255.255", "2.0.0.0"},
		{"255.255.255.255", "0.0.0.0"},
		{"::1", "::2"},
	}

	for i, test := range tests {
		in, out := net.ParseIP(test.in), net.ParseIP(test.out)
		if in == nil || out == nil {
			t.Fatalf("Invalid test case #%d, IPs don't parse", i+1)
		}
		got := nextIP(in)
		if !got.Equal(out) {
			t.Errorf("nextIP(%q), got %q, want %q", in, got, out)
		}
	}
}

func TestConfigReload(t *testing.T) {
	alloc := New()
	if err := alloc.SetPools(pool("test", false, true, "1.2.3.0/30")); err != nil {
		t.Fatalf("SetPools: %s", err)
	}
	if err := alloc.Assign("s1", net.ParseIP("1.2.3.0")); err != nil {
		t.Fatalf("Assign(s1, 1.2.3.0): %s", err)
	}

	tests := []struct {
		desc    string
		pools   map[string]*config.Pool
		wantErr bool
		pool    string // Pool that 1.2.3.0 should be in
	}{
		{
			desc:  "set same config is no-op",
			pools: pool("test", false, true, "1.2.3.0/30"),
			pool:  "test",
		},
		{
			desc:  "expand pool",
			pools: pool("test", false, true, "1.2.3.0/24"),
			pool:  "test",
		},
		{
			desc:  "shrink pool",
			pools: pool("test", false, true, "1.2.3.0/30"),
			pool:  "test",
		},
		{
			desc:    "can't shrink further",
			pools:   pool("test", false, true, "1.2.3.2/31"),
			pool:    "test",
			wantErr: true,
		},
		{
			desc:  "rename the pool",
			pools: pool("test2", false, true, "1.2.3.0/30"),
			pool:  "test2",
		},
		{
			desc:  "split pool",
			pools: pools(pool("test", false, true, "1.2.3.0/31"), pool("test2", false, true, "1.2.3.2/31")),
			pool:  "test",
		},
		{
			desc:  "swap pool names",
			pools: pools(pool("test2", false, true, "1.2.3.0/31"), pool("test", false, true, "1.2.3.2/31")),
			pool:  "test2",
		},
		{
			desc:    "delete used pool",
			pools:   pool("test", true, true, "1.2.3.2/31"),
			pool:    "test2",
			wantErr: true,
		},
		{
			desc:  "delete unused pool",
			pools: pool("test2", false, true, "1.2.3.0/31"),
			pool:  "test2",
		},
		{
			desc:    "enable buggy IPs not allowed",
			pools:   pool("test2", true, true, "1.2.3.0/31"),
			pool:    "test2",
			wantErr: true,
		},
	}

	for _, test := range tests {
		err := alloc.SetPools(test.pools)
		if test.wantErr {
			if err == nil {
				t.Errorf("%q should have failed to SetPools, but succeeded", test.desc)
			}
		} else if err != nil {
			t.Errorf("%q failed to SetPools: %s", test.desc, err)
		}
		gotPool := assignedPool(alloc, "s1")
		if gotPool != test.pool {
			t.Errorf("%q: s1 is in wrong pool, want %q, got %q", test.desc, test.pool, gotPool)
		}
	}
}

func TestAutoAssign(t *testing.T) {
	alloc := New()
	if err := alloc.SetPools(pools(
		pool("test1", false, false, "1.2.3.4/31"),
		pool("test2", false, true, "1.2.3.10/31"))); err != nil {
		t.Fatalf("SetPools: %s", err)
	}

	validIPs := map[string]bool{
		"1.2.3.4":  false,
		"1.2.3.5":  false,
		"1.2.3.10": true,
		"1.2.3.11": true,
	}

	tests := []struct {
		svc     string
		wantErr bool
	}{
		{svc: "s1"},
		{svc: "s2"},
		{
			svc:     "s3",
			wantErr: true,
		},
		{
			svc:     "s4",
			wantErr: true,
		},
		{
			svc:     "s5",
			wantErr: true,
		},
	}

	for i, test := range tests {
		ip, err := alloc.Allocate(test.svc)
		if test.wantErr {
			if err == nil {
				t.Errorf("#%d should have caused an error, but did not", i+1)
			}
			continue
		}
		if err != nil {
			t.Errorf("#%d Allocate(%q, \"test\"): %s", i+1, test.svc, err)
		}
		if !validIPs[ip.String()] {
			t.Errorf("#%d allocated unexpected IP %q", i+1, ip)
		}
	}
}

// Some helpers

// Peeks inside Allocator to find the allocated IP and pool for a service.
func assigned(a *Allocator, svc string) string {
	if a.svcToIP[svc] == nil {
		return ""
	}
	return a.svcToIP[svc].String()
}

func assignedPool(a *Allocator, svc string) string {
	return a.svcToPool[svc]
}

func pools(pools ...map[string]*config.Pool) map[string]*config.Pool {
	ret := map[string]*config.Pool{}
	for _, p := range pools {
		for k, v := range p {
			ret[k] = v
		}
	}
	return ret
}

func pool(name string, avoidBuggyIPs, autoAssign bool, cidrs ...string) map[string]*config.Pool {
	ret := &config.Pool{
		AvoidBuggyIPs: avoidBuggyIPs,
		AutoAssign:    autoAssign,
	}
	for _, cidr := range cidrs {
		_, n, err := net.ParseCIDR(cidr)
		if err != nil {
			panic(fmt.Sprintf("malformed CIDR %q", cidr))
		}
		ret.CIDR = append(ret.CIDR, n)
	}
	return map[string]*config.Pool{
		name: ret,
	}
}
