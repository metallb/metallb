// SPDX-License-Identifier:Apache-2.0

package frr

import (
	"fmt"
	"testing"
	"time"

	"github.com/go-kit/log"
)

const timer = 10 * time.Millisecond
const failureTimer = 10 * time.Millisecond

func TestDebounce(t *testing.T) {
	result := make(chan *frrConfig, 10) // buffered to accommodate spurious rewrites
	dummyUpdate := func(config *frrConfig) error {
		result <- config
		return nil
	}

	reload := make(chan reloadEvent)
	defer close(reload)
	debouncer(dummyUpdate, reload, timer, failureTimer, log.NewNopLogger())
	reload <- reloadEvent{config: &frrConfig{Hostname: "1"}}
	reload <- reloadEvent{config: &frrConfig{Hostname: "2"}}
	reload <- reloadEvent{config: &frrConfig{Hostname: "3"}}
	time.Sleep(3 * timer)
	if len(result) < 1 || len(result) > 3 {
		t.Fatal("received unexpected number of updates", len(result))
	}
	var updated *frrConfig
	for range len(result) {
		updated = <-result
	}
	if updated.Hostname != "3" {
		t.Fatal("Config was not updated")
	}

	reload <- reloadEvent{config: &frrConfig{Hostname: "3"}}
	reload <- reloadEvent{config: &frrConfig{Hostname: "4"}}
	reload <- reloadEvent{config: &frrConfig{Hostname: "5"}}
	time.Sleep(3 * timer)
	if len(result) < 1 || len(result) > 3 {
		t.Fatal("received unexpected number of updates", len(result))
	}
	for range len(result) {
		updated = <-result
	}
	if updated.Hostname != "5" {
		t.Fatal("Config was not updated")
	}
}

func TestDebounceRetry(t *testing.T) {
	result := make(chan *frrConfig, 10) // buffered to accommodate spurious rewrites
	count := 0
	dummyUpdate := func(config *frrConfig) error {
		count++
		if count <= 3 {
			return fmt.Errorf("error")
		}
		result <- config
		return nil
	}

	reload := make(chan reloadEvent)
	defer close(reload)
	debouncer(dummyUpdate, reload, timer, failureTimer, log.NewNopLogger())

	reload <- reloadEvent{config: &frrConfig{Hostname: "1"}}
	reload <- reloadEvent{config: &frrConfig{Hostname: "2"}}
	time.Sleep(10 * failureTimer)
	if len(result) < 1 || len(result) > 2 {
		t.Fatal("received unexpected number of updates", len(result))
	}
	var updated *frrConfig
	for range len(result) {
		updated = <-result
	}
	if updated.Hostname != "2" {
		t.Fatal("Config was not updated")
	}
}

func TestDebounceReuseOld(t *testing.T) {
	result := make(chan *frrConfig, 10) // buffered to accommodate spurious rewrites
	dummyUpdate := func(config *frrConfig) error {
		result <- config
		return nil
	}

	reload := make(chan reloadEvent)
	defer close(reload)
	debouncer(dummyUpdate, reload, timer, failureTimer, log.NewNopLogger())

	reload <- reloadEvent{config: &frrConfig{Hostname: "1"}}
	time.Sleep(3 * timer)
	if len(result) != 1 {
		t.Fatal("received extra updates", len(result))
	}
	updated := <-result
	if updated.Hostname != "1" {
		t.Fatal("Config was not updated")
	}
	// reload to see if the debouncer uses the old config
	reload <- reloadEvent{useOld: true}
	time.Sleep(3 * timer)
	if len(result) != 1 {
		t.Fatal("received extra updates", len(result))
	}
	updated = <-result
	if updated.Hostname != "1" {
		t.Fatal("Config was not updated")
	}
}

func TestDebounceSameConfig(t *testing.T) {
	result := make(chan *frrConfig, 10) // buffered to accommodate spurious rewrites
	dummyUpdate := func(config *frrConfig) error {
		result <- config
		return nil
	}

	reload := make(chan reloadEvent)
	defer close(reload)
	debouncer(dummyUpdate, reload, timer, failureTimer, log.NewNopLogger())
	reload <- reloadEvent{config: &frrConfig{Hostname: "1"}}
	reload <- reloadEvent{config: &frrConfig{Hostname: "2"}}
	reload <- reloadEvent{config: &frrConfig{Hostname: "3", Routers: []*routerConfig{{MyASN: 23}}}}
	time.Sleep(3 * timer)
	if len(result) < 1 || len(result) > 3 {
		t.Fatal("received unexpected number of updates", len(result))
	}
	var updated *frrConfig
	for range len(result) {
		updated = <-result
	}
	if updated.Hostname != "3" {
		t.Fatal("Config was not updated")
	}

	reload <- reloadEvent{config: &frrConfig{Hostname: "3", Routers: []*routerConfig{{MyASN: 23}}}}
	reload <- reloadEvent{config: &frrConfig{Hostname: "3", Routers: []*routerConfig{{MyASN: 23}}}}

	time.Sleep(3 * timer)
	if len(result) != 0 {
		updated := <-result
		t.Fatalf("received extra updates: %d %s", len(result), updated.Hostname)
	}
}
