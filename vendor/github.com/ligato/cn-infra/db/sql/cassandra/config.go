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

package cassandra

import (
	"strings"
	"time"

	"strconv"

	"github.com/gocql/gocql"
)

//TLS used to configure TLS
type TLS struct {
	Certfile               string `json:"cert_path"`                // client certificate
	Keyfile                string `json:"key_path"`                 // client private key
	CAfile                 string `json:"ca_path"`                  // certificate authority
	EnableHostVerification bool   `json:"enable_host_verification"` // whether to skip verification of server name & certificate
	Enabled                bool   `json:"enabled"`                  // enable/disable TLS
}

// Config Configuration for Cassandra clients loaded from a configuration file
type Config struct {
	// A list of host addresses of cluster nodes.
	Endpoints []string `json:"endpoints"`

	// port for Cassandra (default: 9042)
	Port int `json:"port"`

	// session timeout (default: 600ms)
	OpTimeout time.Duration `json:"op_timeout"`

	// initial session timeout, used during initial dial to server (default: 600ms)
	DialTimeout time.Duration `json:"dial_timeout"`

	// If not zero, gocql attempt to reconnect known DOWN nodes in every ReconnectSleep.
	RedialInterval time.Duration `json:"redial_interval"`

	// ProtoVersion sets the version of the native protocol to use, this will
	// enable features in the driver for specific protocol versions, generally this
	// should be set to a known version (2,3,4) for the cluster being connected to.
	//
	// If it is 0 or unset (the default) then the driver will attempt to discover the
	// highest supported protocol for the cluster. In clusters with nodes of different
	// versions the protocol selected is not defined (ie, it can be any of the supported in the cluster)
	ProtocolVersion int `json:"protocol_version"`

	//TLS used to configure TLS
	TLS TLS `json:"tls"`
}

// ClientConfig wrapping gocql ClusterConfig
type ClientConfig struct {
	*gocql.ClusterConfig
}

const defaultOpTimeout = 600 * time.Millisecond
const defaultDialTimeout = 600 * time.Millisecond
const defaultRedialInterval = 60 * time.Second
const defaultProtocolVersion = 4

// ConfigToClientConfig transforms the yaml configuration into ClientConfig.
// If the configuration of endpoints is invalid, error ErrInvalidEndpointConfig
// is returned.
func ConfigToClientConfig(ymlConfig *Config) (*ClientConfig, error) {

	timeout := defaultOpTimeout
	if ymlConfig.OpTimeout > 0 {
		timeout = ymlConfig.OpTimeout
	}

	connectTimeout := defaultDialTimeout
	if ymlConfig.DialTimeout > 0 {
		connectTimeout = ymlConfig.DialTimeout
	}

	reconnectInterval := defaultRedialInterval
	if ymlConfig.RedialInterval > 0 {
		reconnectInterval = ymlConfig.RedialInterval
	}

	protoVersion := defaultProtocolVersion
	if ymlConfig.ProtocolVersion > 0 {
		protoVersion = ymlConfig.ProtocolVersion
	}

	endpoints, port, err := getEndpointsAndPort(ymlConfig.Endpoints)
	if err != nil {
		return nil, err
	}

	var sslOpts *gocql.SslOptions
	if ymlConfig.TLS.Enabled {
		sslOpts = &gocql.SslOptions{
			CaPath:                 ymlConfig.TLS.CAfile,
			CertPath:               ymlConfig.TLS.Certfile,
			KeyPath:                ymlConfig.TLS.Keyfile,
			EnableHostVerification: ymlConfig.TLS.EnableHostVerification,
		}
	}

	clientConfig := &gocql.ClusterConfig{
		Hosts:             endpoints,
		Port:              port,
		Timeout:           timeout * time.Millisecond,
		ConnectTimeout:    connectTimeout * time.Millisecond,
		ReconnectInterval: reconnectInterval * time.Second,
		ProtoVersion:      protoVersion,
		SslOpts:           sslOpts,
	}

	cfg := &ClientConfig{ClusterConfig: clientConfig}

	return cfg, nil
}

// CreateSessionFromConfig creates and initializes the cluster based on the supplied config
// and returns a new session object that can be used to interact with the database.
// The function propagates errors returned from gocql.CreateSession().
func CreateSessionFromConfig(config *ClientConfig) (*gocql.Session, error) {

	gocqlClusterConfig := gocql.NewCluster(HostsAsString(config.Hosts))
	gocqlClusterConfig.Port = config.Port
	gocqlClusterConfig.ConnectTimeout = config.ConnectTimeout
	gocqlClusterConfig.ReconnectInterval = config.ReconnectInterval
	gocqlClusterConfig.Timeout = config.Timeout
	gocqlClusterConfig.ProtoVersion = config.ProtoVersion
	gocqlClusterConfig.SslOpts = config.SslOpts

	session, err := gocqlClusterConfig.CreateSession()

	if err != nil {
		return nil, err
	}

	return session, nil
}

// HostsAsString converts an array of hosts addresses into a comma separated string
func HostsAsString(hostArr []string) string {
	return strings.Join(hostArr, ",")
}

//getEndpointsAndPort does string manipulation to extract []endpoints and port eg: "127.0.0.1:9042" or "127.0.0.1:9042,127.0.0.2:9042"
func getEndpointsAndPort(endpoints []string) (endpointsR []string, portR int, err error) {
	var resultEndpoints []string
	var resultPort int

	if len(endpoints) > 1 {
		return nil, 0, ErrInvalidEndpointConfig
	}

	if len(endpoints[0]) > 0 {
		v := endpoints[0]

		if !strings.Contains(v, ":") {
			return nil, 0, ErrInvalidEndpointConfig
		}

		if strings.Contains(v, ",") {
			endpointsAndPort := strings.Split(v, ",")
			for _, val := range endpointsAndPort {
				endpointAndPort := strings.Split(val, ":")
				resultEndpoints = append(resultEndpoints, endpointAndPort[0])
				resultPort, err = strconv.Atoi(endpointAndPort[1])
				if err != nil {
					return nil, 0, err
				}
			}

		} else {
			endpointAndPort := strings.Split(v, ":")
			resultEndpoints = append(resultEndpoints, endpointAndPort[0])
			resultPort, err = strconv.Atoi(endpointAndPort[1])
			if err != nil {
				return nil, 0, err
			}
		}
	} else {
		return nil, 0, ErrInvalidEndpointConfig
	}

	return resultEndpoints, resultPort, nil
}
