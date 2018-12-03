//  Copyright (c) 2018 Cisco and/or its affiliates.
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at:
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.

package agent_test // Different name from package agent to insure we test with the 'outside the package' experience

import (
	"fmt"
	"os"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/ligato/cn-infra/agent"
	"github.com/ligato/cn-infra/infra"
	"github.com/ligato/cn-infra/logging/logrus"
	. "github.com/onsi/gomega"
)

const (
	initFailedErrorString      = "Init failed"
	afterInitFailedErrorString = "AfterInit failed"
	closeFailedErrorString     = "Close failed"
	defaultPluginName          = "testplugin"
)

func TestEmptyAgent(t *testing.T) {
	RegisterTestingT(t)

	agent := agent.NewAgent()
	Expect(agent).NotTo(BeNil())
	Expect(agent.Options()).NotTo(BeNil())
	Expect(agent.Options().Plugins).To(BeNil())
	err := agent.Start()
	Expect(err).To(BeNil())
	err = agent.Stop()
	Expect(err).To(BeNil())
}

func TestStopBeforeStart(t *testing.T) {
	RegisterTestingT(t)

	agent := agent.NewAgent()
	err := agent.Stop()
	Expect(err).ToNot(BeNil())
	err = agent.Start()
	Expect(err).To(BeNil())
}

func TestWaitBeforeStart(t *testing.T) {
	RegisterTestingT(t)

	agent := agent.NewAgent()
	err := agent.Wait()
	Expect(err).ToNot(BeNil())
	err = agent.Start()
	Expect(err).To(BeNil())
}

func TestAfterBeforeStart(t *testing.T) {
	RegisterTestingT(t)
	agent := agent.NewAgent()
	<-agent.After()
	err := agent.Error()
	Expect(err).ToNot(BeNil())
}

func TestAgentWithPlugin(t *testing.T) {
	RegisterTestingT(t)
	np1 := &TestPlugin{}

	agent := agent.NewAgent(agent.Plugins(np1))
	Expect(agent.Options()).NotTo(BeNil())
	Expect(agent.Options().Plugins).ToNot(BeNil())
	Expect(len(agent.Options().Plugins)).To(Equal(1))
	Expect(agent.Options().Plugins[0]).To(Equal(np1))

	err := agent.Start()
	Expect(err).To(BeNil())
	Expect(agent.Options().Plugins[0].(*TestPlugin).Initialized()).To(BeTrue())
	Expect(agent.Options().Plugins[0].(*TestPlugin).AfterInitialized()).To(BeTrue())
	Expect(agent.Options().Plugins[0].(*TestPlugin).Closed()).To(BeFalse())

	err = agent.Stop()
	Expect(agent.Options().Plugins[0].(*TestPlugin).Closed()).To(BeTrue())

	Expect(err).To(BeNil())
}

func TestAgentWithPluginInitFailed(t *testing.T) {
	RegisterTestingT(t)
	agent := agent.NewAgent(agent.Plugins(NewTestPlugin(true, false, false)))
	Expect(agent.Options()).NotTo(BeNil())
	Expect(agent.Options().Plugins).ToNot(BeNil())

	err := agent.Start()
	Expect(err).ToNot(BeNil())
	Expect(err.Error()).To(Equal(initFailedErrorString))
	Expect(agent.Options().Plugins[0].(*TestPlugin).Initialized()).To(BeTrue())
	Expect(agent.Options().Plugins[0].(*TestPlugin).AfterInitialized()).To(BeFalse())
	Expect(agent.Options().Plugins[0].(*TestPlugin).Closed()).To(BeFalse())

	err = agent.Stop()
	Expect(err).To(HaveOccurred())

	err = agent.Wait()
	Expect(err).To(HaveOccurred())
}

