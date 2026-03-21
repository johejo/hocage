package main

import (
	"testing"
)

func TestDefault(t *testing.T) {
	env := mustNewCELEnv(t)

	t.Run("empty string fallback", func(t *testing.T) {
		prg := mustCompile(t, env, `default("fallback", "")`)
		got := mustEval(t, prg, map[string]any{})
		if got != "fallback" {
			t.Errorf("got %v", got)
		}
	})

	t.Run("non-empty string kept", func(t *testing.T) {
		prg := mustCompile(t, env, `default("fallback", "actual")`)
		got := mustEval(t, prg, map[string]any{})
		if got != "actual" {
			t.Errorf("got %v", got)
		}
	})

	t.Run("zero int fallback", func(t *testing.T) {
		prg := mustCompile(t, env, `default(42, 0)`)
		got := mustEval(t, prg, map[string]any{})
		if got != int64(42) {
			t.Errorf("got %v (%T)", got, got)
		}
	})

	t.Run("non-zero int kept", func(t *testing.T) {
		prg := mustCompile(t, env, `default(42, 7)`)
		got := mustEval(t, prg, map[string]any{})
		if got != int64(7) {
			t.Errorf("got %v (%T)", got, got)
		}
	})

	t.Run("false fallback", func(t *testing.T) {
		prg := mustCompile(t, env, `default(true, false)`)
		got := mustEval(t, prg, map[string]any{})
		if got != true {
			t.Errorf("got %v", got)
		}
	})

	t.Run("empty list fallback", func(t *testing.T) {
		prg := mustCompile(t, env, `default("none", [])`)
		got := mustEval(t, prg, map[string]any{})
		if got != "none" {
			t.Errorf("got %v", got)
		}
	})

	t.Run("non-empty list kept", func(t *testing.T) {
		prg := mustCompile(t, env, `default("none", [1, 2, 3]).size() == 3`)
		got, err := EvalCELBool(prg, map[string]any{}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if !got {
			t.Error("expected true")
		}
	})
}

func TestDefaultWithEvent(t *testing.T) {
	env := mustNewCELEnv(t)
	event := map[string]any{
		"name": "test",
		"tag":  "",
	}
	prg := mustCompile(t, env, `default("latest", event.tag)`)
	got := mustEval(t, prg, event)
	if got != "latest" {
		t.Errorf("got %v", got)
	}
}
