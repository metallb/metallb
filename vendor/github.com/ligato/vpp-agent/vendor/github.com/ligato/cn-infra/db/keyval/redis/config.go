// Copyright (c) 2017 Cisco and/or its affiliates.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at:
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package redis

import (
	"io/ioutil"
	"time"

	"crypto/tls"
	"crypto/x509"
	"fmt"

	"github.com/coreos/etcd/pkg/tlsutil"
	"github.com/ghodss/yaml"
	goredis "github.com/go-redis/redis"
)

// TLS configures Transport layer security properties.
type TLS struct {
	Enabled    bool   `json:"enabled"`     // enable/disable TLS
	SkipVerify bool   `json:"skip-verify"` // whether to skip verification of server name & certificate
	Certfile   string `json:"cert-file"`   // client certificate
	Keyfile    string `json:"key-file"`    // client private key
	CAfile     string `json:"ca-file"`     // certificate authority
}

func createTLSConfig(config TLS) (*tls.Config, error) {
	var (
		cert *tls.Certificate
		cp   *x509.CertPool
		err  error
	)
	if config.Certfile != "" && config.Keyfile != "" {
		cert, err = tlsutil.NewCert(config.Certfile, config.Keyfile, nil)
		if err != nil {
			return nil, fmt.Errorf("tlsutil.NewCert() failed: %s", err)
		}
	}

	if config.CAfile != "" {
		cp, err = tlsutil.NewCertPool([]string{config.CAfile})
		if err != nil {
			return nil, fmt.Errorf("tlsutil.NewCertPool() failed: %s", err)
		}
	}

	tlsConfig := &tls.Config{
		MinVersion:         tls.VersionTLS10,
		InsecureSkipVerify: config.SkipVerify,
		RootCAs:            cp,
	}
	if cert != nil {
		tlsConfig.Certificates = []tls.Certificate{*cert}
	}

	return tlsConfig, nil
}

///////////////////////////////////////////////////////////////////////////////
// go-redis https://github.com/go-redis/redis

// GoRedisNil is error returned by go-redis when Redis replies with nil,
// .e.g. when key does not exist.
const GoRedisNil = goredis.Nil

// Client is common interface used to adapt all types of Redis clients.
type Client interface {
	// The easiest way to adapt Cmdable interface is just to embed it.
	goredis.Cmdable
	/*
		But that means we'll have to mock each and every method in Cmdable for
		unit tests, making it a whole lot more complicated.  When the time comes,
		it may be more manageable to only declare (duplicate) the methods
		we need from Cmdable.  As follows:
			Del(keys ...string) *goredis.IntCmd
			Get(key string) *goredis.StringCmd
			MGet(keys ...string) *goredis.SliceCmd
			MSet(pairs ...interface{}) *goredis.StatusCmd
			Scan(cursor uint64, match string, count int64) *goredis.ScanCmd
			Set(key string, value interface{}, expiration time.Duration) *goredis.StatusCmd
	*/

	// Declare these additional methods to enable access to them through this
	// interface.
	Close() error
	PSubscribe(channels ...string) *goredis.PubSub
}

// ClientConfig is a configuration common to all types of Redis clients.
type ClientConfig struct {
	// Password for authentication, if required.
	Password string `json:"password"`

	// Dial timeout for establishing new connections. Default is 5 seconds.
	DialTimeout time.Duration `json:"dial-timeout"`

	// Timeout for socket reads. If reached, commands will fail with a timeout
	// instead of blocking. Default is 3 seconds.
	ReadTimeout time.Duration `json:"read-timeout"`

	// Timeout for socket writes. If reached, commands will fail with a timeout
	// instead of blocking. Default is ReadTimeout.
	WriteTimeout time.Duration `json:"write-timeout"`

	// Connection pool configuration.
	Pool PoolConfig `json:"pool"`
}

// NodeConfig Node client configuration
type NodeConfig struct {
	// host:port address of a Redis node
	Endpoint string `json:"endpoint"`

	// Database to be selected after connecting to the server.
	DB int `json:"db"`

	// Enables read-only queries on slave nodes.
	EnableReadQueryOnSlave bool `json:"enable-query-on-slave"`

	// TLS configuration -- only applies to node client.
	TLS TLS `json:"tls"`

	// Embedded common client configuration.
	ClientConfig
}

