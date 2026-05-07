package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
		"calltrail-go " + dir + " --list",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("stderr does not contain %q:\n%s", want, got)
		}
	}
}
