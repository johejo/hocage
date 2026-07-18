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
// and escaping one level per parse like a real shell. Expansions resolve
// host-independently: parameter/arithmetic expansions become "", command and
// process substitutions are dropped (the outer walk visits their commands
// directly), and tilde stays literal.
func wordLiteral(w *syntax.Word) string {
	stripped := &syntax.Word{Parts: stripUnexpandable(w.Parts)}
	if s, err := expand.Document(nil, stripped); err == nil {
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
		case *syntax.CmdSubst, *syntax.ProcSubst, *syntax.ParamExp, *syntax.ArithmExp:
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

// interpreterWrappers execute their remaining arguments, so a shell can hide
// behind them (sudo bash -c '...'). time needs no entry: its inner statement
// is a TimeClause the walk visits directly.
var interpreterWrappers = map[string]bool{
	"sudo": true, "doas": true, "env": true, "nice": true, "ionice": true,
	"nohup": true, "setsid": true, "stdbuf": true, "timeout": true,
	"xargs": true, "command": true, "chroot": true, "flock": true,
	"exec": true,
}

// interpreterIndex returns the index of the shell-interpreter word in args
// (matched by basename), looking behind known wrapper programs, or -1.
func interpreterIndex(args []*syntax.Word, name string) int {
	if shellInterpreterNames[filepath.Base(name)] {
		return 0
	}
	if interpreterWrappers[filepath.Base(name)] {
		for j := 1; j < len(args); j++ {
			if shellInterpreterNames[filepath.Base(wordLiteral(args[j]))] {
				return j
			}
		}
	}
	return -1
}

// interpArgs summarizes an interpreter's argument list. Rather than mimic
// each shell's exact flag parsing, every operand is collected and — when a -c
// flag appears anywhere — re-parsed as shell: a mis-classified word is noise,
// never a skipped payload.
type interpArgs struct {
	sawC       bool     // a 'c' flag appeared in an option cluster
	sawS       bool     // an 's' flag appeared: stdin is the program even with operands
	operands   []string // non-flag words, re-parsed as shell when sawC
	hasOperand bool     // any non-empty operand (script file, positional param)
}

func (r *interpArgs) addOperand(lit string) {
	r.operands = append(r.operands, lit)
	if lit != "" {
		r.hasOperand = true
	}
}

func scanInterpreterArgs(args []*syntax.Word) interpArgs {
	var r interpArgs
	rest := false // after "--"
	for i := 0; i < len(args); i++ {
		lit := wordLiteral(args[i])
		switch {
		case rest:
			r.addOperand(lit)
		case lit == "--":
			rest = true
		case lit == "" || (!strings.HasPrefix(lit, "-") && !strings.HasPrefix(lit, "+")):
			r.addOperand(lit)
		case strings.HasPrefix(lit, "--"):
			// Long option: skip; a separate-word value (--rcfile X) becomes
			// a scanned operand.
		default:
			// Option cluster. A lone "-" lands here and must not count as an
			// operand: shells treat it as end-of-options and still run a
			// heredoc as the program.
			cluster := lit[1:]
			if strings.ContainsRune(cluster, 'c') {
				r.sawC = true
			}
			if strings.ContainsRune(cluster, 's') {
				r.sawS = true
			}
			// -o/-O take a value; consume the next word only when the letter
			// ends the cluster (else the value is inline). This cannot hide a
			// payload: an option in the value slot (-o -c) is an invalid
			// option name and the shell refuses to run at all.
			if n := len(cluster); n > 0 && (cluster[n-1] == 'o' || cluster[n-1] == 'O') {
				i++
			}
		}
	}
	return r
}

// walkShellCalls parses src and invokes onCall with every simple command that
// has at least one argument, along with its literalized program name. When a
// shell interpreter is invoked — directly or behind a wrapper — inline script
// bodies are re-parsed as shell and walked too: every operand when a -c flag
// is present, and heredoc/herestring bodies when they are the program source
// (no -c, and no operand or an -s flag). Heredocs that are stdin data
// (bash -c '...' <<EOF, bash script.sh <<EOF) or attached to a non-shell
// command (cat <<EOF) are not re-parsed.
//
// Nothing is counted twice: a -c payload is a quoted word in the outer tree,
// never a CallExpr, and command substitutions inside heredoc bodies are
// visited by the outer walk but stripped from the re-parsed literal.
func walkShellCalls(src string, depth int, onCall func(call *syntax.CallExpr, name string)) {
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
		name := wordLiteral(call.Args[0])
		onCall(call, name)
		idx := interpreterIndex(call.Args, name)
		if idx < 0 {
			return true
		}
		info := scanInterpreterArgs(call.Args[idx+1:])
		if info.sawC {
			for _, p := range info.operands {
				walkShellCalls(p, depth-1, onCall)
			}
			return true
		}
		if !info.hasOperand || info.sawS {
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
		}
		return true
	})
}

func shCommandsImpl(arg ref.Val) ref.Val {
	cmd, ok := arg.Value().(string)
	if !ok {
		return types.NewErr("sh_commands: argument must be string")
	}
	names := []string{}
	walkShellCalls(cmd, maxShellRecursionDepth, func(_ *syntax.CallExpr, name string) {
		names = append(names, name)
	})
	return types.DefaultTypeAdapter.NativeToValue(names)
}

func shWordsImpl(arg ref.Val) ref.Val {
	cmd, ok := arg.Value().(string)
	if !ok {
		return types.NewErr("sh_words: argument must be string")
	}
	words := []string{}
	walkShellCalls(cmd, maxShellRecursionDepth, func(call *syntax.CallExpr, name string) {
		words = append(words, name)
		for _, w := range call.Args[1:] {
			words = append(words, wordLiteral(w))
		}
	})
	return types.DefaultTypeAdapter.NativeToValue(words)
}

// shArgvImpl returns the quote-stripped argv of every simple command, one
// list per command. Depth 1 deliberately suppresses -c/heredoc recursion:
// merged argvs would lose which nesting level an entry came from, defeating
// structural checks like "is this an interpreter invoking a script file".
func shArgvImpl(arg ref.Val) ref.Val {
	cmd, ok := arg.Value().(string)
	if !ok {
		return types.NewErr("sh_argv: argument must be string")
	}
	argvs := [][]string{}
	walkShellCalls(cmd, 1, func(call *syntax.CallExpr, name string) {
		argv := make([]string, 0, len(call.Args))
		argv = append(argv, name)
		for _, w := range call.Args[1:] {
			argv = append(argv, wordLiteral(w))
		}
		argvs = append(argvs, argv)
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
