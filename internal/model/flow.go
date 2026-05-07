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
	Usecases        []CallRef `json:"usecases,omitempty"`
	Services        []CallRef `json:"services,omitempty"`
	Repositories    []CallRef `json:"repositories,omitempty"`
	Models          []TypeRef `json:"models,omitempty"`
	ExternalClients []CallRef `json:"external_clients,omitempty"`
	Converters      []CallRef `json:"converters,omitempty"`
	Async           []CallRef `json:"async,omitempty"`
	Unknown         []CallRef `json:"unknown,omitempty"`
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
