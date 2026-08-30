package config_test

import (
	"os"
	"testing"

	"gopkg.in/yaml.v2"
)

func TestTaskfile_ValidYAML(t *testing.T) {
	data, err := os.ReadFile("../../Taskfile.yml")
	if err != nil {
		t.Fatalf("failed to read Taskfile.yml: %v", err)
	}

	var parsed map[string]any
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Taskfile.yml contains invalid YAML: %v", err)
	}
}
