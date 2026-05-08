package model

type APIFlow struct {
	Name       string       `json:"name"`
	Kind       string       `json:"kind"`
	Entrypoint Entrypoint   `json:"entrypoint"`
	Request    TypeRef      `json:"request"`
	Response   TypeRef      `json:"response"`
	Trail      Trail        `json:"trail"`
	Errors     ErrorSummary `json:"errors"`
	Unresolved []Unresolved `json:"unresolved,omitempty"`
	Confidence Confidence   `json:"confidence"`
}

type Entrypoint struct {
	Symbol string `json:"symbol"`
	File   string `json:"file"`
	Line   int    `json:"line"`
}

type TypeRef struct {
	Type string `json:"type"`
}

type Trail struct {
	Layers     []LayerCalls `json:"layers,omitempty"`
	Async      []CallRef    `json:"async,omitempty"`
	Unknown    []CallRef    `json:"unknown,omitempty"`
	layerOrder []string
}

type LayerCalls struct {
	Name  string    `json:"name"`
	Calls []CallRef `json:"calls,omitempty"`
}

type CallRef struct {
	Symbol   string `json:"symbol"`
	Receiver string `json:"receiver,omitempty"`
	Method   string `json:"method,omitempty"`
	File     string `json:"file"`
	Line     int    `json:"line"`
	Depth    int    `json:"depth,omitempty"`
	Via      string `json:"via,omitempty"`
}

type ErrorSummary struct {
	GRPCCodes []string `json:"grpc_codes,omitempty"`
}

type Unresolved struct {
	Call   string `json:"call"`
	Reason string `json:"reason"`
}

type Confidence struct {
	Overall string `json:"overall"`
}

func NewTrail(layerOrder []string) Trail {
	return Trail{layerOrder: uniqueStrings(layerOrder)}
}

func (t *Trail) AppendLayerCall(name string, call CallRef) {
	if name == "" {
		t.Unknown = append(t.Unknown, call)
		return
	}
	for i := range t.Layers {
		if t.Layers[i].Name == name {
			t.Layers[i].Calls = append(t.Layers[i].Calls, call)
			return
		}
	}

	layer := LayerCalls{Name: name, Calls: []CallRef{call}}
	insertAt := t.layerInsertIndex(name)
	if insertAt == len(t.Layers) {
		t.Layers = append(t.Layers, layer)
		return
	}
	t.Layers = append(t.Layers, LayerCalls{})
	copy(t.Layers[insertAt+1:], t.Layers[insertAt:])
	t.Layers[insertAt] = layer
}

func (t Trail) LayerCalls(name string) []CallRef {
	for _, layer := range t.Layers {
		if layer.Name == name {
			return layer.Calls
		}
	}
	return nil
}

func (t Trail) layerInsertIndex(name string) int {
	targetOrder := orderIndex(t.layerOrder, name)
	if targetOrder < 0 {
		return len(t.Layers)
	}
	for i, layer := range t.Layers {
		layerOrder := orderIndex(t.layerOrder, layer.Name)
		if layerOrder < 0 || layerOrder > targetOrder {
			return i
		}
	}
	return len(t.Layers)
}

func orderIndex(values []string, value string) int {
	for i, existing := range values {
		if existing == value {
			return i
		}
	}
	return -1
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	var out []string
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}
