package main

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	vk "go.universe.tf/virtuakube"
)

type LoadGenerator struct {
	client    *vk.VM
	transport *http.Transport
	targets   map[string]*stats
	stop      chan struct{}
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

func NewLoadGenerator(client *vk.VM, urls ...string) *LoadGenerator {
	ret := &LoadGenerator{
		client: client,
		transport: &http.Transport{
			Dial: func(network, addr string) (net.Conn, error) {
				return client.Dial(network, addr)
			},
			DisableKeepAlives: true,
		},
		targets: map[string]*stats{},
		stop:    make(chan struct{}),
	}
	for _, url := range urls {
		ret.targets[url] = &stats{
			start: time.Now(),
			prev: &Stats{
				HitsPerNode:   map[string]int64{},
				HitsPerPod:    map[string]int64{},
				HitsPerClient: map[string]int64{},
			},
			cur: &Stats{
				HitsPerNode:   map[string]int64{},
				HitsPerPod:    map[string]int64{},
				HitsPerClient: map[string]int64{},
			},
		}
		ret.targets[url].cond = sync.NewCond(&ret.targets[url].mu)
		go ret.run(url)
	}
	return ret
}

// Only returns when the state evolves beyond prev.
func (l *LoadGenerator) Stats(url string, prev *Stats) *Stats {
	st := l.targets[url]
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

func (l *LoadGenerator) AllStats(prev map[string]*Stats) map[string]*Stats {
	ret := map[string]*Stats{}

	if prev == nil {
		prev = map[string]*Stats{}
	}

	for url := range l.targets {
		ret[url] = l.Stats(url, prev[url])
	}

	return ret
}

func (l *LoadGenerator) Close() {
	select {
	case <-l.stop:
	default:
		close(l.stop)
	}
}

func (l *LoadGenerator) run(url string) {
	for {
		select {
		case <-l.stop:
			return
		default:
		}
		l.oneRequest(url)
	}
}

func (l *LoadGenerator) oneRequest(url string) {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		panic(err)
	}
	req = req.WithContext(ctx)

	resp, err := l.transport.RoundTrip(req)
	if err != nil {
		l.recordError(url)
		return
	}
	defer resp.Body.Close()
	bs, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		l.recordError(url)
		return
	}

	fs := strings.Split(string(bs), "\n")
	node, pod, client := fs[0], fs[1], fs[2]
	l.recordHit(url, node, pod, client)
}

func (l *LoadGenerator) recordHit(url, node, pod, client string) {
	st := l.targets[url]
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

func (l *LoadGenerator) recordError(url string) {
	st := l.targets[url]
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
