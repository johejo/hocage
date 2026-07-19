package main

import (
	"reflect"
	"strings"
	"testing"
)

var resolveTestEvent = map[string]any{
	"tool_input": map[string]any{
		"command":   "rm -rf /",
		"file_path": "/tmp/main.go",
	},
	"count": float64(3),
}

func TestResolveValue(t *testing.T) {
	env, err := NewCELEnv()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name  string
		input any
		want  any
	}{
		{
			name:  "literal string stays literal",
			input: "no interpolation",
			want:  "no interpolation",
		},
		{
			name:  "legacy braces stay literal",
			input: "use {{name}} here",
			want:  "use {{name}} here",
		},
		{
			name:  "string expression",
			input: map[string]any{"cel": `event.tool_input.command + " is bad"`},
			want:  "rm -rf / is bad",
		},
		{
			name:  "typed int result",
			input: map[string]any{"cel": "1 + 2"},
			want:  float64(3),
		},
		{
			name:  "typed bool result",
			input: map[string]any{"cel": "true"},
			want:  true,
		},
		{
			name:  "typed object result",
			input: map[string]any{"cel": `{"command": event.tool_input.command, "n": 1}`},
			want:  map[string]any{"command": "rm -rf /", "n": float64(1)},
		},
		{
			name: "node nested in map and list",
			input: map[string]any{
				"reason": map[string]any{"cel": `"%s is not allowed".format([event.tool_input.command])`},
				"static": "text",
				"items":  []any{"a", map[string]any{"cel": "event.count"}},
			},
			want: map[string]any{
				"reason": "rm -rf / is not allowed",
				"static": "text",
				"items":  []any{"a", float64(3)},
			},
		},
		{
			name:  "two-key map with cel key is not a node",
			input: map[string]any{"cel": "1 + 1", "other": "x"},
			want:  map[string]any{"cel": "1 + 1", "other": "x"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveValue(env, tt.input, resolveTestEvent, nil)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestResolveValueErrors(t *testing.T) {
	env, err := NewCELEnv()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		input   any
		wantErr string
	}{
		{
			name:    "non-string cel value",
			input:   map[string]any{"cel": []any{"a"}},
			wantErr: "cel expression must be a string",
		},
		{
			name:    "compile error",
			input:   map[string]any{"cel": "1 +"},
			wantErr: "compile CEL",
		},
		{
			name:    "eval error",
			input:   map[string]any{"cel": "event.no_such.field"},
			wantErr: "no_such",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ResolveValue(env, tt.input, resolveTestEvent, nil)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("err = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestResolveStringSlot(t *testing.T) {
	env, err := NewCELEnv()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		input   any
		want    string
		wantErr string
	}{
		{name: "literal", input: "plain", want: "plain"},
		{name: "string expression", input: map[string]any{"cel": "event.tool_input.file_path"}, want: "/tmp/main.go"},
		{name: "number coerced", input: map[string]any{"cel": "event.count"}, want: "3"},
		{name: "bool coerced", input: map[string]any{"cel": "1 < 2"}, want: "true"},
		{name: "map rejected", input: map[string]any{"cel": "event.tool_input"}, wantErr: "must be a string"},
		{name: "list rejected", input: map[string]any{"cel": "[1, 2]"}, wantErr: "must be a string"},
		{name: "non-string non-node", input: 42, wantErr: "must be a string or {cel: ...} node"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveStringSlot(env, tt.input, resolveTestEvent, nil)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("err = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
