// SPDX-License-Identifier:Apache-2.0

package frr

import (
	"testing"
	"time"
)

const timer = 10 * time.Millisecond

func TestDebounce(t *testing.T) {
	result := make(chan *frrConfig, 10) // buffered to accomodate spurious rewrites
	dummyUpdate := func(config *frrConfig) {
		result <- config
	}

	reload := make(chan *frrConfig)
	defer close(reload)
	debouncer(dummyUpdate, reload, timer)
	reload <- &frrConfig{Hostname: "1"}
	reload <- &frrConfig{Hostname: "2"}
	reload <- &frrConfig{Hostname: "3"}
	if len(result) != 0 {
		t.Fatal("received update before time")
	}
	time.Sleep(3 * timer)
	if len(result) != 1 {
		t.Fatal("received extra updates", len(result))
	}
	updated := <-result
	if updated.Hostname != "3" {
		t.Fatal("Config was not updated")
	}
	reload <- &frrConfig{Hostname: "3"}
	reload <- &frrConfig{Hostname: "4"}
	reload <- &frrConfig{Hostname: "5"}
	time.Sleep(3 * timer)
	if len(result) != 1 {
		t.Fatal("received extra updates", len(result))
	}
	updated = <-result
	if updated.Hostname != "5" {
		t.Fatal("Config was not updated")
	}
}
