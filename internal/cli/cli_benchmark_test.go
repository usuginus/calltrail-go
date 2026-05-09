package cli

import (
	"io"
	"testing"
)

func BenchmarkRunListMapDispatch(b *testing.B) {
	args := []string{
		"../../examples/map-dispatch",
		"--config", "../../examples/map-dispatch/.calltrail.yaml",
		"--list",
	}
	for i := 0; i < b.N; i++ {
		if err := Run(args, io.Discard, io.Discard); err != nil {
			b.Fatalf("Run returned error: %v", err)
		}
	}
}

func BenchmarkRunMarkdownMapDispatch(b *testing.B) {
	args := []string{
		"../../examples/map-dispatch",
		"--config", "../../examples/map-dispatch/.calltrail.yaml",
		"--rpc", "ProcessDocument",
		"--depth", "4",
	}
	for i := 0; i < b.N; i++ {
		if err := Run(args, io.Discard, io.Discard); err != nil {
			b.Fatalf("Run returned error: %v", err)
		}
	}
}

func BenchmarkRunJSONMapDispatch(b *testing.B) {
	args := []string{
		"../../examples/map-dispatch",
		"--config", "../../examples/map-dispatch/.calltrail.yaml",
		"--rpc", "ProcessDocument",
		"--depth", "4",
		"--format", "json",
	}
	for i := 0; i < b.N; i++ {
		if err := Run(args, io.Discard, io.Discard); err != nil {
			b.Fatalf("Run returned error: %v", err)
		}
	}
}
