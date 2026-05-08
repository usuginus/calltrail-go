package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/usuginus/calltrail-go/internal/model"
)

func TestParseDefaultsForHumanReadableOutput(t *testing.T) {
	opts, err := Parse([]string{"./..."}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if opts.Format != "markdown" {
		t.Fatalf("format = %q, want markdown", opts.Format)
	}
	if opts.Depth != 3 {
		t.Fatalf("depth = %d, want 3", opts.Depth)
	}
	if len(opts.Paths) != 1 || opts.Paths[0] != "./..." {
		t.Fatalf("paths = %#v, want [./...]", opts.Paths)
	}
}

func TestParseListFlagBeforePath(t *testing.T) {
	opts, err := Parse([]string{"--list", "./..."}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if !opts.List {
		t.Fatal("list = false, want true")
	}
	if len(opts.Paths) != 1 || opts.Paths[0] != "./..." {
		t.Fatalf("paths = %#v, want [./...]", opts.Paths)
	}
}

func TestParseAllowsFlagsAfterPaths(t *testing.T) {
	opts, err := Parse([]string{"./...", "--rpc", "GetFoo", "--format", "json"}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if opts.RPC != "GetFoo" {
		t.Fatalf("rpc = %q, want GetFoo", opts.RPC)
	}
	if opts.Format != "json" {
		t.Fatalf("format = %q, want json", opts.Format)
	}
	if len(opts.Paths) != 1 || opts.Paths[0] != "./..." {
		t.Fatalf("paths = %#v, want [./...]", opts.Paths)
	}
}

func TestRunListOutputsDetectedHandlers(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run([]string{"../analyzer/testdata/simple", "--list"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run returned error: %v\nstderr:\n%s", err, stderr.String())
	}
	if got := stdout.String(); got != "GetFoo\tServer.GetFoo\tinternal/analyzer/testdata/simple/handler.go:43\n" {
		t.Fatalf("stdout = %q", got)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunJSONOutputsStructuredLayers(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run([]string{"../analyzer/testdata/simple", "--rpc", "GetFoo", "--depth", "2", "--format", "json"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run returned error: %v\nstderr:\n%s", err, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}

	var flows []model.APIFlow
	if err := json.Unmarshal(stdout.Bytes(), &flows); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v\nstdout:\n%s", err, stdout.String())
	}
	if len(flows) != 1 {
		t.Fatalf("len(flows) = %d, want 1", len(flows))
	}
	usecases := flows[0].Trail.LayerCalls("usecase")
	if len(usecases) != 2 {
		t.Fatalf("usecase calls = %#v, want 2 calls", usecases)
	}
	if got := flows[0].Trail.Layers[0].Name; got != "usecase" {
		t.Fatalf("first layer = %q, want usecase", got)
	}
	interfaceCalls := flows[0].Trail.InterfaceCalls
	if len(interfaceCalls) != 1 {
		t.Fatalf("interface calls = %#v, want 1", interfaceCalls)
	}
	if interfaceCalls[0].Interface != "FooUsecase" {
		t.Fatalf("interface = %q, want FooUsecase", interfaceCalls[0].Interface)
	}
	if got := interfaceCalls[0].Implementations[0].Call.Symbol; got != "fooUsecase.GetFoo" {
		t.Fatalf("implementation = %q, want fooUsecase.GetFoo", got)
	}
}

func TestRunJSONOutputsBranchDetails(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run([]string{
		"../../examples/branch-dispatch",
		"--config", "../../examples/branch-dispatch/.calltrail.yaml",
		"--rpc", "ProcessDocument",
		"--depth", "3",
		"--format", "json",
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run returned error: %v\nstderr:\n%s", err, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}

	var flows []model.APIFlow
	if err := json.Unmarshal(stdout.Bytes(), &flows); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v\nstdout:\n%s", err, stdout.String())
	}
	if len(flows) != 1 {
		t.Fatalf("len(flows) = %d, want 1", len(flows))
	}
	branches := flows[0].Trail.Branches
	if len(branches) != 2 {
		t.Fatalf("branches = %#v, want 2 branches", branches)
	}
	if branches[0].Kind != "type_switch" || branches[1].Kind != "switch" {
		t.Fatalf("branch kinds = %#v, want type_switch then switch", branches)
	}
	if len(branches[1].Cases) != 3 {
		t.Fatalf("switch cases = %#v, want 3 cases", branches[1].Cases)
	}
}

func TestRunJSONOutputsDispatchDetails(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run([]string{
		"../../examples/map-dispatch",
		"--config", "../../examples/map-dispatch/.calltrail.yaml",
		"--rpc", "ProcessDocument",
		"--depth", "4",
		"--format", "json",
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run returned error: %v\nstderr:\n%s", err, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}

	var flows []model.APIFlow
	if err := json.Unmarshal(stdout.Bytes(), &flows); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v\nstdout:\n%s", err, stdout.String())
	}
	if len(flows) != 1 {
		t.Fatalf("len(flows) = %d, want 1", len(flows))
	}
	dispatches := flows[0].Trail.Dispatches
	if len(dispatches) != 1 {
		t.Fatalf("dispatches = %#v, want 1 dispatch", dispatches)
	}
	if dispatches[0].Table != "a.processors" || dispatches[0].Key != "cmd.Kind" {
		t.Fatalf("dispatch lookup = %s[%s], want a.processors[cmd.Kind]", dispatches[0].Table, dispatches[0].Key)
	}
	if len(dispatches[0].Cases) != 2 {
		t.Fatalf("dispatch cases = %#v, want 2 cases", dispatches[0].Cases)
	}
}

func TestParseClampsInvalidDepth(t *testing.T) {
	var stderr bytes.Buffer
	opts, err := Parse([]string{"--depth", "0"}, &stderr)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if opts.Depth != 1 {
		t.Fatalf("depth = %d, want 1", opts.Depth)
	}
	if stderr.String() == "" {
		t.Fatal("expected warning for invalid depth")
	}
}

func TestFindConfigSearchesParentDirectories(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "internal", "driver", "grpc")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	configPath := filepath.Join(root, ".calltrail.yaml")
	if err := os.WriteFile(configPath, []byte("version: 1\n"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	got := FindConfig([]string{filepath.Join(child, "...")})
	if got != configPath {
		t.Fatalf("FindConfig = %q, want %q", got, configPath)
	}
}

func TestRunNoHandlersWritesDiagnostics(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package sample\nfunc helper() {}\n"), 0o644)
	if err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	err = Run([]string{dir}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	got := stderr.String()
	for _, want := range []string{
		"calltrail-go: no handlers found",
		"scanned_go_files: 1",
		"handler package_names: grpc",
		"rules: built-in generic",
		"calltrail-go " + dir + " --list",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("stderr does not contain %q:\n%s", want, got)
		}
	}
}
