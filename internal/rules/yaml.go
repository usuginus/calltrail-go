package rules

import (
	"fmt"
	"strings"
)

type parserState struct {
	section         string
	handlerGroup    string
	layerGroup      string
	ignoreGroup     string
	resolutionGroup string
	listKey         string
	currentLayer    *LayerRule
}

func parseYAML(input string) (RuleSet, error) {
	var ruleSet RuleSet
	var state parserState

	lines := strings.Split(input, "\n")
	for lineNo, raw := range lines {
		line := stripComment(raw)
		if strings.TrimSpace(line) == "" {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " "))
		text := strings.TrimSpace(line)

		switch {
		case strings.HasPrefix(text, "- "):
			value := strings.TrimSpace(strings.TrimPrefix(text, "- "))
			if err := addListItem(&ruleSet, &state, indent, value); err != nil {
				return RuleSet{}, withLine(lineNo, err)
			}
		case strings.HasSuffix(text, ":"):
			key := strings.TrimSpace(strings.TrimSuffix(text, ":"))
			if err := setContext(&state, indent, key); err != nil {
				return RuleSet{}, withLine(lineNo, err)
			}
		default:
			key, value, ok := strings.Cut(text, ":")
			if !ok {
				return RuleSet{}, withLine(lineNo, fmt.Errorf("unsupported line %q", text))
			}
			key = strings.TrimSpace(key)
			value = strings.TrimSpace(value)
			if err := setScalar(&ruleSet, &state, indent, key, value); err != nil {
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

func cleanValue(value string) string {
	return strings.Trim(strings.TrimSpace(value), "\"'")
}

func setContext(state *parserState, indent int, key string) error {
	switch indent {
	case 0:
		switch key {
		case "handlers", "layers", "ignore", "resolution":
			*state = parserState{section: key}
			return nil
		default:
			return fmt.Errorf("unknown section %q", key)
		}
	case 2:
		switch state.section {
		case "handlers":
			switch key {
			case "match", "signature":
				state.handlerGroup = key
				state.listKey = ""
				return nil
			default:
				return fmt.Errorf("unknown handlers group %q", key)
			}
		case "ignore":
			switch key {
			case "calls", "getters":
				state.ignoreGroup = key
				state.listKey = ""
				return nil
			default:
				return fmt.Errorf("unknown ignore group %q", key)
			}
		case "resolution":
			if key != "skip_implementations" {
				return fmt.Errorf("unknown resolution group %q", key)
			}
			state.resolutionGroup = key
			state.listKey = ""
			return nil
		default:
			return fmt.Errorf("unexpected nested key %q in section %q", key, state.section)
		}
	case 4:
		switch state.section {
		case "handlers":
			if state.handlerGroup == "" {
				return fmt.Errorf("handlers key %q must be under match or signature", key)
			}
			state.listKey = key
			return nil
		case "ignore":
			if state.ignoreGroup == "" {
				return fmt.Errorf("ignore key %q must be under calls or getters", key)
			}
			state.listKey = key
			return nil
		case "resolution":
			if state.resolutionGroup == "" {
				return fmt.Errorf("resolution key %q must be under skip_implementations", key)
			}
			state.listKey = key
			return nil
		case "layers":
			if state.currentLayer == nil {
				return fmt.Errorf("layer key %q without layer item", key)
			}
			if key != "match" {
				return fmt.Errorf("unknown layer group %q", key)
			}
			state.layerGroup = key
			state.listKey = ""
			return nil
		default:
			return fmt.Errorf("unexpected nested key %q", key)
		}
	case 6:
		if state.section != "layers" || state.currentLayer == nil || state.layerGroup != "match" {
			return fmt.Errorf("unexpected list key %q", key)
		}
		state.listKey = key
		return nil
	default:
		return fmt.Errorf("unsupported indentation for key %q", key)
	}
}

func setScalar(ruleSet *RuleSet, state *parserState, indent int, key string, value string) error {
	value = cleanValue(value)
	if indent == 0 {
		if key != "version" {
			return fmt.Errorf("unknown top-level key %q", key)
		}
		if value != "1" {
			return fmt.Errorf("unsupported version %q", value)
		}
		return nil
	}

	switch state.section {
	case "handlers":
		if state.handlerGroup != "signature" || indent != 4 {
			return fmt.Errorf("unknown handlers scalar %q", key)
		}
		return setHandlerSignatureScalar(&ruleSet.Handlers.Signature, key, value)
	case "layers":
		if state.currentLayer == nil || indent != 4 {
			return fmt.Errorf("unknown layer scalar %q", key)
		}
		return setLayerScalar(state.currentLayer, key, value)
	case "ignore":
		switch {
		case indent == 2 && key == "standard_library":
			ruleSet.Ignore.StandardLibrary = parseBool(value)
			return nil
		case indent == 4 && state.ignoreGroup == "getters" && key == "local_values":
			ruleSet.Ignore.Getters.LocalValues = parseBool(value)
			return nil
		default:
			return fmt.Errorf("unknown ignore scalar %q", key)
		}
	default:
		return fmt.Errorf("unknown scalar section %q", state.section)
	}
}

func setHandlerSignatureScalar(signature *HandlerSignatureRules, key string, value string) error {
	switch key {
	case "require_context_first_arg":
		signature.RequireContextFirstArg = parseBool(value)
	case "require_pointer_request":
		signature.RequirePointerRequest = parseBool(value)
	case "require_pointer_response":
		signature.RequirePointerResponse = parseBool(value)
	case "require_error_return":
		signature.RequireErrorReturn = parseBool(value)
	default:
		return fmt.Errorf("unknown handler signature key %q", key)
	}
	return nil
}

func setLayerScalar(layer *LayerRule, key string, value string) error {
	switch key {
	case "name":
		layer.Name = value
	default:
		return fmt.Errorf("unknown layer key %q", key)
	}
	return nil
}

func addListItem(ruleSet *RuleSet, state *parserState, indent int, value string) error {
	value = cleanValue(value)
	if state.section == "layers" && indent == 2 {
		layer := LayerRule{}
		if value != "" {
			key, scalar, ok := strings.Cut(value, ":")
			if !ok {
				return fmt.Errorf("unsupported layer item %q", value)
			}
			if err := setLayerScalar(&layer, strings.TrimSpace(key), cleanValue(scalar)); err != nil {
				return err
			}
		}
		ruleSet.Layers = append(ruleSet.Layers, layer)
		state.currentLayer = &ruleSet.Layers[len(ruleSet.Layers)-1]
		state.layerGroup = ""
		state.listKey = ""
		return nil
	}

	if state.listKey == "" {
		return fmt.Errorf("list value %q without list key", value)
	}
	switch state.section {
	case "handlers":
		return addHandlerListValue(&ruleSet.Handlers, state.handlerGroup, state.listKey, value)
	case "layers":
		if state.currentLayer == nil || state.layerGroup != "match" {
			return fmt.Errorf("layer list value %q without match group", value)
		}
		return addLayerListValue(state.currentLayer, state.listKey, value)
	case "ignore":
		return addIgnoreListValue(&ruleSet.Ignore, state.ignoreGroup, state.listKey, value)
	case "resolution":
		return addResolutionListValue(&ruleSet.Resolution, state.resolutionGroup, state.listKey, value)
	default:
		return fmt.Errorf("unknown list section %q", state.section)
	}
}

func addHandlerListValue(handlers *HandlerRules, group string, key string, value string) error {
	if group != "match" {
		return fmt.Errorf("handler list %q must be under match", key)
	}
	switch key {
	case "package_names":
		handlers.Match.PackageNames = append(handlers.Match.PackageNames, value)
	case "file_path_contains":
		handlers.Match.FilePathContains = append(handlers.Match.FilePathContains, value)
	default:
		return fmt.Errorf("unknown handler match list %q", key)
	}
	return nil
}

func addLayerListValue(layer *LayerRule, key string, value string) error {
	switch key {
	case "call_name_contains":
		layer.Match.CallNameContains = append(layer.Match.CallNameContains, value)
	case "receiver_type_contains":
		layer.Match.ReceiverTypeContains = append(layer.Match.ReceiverTypeContains, value)
	case "file_path_contains":
		layer.Match.FilePathContains = append(layer.Match.FilePathContains, value)
	case "method_name_prefixes":
		layer.Match.MethodNamePrefixes = append(layer.Match.MethodNamePrefixes, value)
	case "method_name_contains":
		layer.Match.MethodNameContains = append(layer.Match.MethodNameContains, value)
	default:
		return fmt.Errorf("unknown layer match list %q", key)
	}
	return nil
}

func addIgnoreListValue(ignore *IgnoreRules, group string, key string, value string) error {
	switch group {
	case "calls":
		switch key {
		case "full_names":
			ignore.Calls.FullNames = append(ignore.Calls.FullNames, value)
		case "package_names":
			ignore.Calls.PackageNames = append(ignore.Calls.PackageNames, value)
		case "method_names":
			ignore.Calls.MethodNames = append(ignore.Calls.MethodNames, value)
		case "method_name_prefixes":
			ignore.Calls.MethodNamePrefixes = append(ignore.Calls.MethodNamePrefixes, value)
		case "full_name_prefixes":
			ignore.Calls.FullNamePrefixes = append(ignore.Calls.FullNamePrefixes, value)
		default:
			return fmt.Errorf("unknown ignore calls list %q", key)
		}
	case "getters":
		if key != "receiver_names" {
			return fmt.Errorf("unknown ignore getters list %q", key)
		}
		ignore.Getters.ReceiverNames = append(ignore.Getters.ReceiverNames, value)
	default:
		return fmt.Errorf("unknown ignore group %q", group)
	}
	return nil
}

func addResolutionListValue(resolution *ResolutionRules, group string, key string, value string) error {
	if group != "skip_implementations" {
		return fmt.Errorf("unknown resolution group %q", group)
	}
	switch key {
	case "receiver_name_prefixes":
		resolution.SkipImplementations.ReceiverNamePrefixes = append(resolution.SkipImplementations.ReceiverNamePrefixes, value)
	case "file_path_contains":
		resolution.SkipImplementations.FilePathContains = append(resolution.SkipImplementations.FilePathContains, value)
	default:
		return fmt.Errorf("unknown resolution skip_implementations list %q", key)
	}
	return nil
}

func parseBool(value string) bool {
	return strings.EqualFold(value, "true") || value == "1" || strings.EqualFold(value, "yes")
}
