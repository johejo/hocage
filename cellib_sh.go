package main

import (
	"strings"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"mvdan.cc/sh/v3/syntax"
)

type shLib struct{}

func (l *shLib) CompileOptions() []cel.EnvOption {
	return []cel.EnvOption{
		cel.Function("sh_commands",
			cel.Overload("sh_commands_string",
				[]*cel.Type{cel.StringType},
				cel.ListType(cel.StringType),
				cel.UnaryBinding(shCommandsImpl),
			),
		),
		cel.Function("sh_words",
			cel.Overload("sh_words_string",
				[]*cel.Type{cel.StringType},
				cel.ListType(cel.StringType),
				cel.UnaryBinding(shWordsImpl),
			),
		),
		cel.Function("sh_valid",
			cel.Overload("sh_valid_string",
				[]*cel.Type{cel.StringType},
				cel.BoolType,
				cel.UnaryBinding(shValidImpl),
			),
		),
	}
}

func (l *shLib) ProgramOptions() []cel.ProgramOption {
	return nil
}

// parseSh parses a shell command string into a syntax tree using the bash
// dialect. Reads are buffered and parsing is single-shot.
func parseSh(cmd string) (*syntax.File, error) {
	return syntax.NewParser().Parse(strings.NewReader(cmd), "")
}

// wordLiteral extracts the static literal value of a word, stripping quotes.
// Non-literal parts such as parameter expansions or command substitutions are
// skipped, so "rm -rf" yields rm -rf and "foo${bar}" yields foo.
func wordLiteral(w *syntax.Word) string {
	var b strings.Builder
	collectWordParts(&b, w.Parts)
	return b.String()
}

func collectWordParts(b *strings.Builder, parts []syntax.WordPart) {
	for _, part := range parts {
		switch p := part.(type) {
		case *syntax.Lit:
			b.WriteString(p.Value)
		case *syntax.SglQuoted:
			b.WriteString(p.Value)
		case *syntax.DblQuoted:
			collectWordParts(b, p.Parts)
		}
	}
}

func shCommandsImpl(arg ref.Val) ref.Val {
	cmd, ok := arg.Value().(string)
	if !ok {
		return types.NewErr("sh_commands: argument must be string")
	}
	file, err := parseSh(cmd)
	if err != nil {
		return types.DefaultTypeAdapter.NativeToValue([]string{})
	}
	names := []string{}
	syntax.Walk(file, func(node syntax.Node) bool {
		if call, ok := node.(*syntax.CallExpr); ok && len(call.Args) > 0 {
			names = append(names, wordLiteral(call.Args[0]))
		}
		return true
	})
	return types.DefaultTypeAdapter.NativeToValue(names)
}

func shWordsImpl(arg ref.Val) ref.Val {
	cmd, ok := arg.Value().(string)
	if !ok {
		return types.NewErr("sh_words: argument must be string")
	}
	file, err := parseSh(cmd)
	if err != nil {
		return types.DefaultTypeAdapter.NativeToValue([]string{})
	}
	words := []string{}
	syntax.Walk(file, func(node syntax.Node) bool {
		if call, ok := node.(*syntax.CallExpr); ok {
			for _, w := range call.Args {
				words = append(words, wordLiteral(w))
			}
		}
		return true
	})
	return types.DefaultTypeAdapter.NativeToValue(words)
}

func shValidImpl(arg ref.Val) ref.Val {
	cmd, ok := arg.Value().(string)
	if !ok {
		return types.NewErr("sh_valid: argument must be string")
	}
	_, err := parseSh(cmd)
	return types.Bool(err == nil)
}
