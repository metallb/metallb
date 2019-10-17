package e2etest

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/proxy"
)

type LoadGenerator struct {
	Target    string
	SocksAddr string

	dialer proxy.ContextDialer
	stats  stats
	stop   chan struct{}
}

type stats struct {
	mu    sync.Mutex
	cond  *sync.Cond
	start time.Time
	prev  *Stats
	cur   *Stats
}

type Stats struct {
	HitsPerNode   map[string]int64
	HitsPerPod    map[string]int64
	HitsPerClient map[string]int64
	Total         int64
	Errors        int64
}

func mkStats() *Stats {
	return &Stats{
		HitsPerNode:   map[string]int64{},
		HitsPerPod:    map[string]int64{},
		HitsPerClient: map[string]int64{},
	}
}

func (s *Stats) Nodes() int {
	return len(s.HitsPerNode)
}

func (s *Stats) Pods() int {
	return len(s.HitsPerPod)
}

func (s *Stats) Clients() int {
	return len(s.HitsPerClient)
}

func (s *Stats) balancedBy(name string, m map[string]int64, allowedImbalance float64) error {
	errs := []string{}

	perThing := float64(s.Total) / float64(len(m)) / float64(s.Total)
	min, max := perThing-allowedImbalance, perThing+allowedImbalance
	for thing, n := range m {
		fraction := float64(n) / float64(s.Total)
		if fraction < min || fraction > max {
			errs = append(errs, fmt.Sprintf("%s %q got %f%% of traffic, want %f—%f%%", name, thing, fraction*100, min*100, max*100))
		}
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

func (s *Stats) BalancedByNode(allowedImbalance float64) error {
	return s.balancedBy("node", s.HitsPerNode, allowedImbalance)
}

func (s *Stats) BalancedByPod(allowedImbalance float64) error {
	return s.balancedBy("pod", s.HitsPerPod, allowedImbalance)
}

func (s *Stats) BalancedByClient(allowedImbalance float64) error {
	return s.balancedBy("client", s.HitsPerClient, allowedImbalance)
}

func (l *LoadGenerator) Run() {
	if l.stop != nil {
		panic("double start of load-generator")
	}
	l.stop = make(chan struct{})
	if l.SocksAddr != "" {
		socks, err := proxy.SOCKS5("tcp", l.SocksAddr, nil, nil)
		if err != nil {
			panic("I can't socks")
		}
		l.dialer = socks.(proxy.ContextDialer)
	} else {
		l.dialer = &net.Dialer{}
	}
	l.stats = stats{
		start: time.Now(),
		prev:  mkStats(),
		cur:   mkStats(),
	}
	l.stats.cond = sync.NewCond(&l.stats.mu)
	go l.run()
}

// Only returns when the state evolves beyond prev.
func (l *LoadGenerator) Stats(prev *Stats) *Stats {
	st := l.stats
	st.mu.Lock()
	defer st.mu.Unlock()

	for st.prev == prev {
		st.cond.Wait()
	}
	for st.prev.Total == 0 {
		st.cond.Wait()
	}

	return st.prev
}

func (l *LoadGenerator) Close() {
	select {
	case <-l.stop:
	default:
		close(l.stop)
	}
}

func (l *LoadGenerator) run() {
	for {
		select {
		case <-l.stop:
			return
		default:
		}
		l.oneRequest()
	}
}

func (l *LoadGenerator) oneRequest() {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	conn, err := l.dialer.DialContext(ctx, "tcp", l.Target)
	if err != nil {
		l.recordError()
	}
	defer conn.Close()

	bs, err := ioutil.ReadAll(conn)
	if err != nil {
		l.recordError()
		return
	}

	fs := strings.Split(string(bs), "\n")
	node, pod, client := fs[0], fs[1], fs[2]
	l.recordHit(node, pod, client)
}

func (l *LoadGenerator) recordHit(node, pod, client string) {
	st := l.stats
	st.mu.Lock()
	defer st.mu.Unlock()
	if time.Since(st.start) > time.Second {
		st.start = time.Now()
		st.prev = st.cur
		st.cur = &Stats{
			HitsPerNode:   map[string]int64{},
			HitsPerPod:    map[string]int64{},
			HitsPerClient: map[string]int64{},
		}
		st.cond.Broadcast()
	}

	st.cur.Total++
	st.cur.HitsPerNode[node]++
	st.cur.HitsPerPod[pod]++
	st.cur.HitsPerClient[client]++
}

func (l *LoadGenerator) recordError() {
	st := l.stats
	st.mu.Lock()
	defer st.mu.Unlock()
	if time.Since(st.start) > time.Second {
		st.start = time.Now()
		st.prev = st.cur
		st.cur = &Stats{
			HitsPerNode:   map[string]int64{},
			HitsPerPod:    map[string]int64{},
			HitsPerClient: map[string]int64{},
		}
		st.cond.Broadcast()
	}

	st.cur.Total++
	st.cur.Errors++
}
