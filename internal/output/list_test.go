package output

import (
	"bytes"
	"testing"

	"github.com/usuginus/calltrail-go/internal/model"
)

func TestWriteList(t *testing.T) {
	var buf bytes.Buffer
	err := WriteList(&buf, []model.APIFlow{
		{
			Name: "GetFoo",
			Entrypoint: model.Entrypoint{
				Symbol: "Server.GetFoo",
				File:   "handler.go",
				Line:   12,
			},
		},
	})
	if err != nil {
		t.Fatalf("WriteList returned error: %v", err)
	}

	want := "| rpc | handler | location |\n" +
		"| --- | --- | --- |\n" +
		"| `GetFoo` | `Server.GetFoo` | `handler.go:12` |\n"
	if got := buf.String(); got != want {
		t.Fatalf("list output = %q, want %q", got, want)
	}
}
