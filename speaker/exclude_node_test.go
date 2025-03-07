// SPDX-License-Identifier:Apache-2.0
package main

import (
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var validContent = `
labelsToExclude:
  foo: bar
annotationToExclude:
  zig: zag
`

func TestParseExcludeNodePattern(t *testing.T) {
	validFile, cleanup := createTempExcludeFile(t, validContent)
	defer cleanup()
	invalidFile, cleanup := createTempExcludeFile(t, "}{")
	defer cleanup()

	tests := map[string]struct {
		input     string
		want      *NodeExclusionPattern
		wantError bool
	}{
		"valid file": {
			input: validFile,
			want: &NodeExclusionPattern{
				LabelsToExclude:     map[string]string{"foo": "bar"},
				AnnotationToExclude: map[string]string{"zig": "zag"},
			},
		},
		"file doesn't exist returns no error": {
			input: "/this/path/does/not/exist/file.txt",
		},
		"invalid file returns error": {
			input:     invalidFile,
			wantError: true,
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			got, err := parseExcludeNodePattern(tc.input)
			if err != nil && !tc.wantError {
				t.Errorf("error=%v, want %v", got, nil)
			}
			if !cmp.Equal(tc.want, got) {
				t.Errorf("got=%v, want %v", got, tc.want)
			}
		})
	}
}

func TestNodeExclusionPatternMatch(t *testing.T) {
	obj := &NodeExclusionPattern{
		LabelsToExclude:     map[string]string{"foo": "bar"},
		AnnotationToExclude: map[string]string{"zig": "zag"},
	}

	tests := map[string]struct {
		object *NodeExclusionPattern
		input  *corev1.Node
		want   bool
	}{
		"false when nil receiver": {
			object: nil,
			input:  nil,
			want:   false,
		},
		"false when nil input": {
			input: nil,
			want:  false,
		},
		"match node label": {
			object: obj,
			input: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"foo": "bar"}},
			},
			want: true,
		},
		"match node annotation": {
			object: obj,
			input: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{"zig": "zag"}},
			},
			want: true,
		},
		"no match": {
			object: obj,
			input: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{"foo": "zag"}},
			},
			want: false,
		},
	}
	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			got := tc.object.Match(tc.input)
			if got != tc.want {
				t.Errorf("Wrong match, want %v, got %v", tc.want, got)
			}
		})
	}
}

func createTempExcludeFile(t *testing.T, content string) (string, func()) {
	t.Helper()
	tmpFile, err := os.CreateTemp("", "exclude-pattern-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	if _, err := tmpFile.Write([]byte(content)); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("Failed to close temp file: %v", err)
	}

	cleanup := func() {
		os.Remove(tmpFile.Name())
	}

	return tmpFile.Name(), cleanup
}
