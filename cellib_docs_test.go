package main

import (
	"slices"
	"testing"

	"github.com/google/cel-go/cel"
)

// TestCELFuncDocsComplete fails when the celFuncGroups registry and the
// functions registered by HocageLibrary drift apart in either direction.
func TestCELFuncDocsComplete(t *testing.T) {
	registered, err := customCELFunctions()
	if err != nil {
		t.Fatal(err)
	}
	if len(registered) == 0 {
		t.Fatal("no custom CEL functions found; env diff is broken")
	}
	documented := make(map[string]bool)
	for _, g := range celFuncGroups {
		for _, f := range g.Funcs {
			if documented[f.Name] {
				t.Errorf("function %q is documented more than once in celFuncGroups", f.Name)
			}
			documented[f.Name] = true
		}
	}
	for _, name := range registered {
		if !documented[name] {
			t.Errorf("function %q is registered but not documented in celFuncGroups", name)
		}
	}
	for name := range documented {
		if !slices.Contains(registered, name) {
			t.Errorf("celFuncGroups documents %q, which is not registered in the CEL env", name)
		}
	}
}

// TestStdExtDocsComplete fails when the stdExtDocs registry and the extension
// libraries enabled by baseEnvOptions drift apart in either direction.
func TestStdExtDocsComplete(t *testing.T) {
	base, err := cel.NewEnv(baseEnvOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	enabled := make(map[string]bool)
	for _, lib := range base.Libraries() {
		if lib == "cel.lib.std" {
			continue
		}
		enabled[lib] = true
	}
	documented := make(map[string]bool)
	for _, e := range stdExtDocs {
		if documented[e.LibName] {
			t.Errorf("library %q is documented more than once in stdExtDocs", e.LibName)
		}
		documented[e.LibName] = true
		if !enabled[e.LibName] {
			t.Errorf("stdExtDocs documents %q, which is not enabled in baseEnvOptions", e.LibName)
		}
	}
	for lib := range enabled {
		if !documented[lib] {
			t.Errorf("extension library %q is enabled but not documented in stdExtDocs", lib)
		}
	}
}
