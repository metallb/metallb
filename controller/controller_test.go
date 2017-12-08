package main

import (
	"fmt"
	"net"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/kr/pretty"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"go.universe.tf/metallb/internal/allocator"
	"go.universe.tf/metallb/internal/config"
)

func ipnet(s string) *net.IPNet {
	_, n, err := net.ParseCIDR(s)
	if err != nil {
		panic(err)
	}
	return n
}

func statusAssigned(ip string) v1.ServiceStatus {
	return v1.ServiceStatus{
		LoadBalancer: v1.LoadBalancerStatus{
			Ingress: []v1.LoadBalancerIngress{
				{
					IP: ip,
				},
			},
		},
	}
}

// testK8S implements service by recording what the controller wants
// to do to k8s.
type testK8S struct {
	updateService       *v1.Service
	updateServiceStatus *v1.ServiceStatus
	loggedWarning       bool
	t                   *testing.T
}

func (s *testK8S) Update(svc *v1.Service) (*v1.Service, error) {
	s.t.Logf("k8s service updated")
	s.updateService = svc
	return svc, nil
}

func (s *testK8S) UpdateStatus(svc *v1.Service) error {
	s.t.Logf("k8s service status updated")
	s.updateServiceStatus = &svc.Status
	return nil
}

func (s *testK8S) Infof(_ *v1.Service, evtType string, msg string, args ...interface{}) {
	s.t.Logf("k8s Info event %q: %s", evtType, fmt.Sprintf(msg, args...))
}

func (s *testK8S) Errorf(_ *v1.Service, evtType string, msg string, args ...interface{}) {
	s.t.Logf("k8s Warning event %q: %s", evtType, fmt.Sprintf(msg, args...))
	s.loggedWarning = true
}

func (s *testK8S) reset() {
	s.updateService = nil
	s.updateServiceStatus = nil
	s.loggedWarning = false
}

