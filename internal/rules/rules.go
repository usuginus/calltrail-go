package rules

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed presets/*.yaml
var presets embed.FS

type RuleSet struct {
	Handlers       HandlerRules
	Classifiers    []ClassifierRule
	IgnoreCalls    IgnoreCallRules
	Implementation ImplementationRules
}

type HandlerRules struct {
	PackageNames           []string
	PathContains           []string
	RequireContextFirstArg bool
	RequirePointerRequest  bool
	RequirePointerResponse bool
	RequireErrorReturn     bool
}

type ClassifierRule struct {
	Layer          string
	SymbolContains []string
	TypeContains   []string
	PathContains   []string
	MethodPrefixes []string
	MethodContains []string
}

type IgnoreCallRules struct {
	Symbols              []string
	Packages             []string
	Methods              []string
	MethodPrefixes       []string
	SymbolPrefixes       []string
	AutoStdlib           bool
	LocalGetters         bool
	ProtoGetterReceivers []string
}

type ImplementationRules struct {
	MockReceiverPrefixes []string
	MockPathContains     []string
}

func Load(preset string, configPath string) (RuleSet, error) {
	if preset == "" {
		preset = "generic"
	}
	data, err := presets.ReadFile(filepath.Join("presets", preset+".yaml"))
	if err != nil {
		return RuleSet{}, fmt.Errorf("load preset %q: %w", preset, err)
	}
	ruleSet, err := parseYAML(string(data))
	if err != nil {
		return RuleSet{}, fmt.Errorf("parse preset %q: %w", preset, err)
	}
	if configPath == "" {
		return ruleSet, nil
	}
	configData, err := os.ReadFile(configPath)
	if err != nil {
		return RuleSet{}, fmt.Errorf("read config %q: %w", configPath, err)
	}
	configRules, err := parseYAML(string(configData))
	if err != nil {
		return RuleSet{}, fmt.Errorf("parse config %q: %w", configPath, err)
	}
	ruleSet.Merge(configRules)
	return ruleSet, nil
}

func (r RuleSet) IsZero() bool {
	return len(r.Handlers.PackageNames) == 0 &&
		len(r.Handlers.PathContains) == 0 &&
		len(r.Classifiers) == 0 &&
		len(r.IgnoreCalls.Symbols) == 0 &&
		len(r.IgnoreCalls.Packages) == 0 &&
		!r.IgnoreCalls.AutoStdlib
}

func (r *RuleSet) Merge(other RuleSet) {
	r.Handlers.PackageNames = append(r.Handlers.PackageNames, other.Handlers.PackageNames...)
	r.Handlers.PathContains = append(r.Handlers.PathContains, other.Handlers.PathContains...)
	r.Handlers.RequireContextFirstArg = r.Handlers.RequireContextFirstArg || other.Handlers.RequireContextFirstArg
	r.Handlers.RequirePointerRequest = r.Handlers.RequirePointerRequest || other.Handlers.RequirePointerRequest
	r.Handlers.RequirePointerResponse = r.Handlers.RequirePointerResponse || other.Handlers.RequirePointerResponse
	r.Handlers.RequireErrorReturn = r.Handlers.RequireErrorReturn || other.Handlers.RequireErrorReturn
	r.Classifiers = append(r.Classifiers, other.Classifiers...)
	r.IgnoreCalls.Symbols = append(r.IgnoreCalls.Symbols, other.IgnoreCalls.Symbols...)
	r.IgnoreCalls.Packages = append(r.IgnoreCalls.Packages, other.IgnoreCalls.Packages...)
	r.IgnoreCalls.Methods = append(r.IgnoreCalls.Methods, other.IgnoreCalls.Methods...)
	r.IgnoreCalls.MethodPrefixes = append(r.IgnoreCalls.MethodPrefixes, other.IgnoreCalls.MethodPrefixes...)
	r.IgnoreCalls.SymbolPrefixes = append(r.IgnoreCalls.SymbolPrefixes, other.IgnoreCalls.SymbolPrefixes...)
	r.IgnoreCalls.AutoStdlib = r.IgnoreCalls.AutoStdlib || other.IgnoreCalls.AutoStdlib
	r.IgnoreCalls.LocalGetters = r.IgnoreCalls.LocalGetters || other.IgnoreCalls.LocalGetters
	r.IgnoreCalls.ProtoGetterReceivers = append(r.IgnoreCalls.ProtoGetterReceivers, other.IgnoreCalls.ProtoGetterReceivers...)
	r.Implementation.MockReceiverPrefixes = append(r.Implementation.MockReceiverPrefixes, other.Implementation.MockReceiverPrefixes...)
	r.Implementation.MockPathContains = append(r.Implementation.MockPathContains, other.Implementation.MockPathContains...)
}
