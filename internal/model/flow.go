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
	Layers         []LayerCalls         `json:"layers,omitempty"`
	InterfaceCalls []InterfaceCallTrace `json:"interface_calls,omitempty"`
	Dispatches     []DispatchTrace      `json:"dispatches,omitempty"`
	Branches       []BranchTrace        `json:"branches,omitempty"`
	Async          []CallRef            `json:"async,omitempty"`
	Unknown        []CallRef            `json:"unknown,omitempty"`
	layerOrder     []string
}

type LayerCalls struct {
	Name  string    `json:"name"`
	Calls []CallRef `json:"calls,omitempty"`
}

type BranchTrace struct {
	Kind     string       `json:"kind"`
	Function string       `json:"function"`
	Expr     string       `json:"expr,omitempty"`
	File     string       `json:"file"`
	Line     int          `json:"line"`
	Depth    int          `json:"depth,omitempty"`
	Cases    []BranchCase `json:"cases,omitempty"`
}

type BranchCase struct {
	Labels  []string     `json:"labels,omitempty"`
	Default bool         `json:"default,omitempty"`
	Layers  []LayerCalls `json:"layers,omitempty"`
	Unknown []CallRef    `json:"unknown,omitempty"`
}

type DispatchTrace struct {
	Table     string         `json:"table"`
	Key       string         `json:"key,omitempty"`
	Call      CallRef        `json:"call"`
	Interface string         `json:"interface,omitempty"`
	Cases     []DispatchCase `json:"cases,omitempty"`
}

type DispatchCase struct {
	Labels  []string     `json:"labels,omitempty"`
	Layers  []LayerCalls `json:"layers,omitempty"`
	Unknown []CallRef    `json:"unknown,omitempty"`
}

type InterfaceCallTrace struct {
	Call            CallRef                   `json:"call"`
	Interface       string                    `json:"interface"`
	Implementations []ImplementationCandidate `json:"implementations,omitempty"`
}

type ImplementationCandidate struct {
	Call     CallRef `json:"call"`
	Expanded bool    `json:"expanded"`
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
	appendLayerCall(&t.Layers, name, call, t.layerOrder)
}

func (c *BranchCase) AppendLayerCall(name string, call CallRef, layerOrder []string) {
	if name == "" {
		c.Unknown = append(c.Unknown, call)
		return
	}
	appendLayerCall(&c.Layers, name, call, layerOrder)
}

func (c *DispatchCase) AppendLayerCall(name string, call CallRef, layerOrder []string) {
	if name == "" {
		c.Unknown = append(c.Unknown, call)
		return
	}
	appendLayerCall(&c.Layers, name, call, layerOrder)
}

func (t Trail) LayerCalls(name string) []CallRef {
	for _, layer := range t.Layers {
		if layer.Name == name {
			return layer.Calls
		}
	}
	return nil
}

func (c BranchCase) LayerCalls(name string) []CallRef {
	for _, layer := range c.Layers {
		if layer.Name == name {
			return layer.Calls
		}
	}
	return nil
}

func (c DispatchCase) LayerCalls(name string) []CallRef {
	for _, layer := range c.Layers {
		if layer.Name == name {
			return layer.Calls
		}
	}
	return nil
}

func appendLayerCall(layers *[]LayerCalls, name string, call CallRef, layerOrder []string) {
	for i := range *layers {
		if (*layers)[i].Name == name {
			(*layers)[i].Calls = append((*layers)[i].Calls, call)
			return
		}
	}

	layer := LayerCalls{Name: name, Calls: []CallRef{call}}
	insertAt := layerInsertIndex(*layers, name, layerOrder)
	if insertAt == len(*layers) {
		*layers = append(*layers, layer)
		return
	}
	*layers = append(*layers, LayerCalls{})
	copy((*layers)[insertAt+1:], (*layers)[insertAt:])
	(*layers)[insertAt] = layer
}

func layerInsertIndex(layers []LayerCalls, name string, layerOrder []string) int {
	targetOrder := orderIndex(layerOrder, name)
	if targetOrder < 0 {
		return len(layers)
	}
	for i, layer := range layers {
		currentOrder := orderIndex(layerOrder, layer.Name)
		if currentOrder < 0 || currentOrder > targetOrder {
			return i
		}
	}
	return len(layers)
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
