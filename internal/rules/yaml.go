package rules

import (
	"fmt"
	"strings"
)

func parseYAML(input string) (RuleSet, error) {
	var ruleSet RuleSet
	var section string
	var listKey string
	var currentClassifier *ClassifierRule

	lines := strings.Split(input, "\n")
	for lineNo, raw := range lines {
		line := stripComment(raw)
		if strings.TrimSpace(line) == "" {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " "))
		text := strings.TrimSpace(line)

		switch {
		case indent == 0 && strings.HasSuffix(text, ":"):
			section = strings.TrimSuffix(text, ":")
			listKey = ""
			currentClassifier = nil
		case section == "classifiers" && indent == 2 && strings.HasPrefix(text, "- "):
			classifier := ClassifierRule{}
			if err := setClassifierScalar(&classifier, strings.TrimPrefix(text, "- ")); err != nil {
				return RuleSet{}, withLine(lineNo, err)
			}
			ruleSet.Classifiers = append(ruleSet.Classifiers, classifier)
			currentClassifier = &ruleSet.Classifiers[len(ruleSet.Classifiers)-1]
			listKey = ""
		case strings.HasPrefix(text, "- "):
			value := strings.TrimSpace(strings.TrimPrefix(text, "- "))
			if err := addListValue(&ruleSet, currentClassifier, section, listKey, value); err != nil {
				return RuleSet{}, withLine(lineNo, err)
			}
		case strings.HasSuffix(text, ":"):
			listKey = strings.TrimSuffix(text, ":")
		default:
			key, value, ok := strings.Cut(text, ":")
			if !ok {
				return RuleSet{}, withLine(lineNo, fmt.Errorf("unsupported line %q", text))
			}
			key = strings.TrimSpace(key)
			value = strings.TrimSpace(value)
			if section == "classifiers" && currentClassifier != nil {
				if err := setClassifierScalar(currentClassifier, key+": "+value); err != nil {
					return RuleSet{}, withLine(lineNo, err)
				}
				continue
			}
			if err := setScalar(&ruleSet, section, key, value); err != nil {
				return RuleSet{}, withLine(lineNo, err)
			}
		}
	}
	return ruleSet, nil
}

func stripComment(line string) string {
	if idx := strings.Index(line, "#"); idx >= 0 {
		return line[:idx]
	}
	return line
}

func withLine(lineNo int, err error) error {
	return fmt.Errorf("line %d: %w", lineNo+1, err)
}

func setScalar(ruleSet *RuleSet, section string, key string, value string) error {
	switch section {
	case "handlers":
		switch key {
		case "require_context_first_arg":
			ruleSet.Handlers.RequireContextFirstArg = parseBool(value)
		case "require_pointer_request":
			ruleSet.Handlers.RequirePointerRequest = parseBool(value)
		case "require_pointer_response":
			ruleSet.Handlers.RequirePointerResponse = parseBool(value)
		case "require_error_return":
			ruleSet.Handlers.RequireErrorReturn = parseBool(value)
		default:
			return fmt.Errorf("unknown handlers key %q", key)
		}
	case "ignore_calls":
		switch key {
		case "auto_stdlib":
			ruleSet.IgnoreCalls.AutoStdlib = parseBool(value)
		case "local_getters":
			ruleSet.IgnoreCalls.LocalGetters = parseBool(value)
		default:
			return fmt.Errorf("unknown ignore_calls key %q", key)
		}
	default:
		return fmt.Errorf("unknown scalar section %q", section)
	}
	return nil
}

func setClassifierScalar(classifier *ClassifierRule, expr string) error {
	key, value, ok := strings.Cut(expr, ":")
	if !ok {
		return fmt.Errorf("unsupported classifier expression %q", expr)
	}
	key = strings.TrimSpace(key)
	value = strings.TrimSpace(value)
	switch key {
	case "layer":
		classifier.Layer = value
	default:
		return fmt.Errorf("unknown classifier key %q", key)
	}
	return nil
}

func addListValue(ruleSet *RuleSet, classifier *ClassifierRule, section string, key string, value string) error {
	value = strings.Trim(value, "\"'")
	if section == "classifiers" {
		if classifier == nil {
			return fmt.Errorf("classifier list value without classifier")
		}
		switch key {
		case "symbol_contains":
			classifier.SymbolContains = append(classifier.SymbolContains, value)
		case "type_contains":
			classifier.TypeContains = append(classifier.TypeContains, value)
		case "path_contains":
			classifier.PathContains = append(classifier.PathContains, value)
		case "method_prefixes":
			classifier.MethodPrefixes = append(classifier.MethodPrefixes, value)
		case "method_contains":
			classifier.MethodContains = append(classifier.MethodContains, value)
		default:
			return fmt.Errorf("unknown classifier list %q", key)
		}
		return nil
	}

	switch section {
	case "handlers":
		switch key {
		case "package_names":
			ruleSet.Handlers.PackageNames = append(ruleSet.Handlers.PackageNames, value)
		case "path_contains":
			ruleSet.Handlers.PathContains = append(ruleSet.Handlers.PathContains, value)
		default:
			return fmt.Errorf("unknown handlers list %q", key)
		}
	case "ignore_calls":
		switch key {
		case "symbols":
			ruleSet.IgnoreCalls.Symbols = append(ruleSet.IgnoreCalls.Symbols, value)
		case "packages":
			ruleSet.IgnoreCalls.Packages = append(ruleSet.IgnoreCalls.Packages, value)
		case "methods":
			ruleSet.IgnoreCalls.Methods = append(ruleSet.IgnoreCalls.Methods, value)
		case "method_prefixes":
			ruleSet.IgnoreCalls.MethodPrefixes = append(ruleSet.IgnoreCalls.MethodPrefixes, value)
		case "symbol_prefixes":
			ruleSet.IgnoreCalls.SymbolPrefixes = append(ruleSet.IgnoreCalls.SymbolPrefixes, value)
		case "proto_getter_receivers":
			ruleSet.IgnoreCalls.ProtoGetterReceivers = append(ruleSet.IgnoreCalls.ProtoGetterReceivers, value)
		default:
			return fmt.Errorf("unknown ignore_calls list %q", key)
		}
	case "implementation":
		switch key {
		case "mock_receiver_prefixes":
			ruleSet.Implementation.MockReceiverPrefixes = append(ruleSet.Implementation.MockReceiverPrefixes, value)
		case "mock_path_contains":
			ruleSet.Implementation.MockPathContains = append(ruleSet.Implementation.MockPathContains, value)
		default:
			return fmt.Errorf("unknown implementation list %q", key)
		}
	default:
		return fmt.Errorf("unknown list section %q", section)
	}
	return nil
}

func parseBool(value string) bool {
	return strings.EqualFold(value, "true") || value == "1" || strings.EqualFold(value, "yes")
}
