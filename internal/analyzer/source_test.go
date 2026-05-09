package analyzer

import "testing"

func TestIsStdlibImportPath(t *testing.T) {
	tests := []struct {
		name       string
		importPath string
		want       bool
	}{
		{name: "stdlib root", importPath: "context", want: true},
		{name: "stdlib nested", importPath: "net/http", want: true},
		{name: "domain package", importPath: "github.com/usuginus/calltrail-go", want: false},
		{name: "domain package with go prefix", importPath: "go.temporal.io/server/common", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := make(map[string]bool)
			if got := isStdlibImportPath(tt.importPath, cache); got != tt.want {
				t.Fatalf("isStdlibImportPath(%q) = %t, want %t", tt.importPath, got, tt.want)
			}
			if got, ok := cache[tt.importPath]; !ok || got != tt.want {
				t.Fatalf("cache[%q] = %t, %t; want %t, true", tt.importPath, got, ok, tt.want)
			}
		})
	}
}
