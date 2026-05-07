package rules

import "testing"

func TestLoadGenericPreset(t *testing.T) {
	ruleSet, err := Load("generic", "")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if len(ruleSet.Handlers.PackageNames) == 0 {
		t.Fatal("generic preset has no handler package names")
	}
	if len(ruleSet.Classifiers) == 0 {
		t.Fatal("generic preset has no classifiers")
	}
	if len(ruleSet.IgnoreCalls.Packages) == 0 {
		t.Fatal("generic preset has no ignored packages")
	}
	if !ruleSet.IgnoreCalls.AutoStdlib {
		t.Fatal("generic preset does not auto-ignore standard library packages")
	}
}

func TestParseYAMLMergesConfigShape(t *testing.T) {
	ruleSet, err := parseYAML(`
handlers:
  package_names:
    - transport
ignore_calls:
  auto_stdlib: true
  packages:
    - log
  symbols:
    - helper.Noop
classifiers:
  - layer: gateway
    path_contains:
      - /gateway/
implementation:
  mock_receiver_prefixes:
    - Fake
`)
	if err != nil {
		t.Fatalf("parseYAML returned error: %v", err)
	}
	if got := ruleSet.Handlers.PackageNames[0]; got != "transport" {
		t.Fatalf("package name = %q", got)
	}
	if got := ruleSet.IgnoreCalls.Packages[0]; got != "log" {
		t.Fatalf("ignore package = %q", got)
	}
	if !ruleSet.IgnoreCalls.AutoStdlib {
		t.Fatal("auto_stdlib = false, want true")
	}
	if got := ruleSet.Classifiers[0].Layer; got != "gateway" {
		t.Fatalf("classifier layer = %q", got)
	}
	if got := ruleSet.Implementation.MockReceiverPrefixes[0]; got != "Fake" {
		t.Fatalf("mock receiver prefix = %q", got)
	}
}
