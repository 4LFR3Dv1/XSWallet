package server

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/xeipuuv/gojsonschema"
)

func TestCanonicalNodeManagerManifestSchema(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("runtime.Caller failed")
	}
	coreDir := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))
	schemaPath := filepath.Join(coreDir, "config", "nodemanager.manifest.schema.json")
	manifestPath := filepath.Join(coreDir, "config", "nodemanager.manifest.json")

	if _, err := os.Stat(schemaPath); err != nil {
		t.Fatalf("schema not found at %s: %v", schemaPath, err)
	}
	if _, err := os.Stat(manifestPath); err != nil {
		t.Fatalf("manifest not found at %s: %v", manifestPath, err)
	}

	result, err := gojsonschema.Validate(
		gojsonschema.NewReferenceLoader("file://"+filepath.ToSlash(schemaPath)),
		gojsonschema.NewReferenceLoader("file://"+filepath.ToSlash(manifestPath)),
	)
	if err != nil {
		t.Fatalf("schema validation execution failed: %v", err)
	}
	if result.Valid() {
		return
	}
	for _, issue := range result.Errors() {
		t.Errorf("manifest schema issue: %s", issue)
	}
	t.Fatalf("canonical nodemanager manifest is invalid")
}
