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

package safeclose

import (
	"io"
	"reflect"
	"strings"

	"github.com/ligato/cn-infra/logging/logrus"
)

// safeClose closes closable object.
func safeClose(obj interface{}) error {
	defer func() {
		if r := recover(); r != nil {
			logrus.DefaultLogger().Error("Recovered in safeclose: ", r)
		}
	}()

	// closerWithoutErr is similar interface to io.Closer, but Close() does not return error
	type closerWithoutErr interface {
		Close()
	}

	if val := reflect.ValueOf(obj); val.IsValid() && !val.IsNil() {
		if closer, ok := obj.(*io.Closer); ok {
			if closer != nil {
				return (*closer).Close()
			}
		} else if closer, ok := obj.(io.Closer); ok {
			if closer != nil {
				return closer.Close()
			}
		} else if closer, ok := obj.(*closerWithoutErr); ok {
			if closer != nil {
				(*closer).Close()
			}
		} else if closer, ok := obj.(closerWithoutErr); ok {
			if closer != nil {
				closer.Close()
			}
		} else if reflect.TypeOf(obj).Kind() == reflect.Chan {
			val.Close()
		}

	}

	return nil
}

// Close tries to close all objects and return all errors using CloseErrors if there are any.
func Close(objs ...interface{}) error {
	errs := make([]error, len(objs))

	for i, obj := range objs {
		errs[i] = safeClose(obj)
	}

	for _, err := range errs {
		if err != nil {
			return CloseErrors(errs)
		}
	}

	return nil
}

// CloseAll tries to close all objects and return all errors (there are nils if there was no errors).
// DEPRECATED - use safeclose.Close(...) instead
func CloseAll(objs ...interface{}) ([]error, error) {
	logrus.DefaultLogger().Debugf("safeclose.CloseAll() is DEPRECATED! Please use safeclose.Close() instead")

	errs := make([]error, len(objs))

	for i, obj := range objs {
		errs[i] = Close(obj)
	}

	for _, err := range errs {
		if err != nil {
			return errs, CloseErrors(errs)
		}
	}

	return errs, nil
}

// CloseErrors merges multiple errors into single type for simpler use.
type CloseErrors []error

// Error implements error interface.
func (e CloseErrors) Error() string {
	var errMsgs []string

	for _, err := range []error(e) {
		if err == nil {
			continue
		}
		errMsgs = append(errMsgs, err.Error())
	}

	return strings.Join(errMsgs, ", ")
}