func TestAgentWithPluginAfterInitFailed(t *testing.T) {
	RegisterTestingT(t)
	agent := agent.NewAgent(agent.Plugins(NewTestPlugin(false, true, false)))
	Expect(agent.Options()).NotTo(BeNil())
	Expect(agent.Options().Plugins).ToNot(BeNil())

	err := agent.Start()
	Expect(err).To(HaveOccurred())
	Expect(agent.Options().Plugins[0].(*TestPlugin).Initialized()).To(BeTrue())
	Expect(agent.Options().Plugins[0].(*TestPlugin).AfterInitialized()).To(BeTrue())
	Expect(agent.Options().Plugins[0].(*TestPlugin).Closed()).To(BeFalse())

	err = agent.Stop()
	Expect(err).To(HaveOccurred())

	err = agent.Wait()
	Expect(err).To(HaveOccurred())
}

func TestAgentWithPluginCloseFailed(t *testing.T) {
	RegisterTestingT(t)
	agent := agent.NewAgent(agent.Plugins(NewTestPlugin(false, false, true)))
	Expect(agent.Options()).NotTo(BeNil())
	Expect(agent.Options().Plugins).ToNot(BeNil())

	err := agent.Start()
	Expect(err).To(BeNil())
	Expect(agent.Options().Plugins[0].(*TestPlugin).Initialized()).To(BeTrue())
	Expect(agent.Options().Plugins[0].(*TestPlugin).AfterInitialized()).To(BeTrue())
	Expect(agent.Options().Plugins[0].(*TestPlugin).Closed()).To(BeFalse())

	err = agent.Stop()
	Expect(err).To(HaveOccurred())
	Expect(agent.Options().Plugins[0].(*TestPlugin).Closed()).To(BeTrue())
}

func TestAgentWithPluginWait(t *testing.T) {
	RegisterTestingT(t)
	agent := agent.NewAgent(agent.Plugins(&TestPlugin{}))
	err := agent.Start()
	Expect(err).To(BeNil())
	waitOverCh := make(chan struct{})
	go func() {
		err = agent.Wait()
		Expect(err).To(BeNil())
		close(waitOverCh)
	}()
	time.Sleep(100 * time.Millisecond) // Make sure we aren't racing the go routing
	select {
	case <-waitOverCh:
		Expect(false).To(BeTrue()) // Fail if we get here at all because waitOverCh should not be closed
	default:
	}
	Expect(agent.Stop()).To(BeNil())
	time.Sleep(100 * time.Millisecond) // Make sure we aren't racing the go routing
	_, channelOpen := <-waitOverCh
	Expect(channelOpen).To(BeFalse()) // ie, the wait returned

}

func TestAgentWithPluginsRun(t *testing.T) {
	RegisterTestingT(t)
	agent := agent.NewAgent(agent.Plugins(&TestPlugin{}))
	waitOverCh := make(chan struct{})
	go func() {
		err := agent.Run()
		Expect(err).To(BeNil())
		close(waitOverCh)
	}()
	time.Sleep(100 * time.Millisecond) // Make sure we aren't racing the go routing
	select {
	case <-waitOverCh:
		Expect(false).To(BeTrue()) // Fail if we get here at all because waitOverCh should not be closed
	default:
	}
	Expect(agent.Stop()).To(BeNil())
	time.Sleep(100 * time.Millisecond) // Make sure we aren't racing the go routing
	_, channelOpen := <-waitOverCh
	Expect(channelOpen).To(BeFalse()) // ie, the wait returned
}

func TestAgentWithPluginsRunFail(t *testing.T) {
	RegisterTestingT(t)
	agent := agent.NewAgent(agent.Plugins(NewTestPlugin(true, false, false)))
	waitOverCh := make(chan struct{})
	go func() {
		err := agent.Run()
		Expect(err).ToNot(BeNil())
		close(waitOverCh)
	}()
	select {
	case <-waitOverCh:
	case <-time.After(100 * time.Millisecond):
		Expect(false).To(BeTrue()) // Fail if we get here at all because waitOverCh should already be closed
	}
}

