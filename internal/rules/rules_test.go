package rules

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadUsesGenericPresetWithoutConfig(t *testing.T) {
	ruleSet, err := Load("")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if len(ruleSet.Handlers.Match.PackageNames) == 0 {
		t.Fatal("generic preset has no handler package names")
	}
	if len(ruleSet.Layers) == 0 {
		t.Fatal("generic preset has no layers")
	}
	if len(ruleSet.Ignore.Calls.PackageNames) == 0 {
		t.Fatal("generic preset has no ignored packages")
	}
	if !ruleSet.Ignore.StandardLibrary {
		t.Fatal("generic preset does not auto-ignore standard library packages")
	}
}

func TestLoadConfigReplacesGenericPreset(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".calltrail.yaml")
	err := os.WriteFile(configPath, []byte(`
version: 1
handlers:
  match:
    package_names:
      - transport
    file_path_contains:
      - /transport/
  signature:
    require_error_return: true
layers:
  - name: gateway
    match:
      file_path_contains:
        - /gateway/
ignore:
  standard_library: false
`), 0o644)
	if err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	ruleSet, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if got := ruleSet.Handlers.Match.PackageNames; len(got) != 1 || got[0] != "transport" {
		t.Fatalf("package names = %#v, want [transport]", got)
	}
	if contains(ruleSet.Handlers.Match.PackageNames, "grpc") {
		t.Fatalf("config should replace generic preset, got package names %#v", ruleSet.Handlers.Match.PackageNames)
	}
	if ruleSet.Ignore.StandardLibrary {
		t.Fatal("standard_library = true, want false from config")
	}
	if got := ruleSet.Layers[0].Name; got != "gateway" {
		t.Fatalf("layer name = %q, want gateway", got)
	}
}

func TestLoadExampleConfig(t *testing.T) {
	ruleSet, err := Load(filepath.Join("..", "..", "calltrail.example.yaml"))
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if len(ruleSet.Layers) == 0 {
		t.Fatal("example config has no layers")
	}
	if !ruleSet.Ignore.StandardLibrary {
		t.Fatal("example config should auto-ignore standard library package calls")
	}
	if got := ruleSet.Resolution.SkipImplementations.FilePathContains; !contains(got, "/mock/") {
		t.Fatalf("skip implementation paths = %#v, want /mock/", got)
	}
}

func TestParseYAMLConfigShape(t *testing.T) {
	ruleSet, err := parseYAML(`
version: 1
handlers:
  match:
    package_names:
      - transport
    file_path_contains:
      - /transport/
  signature:
    require_context_first_arg: true
    require_pointer_request: true
    require_pointer_response: true
    require_error_return: true
ignore:
  standard_library: true
  calls:
    package_names:
      - log
    full_names:
      - helper.Noop
  getters:
    local_values: true
    receiver_names:
      - req
layers:
  - name: gateway
    match:
      call_name_contains:
        - gateway
      receiver_type_contains:
        - Gateway
      file_path_contains:
        - /gateway/
      method_name_prefixes:
        - convert
      method_name_contains:
        - topb
resolution:
  skip_implementations:
    receiver_name_prefixes:
      - Fake
    file_path_contains:
      - /testdata/
`)
	if err != nil {
		t.Fatalf("parseYAML returned error: %v", err)
	}
	if got := ruleSet.Handlers.Match.PackageNames[0]; got != "transport" {
		t.Fatalf("package name = %q", got)
	}
	if !ruleSet.Handlers.Signature.RequireErrorReturn {
		t.Fatal("require_error_return = false, want true")
	}
	if got := ruleSet.Ignore.Calls.PackageNames[0]; got != "log" {
		t.Fatalf("ignore package = %q", got)
	}
	if !ruleSet.Ignore.StandardLibrary {
		t.Fatal("standard_library = false, want true")
	}
	if got := ruleSet.Ignore.Getters.ReceiverNames[0]; got != "req" {
		t.Fatalf("getter receiver = %q", got)
	}
	if got := ruleSet.Layers[0].Name; got != "gateway" {
		t.Fatalf("layer name = %q", got)
	}
	if got := ruleSet.Resolution.SkipImplementations.ReceiverNamePrefixes[0]; got != "Fake" {
		t.Fatalf("skip receiver prefix = %q", got)
	}
}

func TestParseYAMLRejectsOldConfigShape(t *testing.T) {
	_, err := parseYAML(`
version: 1
classifiers:
  repositories:
    symbol_contains:
      - repository
ignore_calls:
  packages:
    - log
`)
	if err == nil {
		t.Fatal("parseYAML returned nil error for old config shape")
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
