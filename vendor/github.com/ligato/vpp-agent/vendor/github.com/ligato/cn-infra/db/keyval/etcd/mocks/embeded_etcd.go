package mocks

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/embed"
	"github.com/coreos/etcd/etcdserver/api/v3client"
	"github.com/ligato/cn-infra/logging/logrus"
)

const etcdStartTimeout = 30

// Embedded ETCD instance with tmp directory for serialized key&vals and etcd client.
type Embedded struct {
	tmpDir string
	ETCD   *embed.Etcd
	client *clientv3.Client
}

// Start starts embedded ETCD.
func (embd *Embedded) Start(t *testing.T) {
	dir, err := ioutil.TempDir("", "ETCD")
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	cfg := embed.NewConfig()
	cfg.Dir = dir
	lpurl, _ := url.Parse("http://localhost:0")
	lcurl, _ := url.Parse("http://localhost:0")
	cfg.LPUrls = []url.URL{*lpurl}
	cfg.LCUrls = []url.URL{*lcurl}
	embd.ETCD, err = embed.StartEtcd(cfg)
	if err != nil {
		t.Error(err)
		t.FailNow()

	}

	select {
	case <-embd.ETCD.Server.ReadyNotify():
		logrus.DefaultLogger().Debug("Server is ready!")
	case <-time.After(etcdStartTimeout * time.Second):
		embd.ETCD.Server.Stop() // trigger a shutdown
		t.Error("Server took too long to start!")
		t.FailNow()
	}
	embd.client = v3client.New(embd.ETCD.Server)
}

// Stop stops the embedded ETCD & cleanups the tmp dir.
func (embd *Embedded) Stop() {
	embd.ETCD.Close()
	os.RemoveAll(embd.tmpDir)
}

// CleanDs deletes all stored key-value pairs.
func (embd *Embedded) CleanDs() {
	if embd.client != nil {
		resp, err := embd.client.Delete(context.Background(), "", clientv3.WithPrefix())
		if err != nil {
			panic(err)
		}
		fmt.Printf("resp: %+v\n", resp)
	}
}

// Client is a getter for embedded ETCD client.
func (embd *Embedded) Client() *clientv3.Client {
	return embd.client
}