func TestAgentWithPluginsReceiveSignal(t *testing.T) {
	RegisterTestingT(t)
	agent := agent.NewAgent(agent.Plugins(&TestPlugin{}))
	err := agent.Start()
	Expect(err).To(BeNil())
	wg := sync.WaitGroup{}
	wg.Add(1) //  We have one go routine we are about to launch
	logrus.DefaultLogger().Info("wg.Add(1)")
	go func() {
		logrus.DefaultLogger().Info("entering goroutine")
		defer wg.Done() // Release the wg when we are done
		err := agent.Wait()
		Expect(err).To(BeNil())
		logrus.DefaultLogger().Info("exiting goroutine")
	}()

	// Send the signal

	syscall.Kill(os.Getpid(), syscall.SIGINT)
	logrus.DefaultLogger().Info("wg.Wait()")
	wg.Wait()
}

func TestAgentWithNamedPlugin(t *testing.T) {
	RegisterTestingT(t)
	p := NewTestPlugin(false, false, false)
	p.SetName(defaultPluginName)
	agent := agent.NewAgent(agent.Plugins(p))
	Expect(agent).NotTo(BeNil())
	Expect(agent.Options()).NotTo(BeNil())
	Expect(agent.Options().Plugins).ToNot(BeNil())
	Expect(len(agent.Options().Plugins)).To(Equal(1))
	Expect(agent.Options().Plugins[0]).To(Equal(p))
	err := agent.Start()
	Expect(err).To(BeNil())
	err = agent.Stop()
	Expect(err).To(BeNil())
}

func TestAgentWithPluginNoAfterInit(t *testing.T) {
	RegisterTestingT(t)
	p := &TestPluginNoAfterInit{}
	agent := agent.NewAgent(agent.Plugins(p))
	Expect(agent).NotTo(BeNil())
	Expect(agent.Options()).NotTo(BeNil())
	Expect(agent.Options().Plugins).ToNot(BeNil())
	Expect(len(agent.Options().Plugins)).To(Equal(1))
	Expect(agent.Options().Plugins[0]).To(Equal(p))
	err := agent.Start()
	Expect(err).To(BeNil())
	err = agent.Stop()
	Expect(err).To(BeNil())
}

// Define the TestPluginNoAfterInit we will use for testing

type TestPluginNoAfterInit struct{}

func (*TestPluginNoAfterInit) Close() error {
	return nil
}

func (*TestPluginNoAfterInit) Init() error {
	return nil
}

func (*TestPluginNoAfterInit) String() string {
	return defaultPluginName
}

// Define the TestPlugin we will use for testing

type TestPlugin struct {
	failInit      bool
	failAfterInit bool
	failClose     bool

	sync.Mutex
	initCalled      bool
	afterInitCalled bool
	closeCalled     bool

	infra.PluginName
}

func NewTestPlugin(failInit, failAfterInit, failClose bool) *TestPlugin {
	return &TestPlugin{
		failInit:      failInit,
		failAfterInit: failAfterInit,
		failClose:     failClose,
	}
}

func (p *TestPlugin) Init() error {
	p.Lock()
	defer p.Unlock()
	p.initCalled = true
	if p.failInit {
		return fmt.Errorf(initFailedErrorString)
	}
	return nil
}
func (p *TestPlugin) AfterInit() error {
	p.Lock()
	defer p.Unlock()
	p.afterInitCalled = true
	if p.failAfterInit {
		return fmt.Errorf(afterInitFailedErrorString)
	}
	return nil
}
func (p *TestPlugin) Close() error {
	p.Lock()
	defer p.Unlock()
	p.closeCalled = true
	if p.failClose {
		return fmt.Errorf(closeFailedErrorString)
	}
	return nil
}

func (p *TestPlugin) Initialized() bool {
	p.Lock()
	defer p.Unlock()
	return p.initCalled
}

func (p *TestPlugin) AfterInitialized() bool {
	p.Lock()
	defer p.Unlock()
	return p.afterInitCalled
}

func (p *TestPlugin) Closed() bool {
	p.Lock()
	defer p.Unlock()
	return p.closeCalled
}
