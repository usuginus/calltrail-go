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
	Handlers   HandlerRules
	Layers     []LayerRule
	Ignore     IgnoreRules
	Resolution ResolutionRules
}

type HandlerRules struct {
	Match     HandlerMatchRules
	Signature HandlerSignatureRules
}

type HandlerMatchRules struct {
	PackageNames     []string
	FilePathContains []string
}

type HandlerSignatureRules struct {
	RequireContextFirstArg bool
	RequirePointerRequest  bool
	RequirePointerResponse bool
	RequireErrorReturn     bool
}

type LayerRule struct {
	Name  string
	Match LayerMatchRules
}

type LayerMatchRules struct {
	CallNameContains     []string
	ReceiverTypeContains []string
	FilePathContains     []string
	MethodNamePrefixes   []string
	MethodNameContains   []string
}

type IgnoreRules struct {
	StandardLibrary bool
	Calls           IgnoreCallRules
	Getters         IgnoreGetterRules
}

type IgnoreCallRules struct {
	FullNames          []string
	PackageNames       []string
	MethodNames        []string
	MethodNamePrefixes []string
	FullNamePrefixes   []string
}

type IgnoreGetterRules struct {
	LocalValues   bool
	ReceiverNames []string
}

type ResolutionRules struct {
	SkipImplementations SkipImplementationRules
}

type SkipImplementationRules struct {
	ReceiverNamePrefixes []string
	FilePathContains     []string
}

func Load(configPath string) (RuleSet, error) {
	if configPath != "" {
		data, err := os.ReadFile(configPath)
		if err != nil {
			return RuleSet{}, fmt.Errorf("read config %q: %w", configPath, err)
		}
		ruleSet, err := parseYAML(string(data))
		if err != nil {
			return RuleSet{}, fmt.Errorf("parse config %q: %w", configPath, err)
		}
		return ruleSet, nil
	}
	return loadPreset("generic")
}

func loadPreset(name string) (RuleSet, error) {
	if name == "" {
		name = "generic"
	}
	data, err := presets.ReadFile(filepath.Join("presets", name+".yaml"))
	if err != nil {
		return RuleSet{}, fmt.Errorf("load preset %q: %w", name, err)
	}
	ruleSet, err := parseYAML(string(data))
	if err != nil {
		return RuleSet{}, fmt.Errorf("parse preset %q: %w", name, err)
	}
	return ruleSet, nil
}

func (r RuleSet) IsZero() bool {
	return r.Handlers.isZero() &&
		len(r.Layers) == 0 &&
		r.Ignore.isZero() &&
		r.Resolution.isZero()
}

func (r HandlerRules) isZero() bool {
	return len(r.Match.PackageNames) == 0 &&
		len(r.Match.FilePathContains) == 0 &&
		!r.Signature.RequireContextFirstArg &&
		!r.Signature.RequirePointerRequest &&
		!r.Signature.RequirePointerResponse &&
		!r.Signature.RequireErrorReturn
}

func (r IgnoreRules) isZero() bool {
	return !r.StandardLibrary &&
		len(r.Calls.FullNames) == 0 &&
		len(r.Calls.PackageNames) == 0 &&
		len(r.Calls.MethodNames) == 0 &&
		len(r.Calls.MethodNamePrefixes) == 0 &&
		len(r.Calls.FullNamePrefixes) == 0 &&
		!r.Getters.LocalValues &&
		len(r.Getters.ReceiverNames) == 0
}

func (r ResolutionRules) isZero() bool {
	return len(r.SkipImplementations.ReceiverNamePrefixes) == 0 &&
		len(r.SkipImplementations.FilePathContains) == 0
}
