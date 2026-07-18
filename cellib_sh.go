package main

import (
	"path/filepath"
	"strings"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"mvdan.cc/sh/v3/expand"
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
		cel.Function("sh_argv",
			cel.Overload("sh_argv_string",
				[]*cel.Type{cel.StringType},
				cel.ListType(cel.ListType(cel.StringType)),
				cel.UnaryBinding(shArgvImpl),
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

// wordLiteral extracts the static literal value of a word, resolving quoting
// and escaping one level per parse like a real shell, so the payload of
// bash -c "bash -c \"rm x\"" re-parses as valid shell. Command and process
// substitutions are dropped — expand.Literal cannot evaluate them, and the
// outer walk visits their commands directly.
func wordLiteral(w *syntax.Word) string {
	stripped := &syntax.Word{Parts: stripUnexpandable(w.Parts)}
	if s, err := expand.Literal(nil, stripped); err == nil {
		return s
	}
	var b strings.Builder
	collectWordParts(&b, w.Parts)
	return b.String()
}

func stripUnexpandable(parts []syntax.WordPart) []syntax.WordPart {
	out := make([]syntax.WordPart, 0, len(parts))
	for _, part := range parts {
		switch p := part.(type) {
		case *syntax.CmdSubst, *syntax.ProcSubst:
		case *syntax.DblQuoted:
			out = append(out, &syntax.DblQuoted{Dollar: p.Dollar, Parts: stripUnexpandable(p.Parts)})
		default:
			out = append(out, part)
		}
	}
	return out
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

// maxShellRecursionDepth bounds how many nested inline script bodies
// (sh -c '...' payloads, heredocs piped to a shell) are re-parsed. Deeper
// nesting is silently ignored (fail-open).
const maxShellRecursionDepth = 5

var shellInterpreterNames = map[string]bool{
	"sh": true, "bash": true, "zsh": true, "dash": true, "ksh": true, "mksh": true,
}

// walkShellCalls parses src and invokes onCall for every simple command with
// at least one argument. When the invoked program is a shell interpreter
// (matched by basename), inline script bodies are re-parsed as shell and
// walked too: the -c string payload, heredoc bodies (<<, <<-), and herestrings
// (<<<). Heredocs on non-shell commands (e.g. cat <<EOF) are not re-parsed.
//
// Nothing is counted twice: a -c payload is a quoted word in the outer tree,
// never a CallExpr, and command substitutions inside heredoc bodies are
// visited by the outer walk but stripped from the re-parsed literal.
func walkShellCalls(src string, depth int, onCall func(*syntax.CallExpr)) {
	if depth <= 0 {
		return
	}
	file, err := parseSh(src)
	if err != nil {
		return
	}
	syntax.Walk(file, func(node syntax.Node) bool {
		stmt, ok := node.(*syntax.Stmt)
		if !ok {
			return true
		}
		call, ok := stmt.Cmd.(*syntax.CallExpr)
		if !ok || len(call.Args) == 0 {
			return true
		}
		onCall(call)
		if !shellInterpreterNames[filepath.Base(wordLiteral(call.Args[0]))] {
			return true
		}
		if payload, ok := dashCPayload(call.Args[1:]); ok {
			walkShellCalls(payload, depth-1, onCall)
		}
		for _, r := range stmt.Redirs {
			switch r.Op {
			case syntax.Hdoc, syntax.DashHdoc:
				if r.Hdoc != nil {
					walkShellCalls(wordLiteral(r.Hdoc), depth-1, onCall)
				}
			case syntax.WordHdoc:
				if r.Word != nil {
					walkShellCalls(wordLiteral(r.Word), depth-1, onCall)
				}
			}
		}
		return true
	})
}

// dashCPayload scans interpreter arguments for the -c flag and returns the
// command-string operand that follows the option cluster containing 'c'.
// Scanning stops at the first operand, so `bash script.sh` never matches.
func dashCPayload(args []*syntax.Word) (string, bool) {
	for i := 0; i < len(args); i++ {
		lit := wordLiteral(args[i])
		if lit == "--" || lit == "-" || !strings.HasPrefix(lit, "-") {
			return "", false
		}
		if strings.HasPrefix(lit, "--") {
			continue
		}
		cluster := lit[1:]
		if strings.ContainsRune(cluster, 'c') {
			if i+1 < len(args) {
				return wordLiteral(args[i+1]), true
			}
			return "", false
		}
		if strings.HasSuffix(cluster, "o") {
			i++ // -o takes a value (e.g. -euo pipefail)
		}
	}
	return "", false
}

func shCommandsImpl(arg ref.Val) ref.Val {
	cmd, ok := arg.Value().(string)
	if !ok {
		return types.NewErr("sh_commands: argument must be string")
	}
	names := []string{}
	walkShellCalls(cmd, maxShellRecursionDepth, func(call *syntax.CallExpr) {
		names = append(names, wordLiteral(call.Args[0]))
	})
	return types.DefaultTypeAdapter.NativeToValue(names)
}

func shWordsImpl(arg ref.Val) ref.Val {
	cmd, ok := arg.Value().(string)
	if !ok {
		return types.NewErr("sh_words: argument must be string")
	}
	words := []string{}
	walkShellCalls(cmd, maxShellRecursionDepth, func(call *syntax.CallExpr) {
		for _, w := range call.Args {
			words = append(words, wordLiteral(w))
		}
	})
	return types.DefaultTypeAdapter.NativeToValue(words)
}

// shArgvImpl returns the quote-stripped argv of every simple command, one
// list per command. Deliberately no -c/heredoc recursion: merged argvs would
// lose which nesting level an entry came from, defeating structural checks
// like "is this an interpreter invoking a script file".
func shArgvImpl(arg ref.Val) ref.Val {
	cmd, ok := arg.Value().(string)
	if !ok {
		return types.NewErr("sh_argv: argument must be string")
	}
	argvs := [][]string{}
	file, err := parseSh(cmd)
	if err != nil {
		return types.DefaultTypeAdapter.NativeToValue(argvs)
	}
	syntax.Walk(file, func(node syntax.Node) bool {
		if call, ok := node.(*syntax.CallExpr); ok && len(call.Args) > 0 {
			argv := make([]string, 0, len(call.Args))
			for _, w := range call.Args {
				argv = append(argv, wordLiteral(w))
			}
			argvs = append(argvs, argv)
		}
		return true
	})
	return types.DefaultTypeAdapter.NativeToValue(argvs)
}

func shValidImpl(arg ref.Val) ref.Val {
	cmd, ok := arg.Value().(string)
	if !ok {
		return types.NewErr("sh_valid: argument must be string")
	}
	_, err := parseSh(cmd)
	return types.Bool(err == nil)
}
