package main

import (
	"testing"
)

func TestExtStrings(t *testing.T) {
	env := mustNewCELEnv(t)
	tests := []struct {
		name string
		expr string
		want bool
	}{
		{"split", `"a,b,c".split(",").size() == 3`, true},
		{"join", `["a", "b"].join("-") == "a-b"`, true},
		{"lowerAscii", `"HELLO".lowerAscii() == "hello"`, true},
		{"upperAscii", `"hello".upperAscii() == "HELLO"`, true},
		{"trim", `"  hello  ".trim() == "hello"`, true},
		{"replace", `"foo bar".replace("bar", "baz") == "foo baz"`, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prg := mustCompile(t, env, tt.expr)
			got, err := EvalCELBool(prg, map[string]any{}, nil)
			if err != nil {
				t.Fatal(err)
			}
			if !got {
				t.Errorf("expected true for %s", tt.expr)
			}
		})
	}
}

func TestExtLists(t *testing.T) {
	env := mustNewCELEnv(t)
	tests := []struct {
		name string
		expr string
		want bool
	}{
		{"sort", `[3, 1, 2].sort() == [1, 2, 3]`, true},
		{"distinct", `[1, 2, 2, 3].distinct() == [1, 2, 3]`, true},
		{"flatten", `[[1, 2], [3]].flatten() == [1, 2, 3]`, true},
		{"slice", `[1, 2, 3, 4].slice(1, 3) == [2, 3]`, true},
		{"reverse", `[1, 2, 3].reverse() == [3, 2, 1]`, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prg := mustCompile(t, env, tt.expr)
			got, err := EvalCELBool(prg, map[string]any{}, nil)
			if err != nil {
				t.Fatal(err)
			}
			if !got {
				t.Errorf("expected true for %s", tt.expr)
			}
		})
	}
}

func TestExtSets(t *testing.T) {
	env := mustNewCELEnv(t)
	prg := mustCompile(t, env, `sets.intersects([1, 2], [2, 3])`)
	got, err := EvalCELBool(prg, map[string]any{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !got {
		t.Error("expected true")
	}
}

func TestExtMath(t *testing.T) {
	env := mustNewCELEnv(t)
	tests := []struct {
		name string
		expr string
		want bool
	}{
		{"greatest", `math.greatest(1, 3, 2) == 3`, true},
		{"least", `math.least(1, 3, 2) == 1`, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prg := mustCompile(t, env, tt.expr)
			got, err := EvalCELBool(prg, map[string]any{}, nil)
			if err != nil {
				t.Fatal(err)
			}
			if !got {
				t.Errorf("expected true for %s", tt.expr)
			}
		})
	}
}

func TestExtEncoders(t *testing.T) {
	env := mustNewCELEnv(t)
	prg := mustCompile(t, env, `base64.decode("aGVsbG8=") == b"hello"`)
	got, err := EvalCELBool(prg, map[string]any{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !got {
		t.Error("expected true")
	}
}

func TestExtBindings(t *testing.T) {
	env := mustNewCELEnv(t)
	tests := []struct {
		name  string
		expr  string
		event any
		want  bool
	}{
		{
			"bind_simple",
			`cel.bind(ti, event.tool_input, ti.command.contains("rm") && ti.command.contains("-rf"))`,
			map[string]any{"tool_input": map[string]any{"command": "rm -rf /"}},
			true,
		},
		{
			"bind_false",
			`cel.bind(ti, event.tool_input, ti.command.contains("rm -rf"))`,
			map[string]any{"tool_input": map[string]any{"command": "ls -la"}},
			false,
		},
		{
			"bind_nested",
			`cel.bind(ti, event.tool_input, cel.bind(cmd, ti.command, cmd.contains("deploy") && cmd.contains("prod")))`,
			map[string]any{"tool_input": map[string]any{"command": "deploy to prod"}},
			true,
		},
		{
			"bind_multiple_fields",
			`cel.bind(ti, event.tool_input, ti.command.contains("go") && ti.file_path.endsWith(".go"))`,
			map[string]any{"tool_input": map[string]any{"command": "go build", "file_path": "main.go"}},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prg := mustCompile(t, env, tt.expr)
			got, err := EvalCELBool(prg, tt.event, nil)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("got %v, want %v for %s", got, tt.want, tt.expr)
			}
		})
	}
}

func TestExtRegex(t *testing.T) {
	env := mustNewCELEnv(t)
	tests := []struct {
		name string
		expr string
		want bool
	}{
		{"replace", `regex.replace("foo123bar", "[0-9]+", "NUM") == "fooNUMbar"`, true},
		{"extract", `regex.extract("foo123bar", "([0-9]+)") == optional.of("123")`, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prg := mustCompile(t, env, tt.expr)
			got, err := EvalCELBool(prg, map[string]any{}, nil)
			if err != nil {
				t.Fatal(err)
			}
			if !got {
				t.Errorf("expected true for %s", tt.expr)
			}
		})
	}
}
