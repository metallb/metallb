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

package rest

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"net"
	"net/http"
	"time"

	"github.com/ligato/cn-infra/logging"
)

// ListenAndServe starts a http server.
func ListenAndServe(config Config, handler http.Handler) (srv *http.Server, err error) {
	server := &http.Server{
		Addr:              config.Endpoint,
		ReadTimeout:       config.ReadTimeout,
		ReadHeaderTimeout: config.ReadHeaderTimeout,
		WriteTimeout:      config.WriteTimeout,
		IdleTimeout:       config.IdleTimeout,
		MaxHeaderBytes:    config.MaxHeaderBytes,
		Handler:           handler,
	}

	if len(config.ClientCerts) > 0 {
		// require client certificate
		caCertPool := x509.NewCertPool()

		for _, c := range config.ClientCerts {
			caCert, err := ioutil.ReadFile(c)
			if err != nil {
				return nil, err
			}
			caCertPool.AppendCertsFromPEM(caCert)
		}

		server.TLSConfig = &tls.Config{
			ClientAuth: tls.RequireAndVerifyClientCert,
			ClientCAs:  caCertPool,
		}
	}

	ln, err := net.Listen("tcp", server.Addr)
	if err != nil {
		return nil, err
	}
	l := tcpKeepAliveListener{ln.(*net.TCPListener)}

	go func() {
		var err error
		if config.UseHTTPS() {
			// if server certificate and key is configured use HTTPS
			err = server.ServeTLS(l, config.ServerCertfile, config.ServerKeyfile)
		} else {
			err = server.Serve(l)
		}
		// Serve always returns non-nil error
		logging.DefaultLogger.Debugf("HTTP server Serve: %v", err)
	}()

	return server, nil
}

// tcpKeepAliveListener sets TCP keep-alive timeouts on accepted
// connections. It's used by ListenAndServe and ListenAndServeTLS so
// dead TCP connections (e.g. closing laptop mid-download) eventually
// go away.
type tcpKeepAliveListener struct {
	*net.TCPListener
}

func (ln tcpKeepAliveListener) Accept() (net.Conn, error) {
	tc, err := ln.AcceptTCP()
	if err != nil {
		return nil, err
	}
	tc.SetKeepAlive(true)
	tc.SetKeepAlivePeriod(3 * time.Minute)
	return tc, nil
}