// ClusterConfig Cluster client configuration
type ClusterConfig struct {
	// A seed list of host:port addresses of cluster nodes.
	Endpoints []string `json:"endpoints"`

	// Enables read-only queries on slave nodes.
	EnableReadQueryOnSlave bool `json:"enable-query-on-slave"`

	// The maximum number of redirects before giving up.
	// Command is retried on network errors and MOVED/ASK redirects. Default is 16.
	MaxRedirects int `json:"max-rediects"`
	// Allows routing read-only commands to the closest master or slave node.
	RouteByLatency bool `json:"route-by-latency"`

	ClientConfig
}

// SentinelConfig Sentinel client configuration
type SentinelConfig struct {
	// A seed list of host:port addresses sentinel nodes.
	Endpoints []string `json:"endpoints"`

	// The sentinel master name.
	MasterName string `json:"master-name"`

	// Database to be selected after connecting to the server.
	DB int `json:"db"`

	ClientConfig
}

// PoolConfig is a configuration of the go-redis connection pool.
type PoolConfig struct {
	// Maximum number of socket connections.
	// Default is 10 connections per every CPU as reported by runtime.NumCPU.
	PoolSize int `json:"max-connections"`
	// Amount of time, in seconds, a client waits for connection if all connections
	// are busy before returning an error.
	// Default is ReadTimeout + 1 second.
	PoolTimeout time.Duration `json:"busy-timeout"`
	// Amount of time, in seconds, after which a client closes idle connections.
	// Should be less than server's timeout.
	// Default is 5 minutes.
	IdleTimeout time.Duration `json:"idle-timeout"`
	// Frequency of idle checks.
	// Default is 1 minute.
	// When negative value is set, then idle check is disabled.
	IdleCheckFrequency time.Duration `json:"idle-check-frequency"`
}

// ConfigToClient creates an appropriate client according to the configuration
// parameter.
func ConfigToClient(config interface{}) (Client, error) {
	switch cfg := config.(type) {
	case NodeConfig:
		return CreateNodeClient(cfg)
	case ClusterConfig:
		return CreateClusterClient(cfg)
	case SentinelConfig:
		return CreateSentinelClient(cfg)
	case nil:
		return nil, fmt.Errorf("Configuration cannot be nil")
	}
	return nil, fmt.Errorf("Unknown configuration type %T", config)
}

// CreateNodeClient creates a client that will connect to a redis node,
// like master and/or slave.
func CreateNodeClient(config NodeConfig) (Client, error) {
	var tlsConfig *tls.Config
	if config.TLS.Enabled {
		var err error
		tlsConfig, err = createTLSConfig(config.TLS)
		if err != nil {
			return nil, err
		}
	}
	return goredis.NewClient(&goredis.Options{
		Network: "tcp",
		Addr:    config.Endpoint,

		// Database to be selected after connecting to the server
		DB: config.DB,

		// Enables read only queries on slave nodes.
		/*ReadOnly: config.EnableReadQueryOnSlave,*/

		// TLS Config to use. When set TLS will be negotiated.
		TLSConfig: tlsConfig,

		// Optional password. Must match the password specified in the requirepass server configuration option.
		Password: config.Password,

		// Dial timeout for establishing new connections. Default is 5 seconds.
		DialTimeout: config.DialTimeout,
		// Timeout for socket reads. If reached, commands will fail with a timeout instead of blocking. Default is 3 seconds.
		ReadTimeout: config.ReadTimeout,
		// Timeout for socket writes. If reached, commands will fail with a timeout instead of blocking. Default is ReadTimeout.
		WriteTimeout: config.WriteTimeout,

		// Maximum number of socket connections. Default is 10 connections per every CPU as reported by runtime.NumCPU.
		PoolSize: config.Pool.PoolSize,
		// Amount of time a client waits for connection if all connections are busy before returning an error. Default is ReadTimeout + 1 second.
		PoolTimeout: config.Pool.PoolTimeout,
		// Amount of time after which a client closes idle connections. Should be less than server's timeout. Default is 5 minutes.
		IdleTimeout: config.Pool.IdleTimeout,
		// Frequency of idle checks. Default is 1 minute. When negative value is set, then idle check is disabled.
		IdleCheckFrequency: config.Pool.IdleCheckFrequency,

		// Dialer creates new network connection and has priority over Network and Addr options.
		// Dialer func() (net.Conn, error)
		// Hook that is called when new connection is established
		// OnConnect func(*Conn) error

		// Maximum number of retries before giving up. Default is to not retry failed commands.
		MaxRetries: 0,
		// Minimum backoff between each retry. Default is 8 milliseconds; -1 disables backoff.
		MinRetryBackoff: 0,
		// Maximum backoff between each retry. Default is 512 milliseconds; -1 disables backoff.
		MaxRetryBackoff: 0,
	}), nil
}

