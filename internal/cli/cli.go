package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/usuginus/calltrail-go/internal/analyzer"
	"github.com/usuginus/calltrail-go/internal/model"
	"github.com/usuginus/calltrail-go/internal/output"
	"github.com/usuginus/calltrail-go/internal/rules"
)

const (
	defaultDepth  = 3
	defaultFormat = "markdown"
)

var ErrHelp = errors.New("help requested")

type Options struct {
	Format string
	RPC    string
	Depth  int
	Config string
	List   bool
	Paths  []string
}

func Run(args []string, stdout io.Writer, stderr io.Writer) error {
	opts, err := Parse(args, stderr)
	if err != nil {
		if err != ErrHelp {
			fmt.Fprintf(stderr, "calltrail-go: %v\n", err)
		}
		return err
	}

	ruleSet, err := rules.Load(opts.Config)
	if err != nil {
		fmt.Fprintf(stderr, "calltrail-go: %v\n", err)
		return err
	}
	if opts.List {
		return runList(stdout, stderr, opts, ruleSet)
	}
	return runAnalyze(stdout, stderr, opts, ruleSet)
}

func runAnalyze(stdout io.Writer, stderr io.Writer, opts Options, ruleSet rules.RuleSet) error {
	flows, err := analyzer.Analyze(opts.Paths, analyzerOptions(opts, ruleSet))
	if err != nil {
		fmt.Fprintf(stderr, "calltrail-go: %v\n", err)
		return err
	}
	if len(flows) == 0 {
		writeNoResults(stderr, opts, ruleSet)
	}
	if err := rejectAmbiguousRPC(opts, flows); err != nil {
		fmt.Fprintf(stderr, "calltrail-go: %v\n", err)
		return err
	}
	return writeAnalysisOutput(stdout, stderr, opts, flows)
}

func runList(stdout io.Writer, stderr io.Writer, opts Options, ruleSet rules.RuleSet) error {
	flows, err := analyzer.DetectHandlers(opts.Paths, analyzerOptions(opts, ruleSet))
	if err != nil {
		fmt.Fprintf(stderr, "calltrail-go: %v\n", err)
		return err
	}
	if len(flows) == 0 {
		writeNoResults(stderr, opts, ruleSet)
	}
	return output.WriteList(stdout, flows)
}

func analyzerOptions(opts Options, ruleSet rules.RuleSet) analyzer.Options {
	return analyzer.Options{
		RPC:   opts.RPC,
		Depth: opts.Depth,
		Rules: ruleSet,
	}
}

func rejectAmbiguousRPC(opts Options, flows []model.APIFlow) error {
	if opts.RPC == "" || strings.Contains(opts.RPC, ".") || len(flows) <= 1 {
		return nil
	}

	symbols := make([]string, 0, len(flows))
	for _, flow := range flows {
		symbols = append(symbols, flow.Entrypoint.Symbol)
	}
	sort.Strings(symbols)

	return fmt.Errorf(
		"ambiguous --rpc %q matched %d handlers; use one of: %s",
		opts.RPC,
		len(flows),
		strings.Join(symbols, ", "),
	)
}

func writeAnalysisOutput(stdout io.Writer, stderr io.Writer, opts Options, flows []model.APIFlow) error {
	switch opts.Format {
	case "json":
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(flows); err != nil {
			fmt.Fprintf(stderr, "calltrail-go: encode json: %v\n", err)
			return err
		}
	case "markdown", "md":
		if err := output.WriteMarkdown(stdout, flows); err != nil {
			fmt.Fprintf(stderr, "calltrail-go: write markdown: %v\n", err)
			return err
		}
	default:
		err := fmt.Errorf("unsupported format %q; use json or markdown", opts.Format)
		fmt.Fprintf(stderr, "calltrail-go: %v\n", err)
		return err
	}
	return nil
}

func Parse(args []string, stderr io.Writer) (Options, error) {
	var opts Options
	fs := flag.NewFlagSet("calltrail-go", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&opts.Format, "format", defaultFormat, "output format: markdown or json")
	fs.StringVar(&opts.RPC, "rpc", "", "filter by RPC/API handler name or receiver-qualified symbol")
	fs.IntVar(&opts.Depth, "depth", defaultDepth, "call extraction depth")
	fs.StringVar(&opts.Config, "config", "", "path to .calltrail.yaml")
	fs.BoolVar(&opts.List, "list", false, "list detected handlers and exit")
	fs.Usage = func() {
		fmt.Fprint(stderr, usageText())
		fs.PrintDefaults()
	}

	if err := fs.Parse(normalizeArgs(args)); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return Options{}, ErrHelp
		}
		return Options{}, err
	}
	if opts.Depth < 1 {
		fmt.Fprintln(stderr, "warning: --depth must be greater than 0; using 1")
		opts.Depth = 1
	}
	opts.Paths = fs.Args()
	if len(opts.Paths) == 0 {
		opts.Paths = []string{"."}
	}
	if opts.Config == "" {
		opts.Config = FindConfig(opts.Paths)
	}
	return opts, nil
}

func usageText() string {
	return `Usage:
  calltrail-go [flags] [path ...]

Examples:
  calltrail-go ./...
  calltrail-go ./... --rpc GetFoo
  calltrail-go ./... --rpc Server.GetFoo
  calltrail-go ./... --rpc GetFoo --depth 5
  calltrail-go ./... --list
  calltrail-go ./... --format json
  calltrail-go ./... --config .calltrail.yaml

Flags:
`
}

func FindConfig(paths []string) string {
	for _, path := range paths {
		root := strings.TrimSuffix(path, "/...")
		if root == "" {
			root = "."
		}
		if info, err := stat(root); err == nil && !info.IsDir() {
			root = filepath.Dir(root)
		}
		abs, err := filepath.Abs(root)
		if err != nil {
			continue
		}
		for {
			candidate := filepath.Join(abs, ".calltrail.yaml")
			if _, err := stat(candidate); err == nil {
				return candidate
			}
			parent := filepath.Dir(abs)
			if parent == abs {
				break
			}
			abs = parent
		}
	}
	return ""
}

var stat = os.Stat

func normalizeArgs(args []string) []string {
	var flags []string
	var paths []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "-") {
			paths = append(paths, arg)
			continue
		}
		flags = append(flags, arg)
		if strings.Contains(arg, "=") || !flagTakesValue(arg) {
			continue
		}
		if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
			flags = append(flags, args[i+1])
			i++
		}
	}
	return append(flags, paths...)
}

func flagTakesValue(arg string) bool {
	name := strings.TrimLeft(arg, "-")
	if idx := strings.Index(name, "="); idx >= 0 {
		name = name[:idx]
	}
	switch name {
	case "config", "depth", "format", "rpc":
		return true
	default:
		return false
	}
}
