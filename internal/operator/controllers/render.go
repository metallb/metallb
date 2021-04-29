/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"bytes"
	"context"
	"github.com/pkg/errors"
	"io/ioutil"
	uns "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
	"text/template"
)

func renderConfig(ctx context.Context, configMapFile string, data map[string]interface{}) (uns.Unstructured, error) {
	obj := uns.Unstructured{}
	source, err := ioutil.ReadFile(configMapFile)
	if err != nil {
		return obj, errors.Wrapf(err, "failed to read manifest %s", configMapFile)
	}

	tmpl := template.New(configMapFile).Option("missingkey=error")
	if _, err := tmpl.Parse(string(source)); err != nil {
		return obj, errors.Wrapf(err, "failed to parse manifest %s as template", configMapFile)
	}

	rendered := bytes.Buffer{}
	if err := tmpl.Execute(&rendered, data); err != nil {
		return obj, errors.Wrapf(err, "failed to render manifest %s", configMapFile)
	}

	decoder := yaml.NewYAMLOrJSONDecoder(&rendered, 4096)
	if err := decoder.Decode(&obj); err != nil {
		return obj, errors.Wrapf(err, "failed to unmarshal manifest %s", configMapFile)
	}

	return obj, err
}

func mergeObjects(obj1 *uns.Unstructured, obj2 *uns.Unstructured) (*uns.Unstructured, error) {
	s1, ok, err := uns.NestedStringMap(obj1.Object, "data")
	if ok == false || err != nil {
		return nil, err
	}

	s2, ok, err := uns.NestedStringMap(obj2.Object, "data")
	if ok == false || err != nil {
		return nil, err
	}

	for k, v := range s2 {
		s1[k] += v
	}

	err = uns.SetNestedStringMap(obj1.Object, s1, "data")

	return obj1, err
}