// CreateClusterClient Creates a client that will connect to a redis cluster.
func CreateClusterClient(config ClusterConfig) (Client, error) {
	return goredis.NewClusterClient(&goredis.ClusterOptions{
		Addrs: config.Endpoints,

		// Enables read only queries on slave nodes.
		ReadOnly: config.EnableReadQueryOnSlave,

		MaxRedirects:   config.MaxRedirects,
		RouteByLatency: config.RouteByLatency,

		// Optional password. Must match the password specified in the requirepass server configuration option.
		Password: config.Password,

		// Dial timeout for establishing new connections. Default is 5 seconds.
		DialTimeout: config.DialTimeout,
		// Timeout for socket reads. If reached, commands will fail with a timeout instead of blocking. Default is 3 seconds.
		ReadTimeout: config.ReadTimeout,
		// Timeout for socket writes. If reached, commands will fail with a timeout instead of blocking. Default is ReadTimeout.
		WriteTimeout: config.WriteTimeout,

		// Maximum number of socket connections. Default is 10 connections per every CPU as reported by runtime.NumCPU.
		PoolSize: config.Pool.PoolSize,
		// Amount of time a client waits for connection if all connections are busy before returning an error. Default is ReadTimeout + 1 second.
		PoolTimeout: config.Pool.PoolTimeout,
		// Amount of time after which a client closes idle connections. Should be less than server's timeout. Default is 5 minutes.
		IdleTimeout: config.Pool.IdleTimeout,
		// Frequency of idle checks. Default is 1 minute. When negative value is set, then idle check is disabled.
		IdleCheckFrequency: config.Pool.IdleCheckFrequency,

		// Maximum number of retries before giving up. Default is to not retry failed commands.
		MaxRetries: 0,
		// Minimum backoff between each retry. Default is 8 milliseconds; -1 disables backoff.
		MinRetryBackoff: 0,
		// Maximum backoff between each retry. Default is 512 milliseconds; -1 disables backoff.
		MaxRetryBackoff: 0,

		// Hook that is called when new connection is established
		// OnConnect func(*Conn) error
	}), nil
}

// CreateSentinelClient Creates a failover client that will connect to redis sentinels.
func CreateSentinelClient(config SentinelConfig) (Client, error) {
	return goredis.NewFailoverClient(&goredis.FailoverOptions{
		SentinelAddrs: config.Endpoints,

		DB: config.DB,

		MasterName: config.MasterName,

		// Optional password. Must match the password specified in the requirepass server configuration option.
		Password: config.Password,

		// Dial timeout for establishing new connections. Default is 5 seconds.
		DialTimeout: config.DialTimeout,
		// Timeout for socket reads. If reached, commands will fail with a timeout instead of blocking. Default is 3 seconds.
		ReadTimeout: config.ReadTimeout,
		// Timeout for socket writes. If reached, commands will fail with a timeout instead of blocking. Default is ReadTimeout.
		WriteTimeout: config.WriteTimeout,

		// Maximum number of socket connections. Default is 10 connections per every CPU as reported by runtime.NumCPU.
		PoolSize: config.Pool.PoolSize,
		// Amount of time a client waits for connection if all connections are busy before returning an error. Default is ReadTimeout + 1 second.
		PoolTimeout: config.Pool.PoolTimeout,
		// Amount of time after which a client closes idle connections. Should be less than server's timeout. Default is 5 minutes.
		IdleTimeout: config.Pool.IdleTimeout,
		// Frequency of idle checks. Default is 1 minute. When negative value is set, then idle check is disabled.
		IdleCheckFrequency: config.Pool.IdleCheckFrequency,

		// Maximum number of retries before giving up. Default is to not retry failed commands.
		MaxRetries: 0,

		// Hook that is called when new connection is established
		// OnConnect func(*Conn) error
	}), nil
}

// LoadConfig Loads the given configFile and returns appropriate config instance.
func LoadConfig(configFile string) (cfg interface{}, err error) {
	b, err := ioutil.ReadFile(configFile)
	if err != nil {
		return nil, err
	}

	var s SentinelConfig
	err = yaml.Unmarshal(b, &s)
	if err != nil {
		return nil, err
	}
	if s.MasterName != "" {
		return s, nil
	}

	n := NodeConfig{}
	err = yaml.Unmarshal(b, &n)
	if err != nil {
		return nil, err
	}
	if n.Endpoint != "" {
		return n, nil
	}

	c := ClusterConfig{}
	err = yaml.Unmarshal(b, &c)
	if err != nil {
		return nil, err
	}
	return c, nil
}
