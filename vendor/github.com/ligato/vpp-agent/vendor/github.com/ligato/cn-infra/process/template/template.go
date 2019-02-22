// Copyright (c) 2018 Cisco and/or its affiliates.
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

//go:generate protoc --proto_path=model/process --gogo_out=model/process model/process/process.proto

package template

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/cn-infra/process/template/model/process"
	"github.com/pkg/errors"
)

// JSONExt - JSON extension
const JSONExt = ".json"

// DefaultMode defines default permission bits for template file
const DefaultMode = os.FileMode(0777)

// Reader reads/writes process templates to path
type Reader struct {
	log logging.Logger

	path string
}

// NewTemplateReader returns a new instance of template reader with given path. The path is also verified and
// crated if not existed
func NewTemplateReader(path string, log logging.Logger) (*Reader, error) {
	reader := &Reader{
		log:  log,
		path: path,
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.MkdirAll(path, DefaultMode); err != nil {
			return nil, errors.Errorf("cannot initialize template reader: path processing error: %v", err)
		}
	}
	return reader, nil
}

// GetAllTemplates reads all templates from reader's path. Error is returned if path is not a directory. All JSON
// file are read and un-marshaled as template objects
func (r *Reader) GetAllTemplates() ([]*process.Template, error) {
	var templates []*process.Template
	entries, err := ioutil.ReadDir(r.path)
	if err != nil {
		return templates, errors.Errorf("failed to open process template path %s: %v", r.path, err)
	}
	// From this point, log all errors bot continue reading
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if filepath.Ext(entry.Name()) != JSONExt {
			continue
		}
		filePath := filepath.Join(r.path, entry.Name())
		file, err := ioutil.ReadFile(filePath)
		if err != nil {
			r.log.Errorf("failed to read file %s: %v", filePath, err)
			continue
		}
		template := &process.Template{}
		if err := json.Unmarshal(file, template); err != nil {
			r.log.Errorf("failed to unmarshal file %s: %v", filePath, err)
			continue
		}

		templates = append(templates, template)
	}

	r.log.Debugf("read %d process template(s)", len(templates))

	return templates, nil
}

// WriteTemplate creates a new file (defined by template name) and writes it to the reader's path
func (r *Reader) WriteTemplate(template *process.Template, permission os.FileMode) error {
	if template == nil {
		return errors.Errorf("provided process template is nil")
	}

	templateData, err := json.Marshal(template)
	if err != nil {
		return errors.Errorf("failed to marshal template %s: %v", template.Name, err)
	}

	templateFile := template.Name + JSONExt
	filePath := filepath.Join(r.path, templateFile)
	err = ioutil.WriteFile(filePath, templateData, permission)
	if err != nil {
		return errors.Errorf("failed to write template %s to file: %v", templateFile, err)
	}

	r.log.Debugf("process template file %s written", templateFile)

	return nil
}