func TestControllerMutation(t *testing.T) {
	k := &testK8S{t: t}
	c := &controller{
		ips:    allocator.New(),
		client: k,
	}
	cfg := &config.Config{
		Pools: map[string]*config.Pool{
			"pool1": &config.Pool{
				CIDR: []*net.IPNet{ipnet("1.2.3.0/31")},
			},
		},
	}

	// For this test, we just set a static config and immediately sync
	// the controller. The mutations around config setting and syncing
	// are tested elsewhere.
	if err := c.SetConfig(cfg); err != nil {
		t.Fatalf("SetConfig: %s", err)
	}
	c.MarkSynced()

	// In steady state, every input below should be equivalent to a
	// pure function that reliably produces the same end state
	// regardless of past controller state.
	tests := []struct {
		desc    string
		in      *v1.Service
		want    *v1.Service
		wantErr bool
	}{
		{
			desc: "deleted balancer",
		},

		{
			desc: "simple non-LoadBalancer",
			in: &v1.Service{
				Spec: v1.ServiceSpec{
					Type:      "ClusterIP",
					ClusterIP: "1.2.3.4",
				},
			},
		},

		{
			desc: "simple LoadBalancer",
			in: &v1.Service{
				Spec: v1.ServiceSpec{
					Type: "LoadBalancer",
				},
			},
			want: &v1.Service{
				Spec: v1.ServiceSpec{
					Type: "LoadBalancer",
				},
				Status: statusAssigned("1.2.3.0"),
			},
		},

		{
			desc: "request specific IP",
			in: &v1.Service{
				Spec: v1.ServiceSpec{
					Type:           "LoadBalancer",
					LoadBalancerIP: "1.2.3.1",
				},
				Status: statusAssigned("1.2.3.0"),
			},
			want: &v1.Service{
				Spec: v1.ServiceSpec{
					Type:           "LoadBalancer",
					LoadBalancerIP: "1.2.3.1",
				},
				Status: statusAssigned("1.2.3.1"),
			},
		},

		{
			desc: "request infeasible IP",
			in: &v1.Service{
				Spec: v1.ServiceSpec{
					Type:           "LoadBalancer",
					LoadBalancerIP: "1.2.3.4",
				},
				Status: statusAssigned("1.2.3.1"),
			},
			want: &v1.Service{
				Spec: v1.ServiceSpec{
					Type:           "LoadBalancer",
					LoadBalancerIP: "1.2.3.4",
				},
			},
			wantErr: true,
		},

		{
			desc: "request IP from specific pool",
			in: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"metallb.universe.tf/address-pool": "pool1",
					},
				},
				Spec: v1.ServiceSpec{
					Type: "LoadBalancer",
				},
			},
			want: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"metallb.universe.tf/address-pool": "pool1",
					},
				},
				Spec: v1.ServiceSpec{
					Type: "LoadBalancer",
				},
				Status: statusAssigned("1.2.3.0"),
			},
		},

		{
			desc: "unknown pool requested",
			in: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"metallb.universe.tf/address-pool": "does-not-exist",
					},
				},
				Spec: v1.ServiceSpec{
					Type: "LoadBalancer",
				},
			},
			wantErr: true,
		},

		{
			desc: "invalid IP assigned",
			in: &v1.Service{
				Spec: v1.ServiceSpec{
					Type: "LoadBalancer",
				},
				Status: statusAssigned("2.3.4.5"),
			},
			want: &v1.Service{
				Spec: v1.ServiceSpec{
					Type: "LoadBalancer",
				},
				Status: statusAssigned("1.2.3.0"),
			},
		},

		{
			desc: "invalid ingress state",
			in: &v1.Service{
				Spec: v1.ServiceSpec{
					Type: "LoadBalancer",
				},
				Status: v1.ServiceStatus{
					LoadBalancer: v1.LoadBalancerStatus{
						Ingress: []v1.LoadBalancerIngress{
							{
								Hostname: "foo.bar.local",
							},
							{
								IP: "10.10.10.10",
							},
						},
					},
				},
			},
			want: &v1.Service{
				Spec: v1.ServiceSpec{
					Type: "LoadBalancer",
				},
				Status: statusAssigned("1.2.3.0"),
			},
		},
	}

	for _, test := range tests {
		t.Logf("Running case %q", test.desc)
		k.reset()
		// Delete the test balancer, to clean up all state

		if err := c.SetBalancer("test", test.in, nil); err != nil {
			t.Errorf("%q: SetBalancer returned error: %s", test.desc, err)
			continue
		}
		if test.wantErr != k.loggedWarning {
			t.Errorf("%q: unexpected loggedWarning value, want %v, got %v", test.desc, test.wantErr, k.loggedWarning)
		}

		gotSvc := k.updateService
		if k.updateServiceStatus != nil {
			if gotSvc == nil {
				gotSvc = new(v1.Service)
				*gotSvc = *test.in
			}
			gotSvc.Status = *k.updateServiceStatus
		}
		pretty.Print(test.in)
		pretty.Print(gotSvc)
		// v5 of the k8s client does not correctly compare nil
		// *metav1.Time objects, which svc.ObjectMeta contains. Add
		// some dummy non-nil values to all of in, want, got to work
		// around this until we migrate to v6.
		if test.in != nil {
			test.in.ObjectMeta.DeletionTimestamp = &metav1.Time{}
		}
		if test.want != nil {
			test.want.ObjectMeta.DeletionTimestamp = &metav1.Time{}
		}
		if gotSvc != nil {
			gotSvc.ObjectMeta.DeletionTimestamp = &metav1.Time{}
		}

		switch {
		case test.want == nil && gotSvc != nil:
			t.Errorf("%q: unexpectedly mutated service (-in +out)\n%s", test.desc, cmp.Diff(test.in, gotSvc))
		case test.want != nil && gotSvc == nil:
			t.Errorf("%q: did not mutate service, wanted (-in +out)\n%s", test.desc, cmp.Diff(test.in, test.want))
		case test.want != nil && gotSvc != nil:
			if diff := cmp.Diff(test.want, gotSvc); diff != "" {
				t.Errorf("%q: wrong service mutation (-want +got)\n%s", test.desc, diff)
			}
		}
	}
}
