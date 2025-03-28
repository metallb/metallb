// SPDX-License-Identifier:Apache-2.0
package main

import (
	"fmt"
	"os"
	"path/filepath"

	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"
)

// NodeExclusionPattern represents patterns of labels and annotations to exclude.
type NodeExclusionPattern struct {
	LabelsToExclude     map[string]string `yaml:"labelsToExclude"`
	AnnotationToExclude map[string]string `yaml:"annotationToExclude"`
}

// String returns a string representation for logging.
func (p *NodeExclusionPattern) String() string {
	if p == nil {
		return "NodeExclusionPattern<nil>"
	}
	return fmt.Sprintf("NodeExclusionPattern{LabelsToExclude:%+v, AnnotationToExclude:%+v}",
		p.LabelsToExclude, p.AnnotationToExclude)
}

// Match checks if the given node's labels or annotations match any exclusion patterns.
func (p *NodeExclusionPattern) Match(node *v1.Node) bool {
	if p == nil || node == nil {
		return false
	}

	for key, value := range p.LabelsToExclude {
		if val, exists := node.Labels[key]; exists && val == value {
			return true
		}
	}

	for key, value := range p.AnnotationToExclude {
		if val, exists := node.Annotations[key]; exists && val == value {
			return true
		}
	}

	return false
}

func parseExcludeNodePattern(file string) (*NodeExclusionPattern, error) {
	data, err := os.ReadFile(filepath.Clean(file))
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	var ret *NodeExclusionPattern
	if err := yaml.Unmarshal(data, &ret); err != nil {
		return nil, fmt.Errorf("parsing file %s: %w ", file, err)
	}
	return ret, nil
}
