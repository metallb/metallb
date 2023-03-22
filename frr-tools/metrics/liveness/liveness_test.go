// SPDX-License-Identifier:Apache-2.0

package liveness

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.universe.tf/metallb/internal/logging"
)

func TestLiveness(t *testing.T) {
	tests := []struct {
		desc               string
		vtyshRes           string
		vtyshError         error
		expectedStatusCode int
	}{
		{
			desc:               "regular",
			vtyshRes:           " zebra bgpd watchfrr staticd bfdd\n",
			expectedStatusCode: http.StatusOK,
		},
		{
			desc:               "returns error",
			vtyshError:         fmt.Errorf("failed to run"),
			expectedStatusCode: http.StatusInternalServerError,
		},
		{
			desc:               "less daemons",
			vtyshRes:           " zebra bgpd staticd bfdd\n",
			expectedStatusCode: http.StatusNotFound,
		},
	}

	logger, err := logging.Init("error")
	if err != nil {
		t.Fatalf("failed to create logger %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/livez", nil)
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			w := httptest.NewRecorder()
			vtysh := func(args string) (string, error) {
				return test.vtyshRes, test.vtyshError
			}
			handler := Handler(vtysh, logger)
			handler.ServeHTTP(w, req)
			res := w.Result()
			defer res.Body.Close()
			if res.StatusCode != test.expectedStatusCode {
				t.Errorf("status code %d different from expected %d", res.StatusCode, test.expectedStatusCode)
			}
		})
	}
}
