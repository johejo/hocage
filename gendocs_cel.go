package main

import (
	"fmt"
	"strings"

	"github.com/google/cel-go/common/decls"
)

// celFuncDoc documents one custom CEL function for the generated
// cel-functions.md. Name must match a function registered by HocageLibrary;
// TestCELFuncDocsComplete keeps the mapping complete in both directions.
type celFuncDoc struct {
	Name string
	// Sig overrides the signature derived from the CEL env. Set it only for
	// functions whose declared types are dyn but accept a narrower shape in
	// practice (e.g. keys takes a map, not any dyn).
	Sig string
	// Doc is the one-line markdown description ("|" escaped as "\|").
	Doc string
}

// celFuncGroup is one "###" section under "Custom hocage Functions".
type celFuncGroup struct {
	Name  string
	Intro string // prose before the function table
	Outro string // prose after the function table
	Funcs []celFuncDoc
}

// lines joins its arguments with newlines, so multi-line prose blocks read
// exactly as they render in the generated markdown.
func lines(l ...string) string {
	return strings.Join(l, "\n")
}

var celFuncGroups = []celFuncGroup{
	{
		Name: "Filesystem",
		Funcs: []celFuncDoc{
			{Name: "file_exists", Doc: "Returns true if path exists and is a file (not directory)"},
			{Name: "dir_exists", Doc: "Returns true if path exists and is a directory"},
			{Name: "is_symlink", Doc: "Returns true if path is a symbolic link (`os.Lstat` + `ModeSymlink`)"},
			{Name: "read_file", Doc: "File contents as UTF-8 text. Returns `\"\"` on any failure (fail-open): missing, not a regular file (directory, device, fifo), unreadable, larger than 1 MiB, or invalid UTF-8. Follows symlinks. Relative paths resolve against the hook process cwd, which may differ from the Bash tool's persistent shell cwd — prefer absolute paths"},
			{Name: "read_file_ok", Doc: "Returns true iff `read_file` would return the file's actual contents (true for an empty file, false on any of the failures above). Turns `read_file`'s fail-open `\"\"` into a fail-closed deny: `!read_file_ok(p) \\|\\| \"rm\" in sh_commands(read_file(p))`"},
		},
	},
	{
		Name: "Git",
		Funcs: []celFuncDoc{
			{Name: "git_tracked", Doc: "Returns true if the file is tracked by git (`git ls-files`)"},
			{Name: "git_branch", Doc: "Returns current branch name (`HEAD` when detached)"},
			{Name: "git_ignored", Doc: "Returns true if the path is ignored by `.gitignore`"},
			{Name: "git_modified", Doc: "Returns true if the file has unstaged changes"},
			{Name: "git_staged", Doc: "Returns true if the file has staged changes"},
		},
	},
	{
		Name: "Shell / Command",
		Intro: lines(
			"Parse a shell command string with a real bash parser (`mvdan.cc/sh/v3`) instead of",
			"fragile substring matching. `sh_commands` and `sh_words` strip quotes and ignore",
			"whitespace noise, so `echo \"rm -rf /\"` does not match `rm` while `sudo  rm  -rf /` does.",
		),
		Funcs: []celFuncDoc{
			{Name: "sh_commands", Doc: "Directly-invoked program names (first word of each simple command), recursing through pipes, `&&`/`\\|\\|`, subshells, command substitutions, and inline script bodies (see below). Returns `[]` if the command does not parse"},
			{Name: "sh_words", Doc: "All argument words across every simple command, as quote-stripped literals. Catches a program anywhere (e.g. behind `sudo`/`xargs`). Recurses like `sh_commands`. Returns `[]` if the command does not parse"},
			{Name: "sh_argv", Doc: "Quote-stripped argv per simple command, for structural inspection — e.g. finding the script-file operand of `bash x.sh`. Does NOT recurse into inline script bodies. Returns `[]` if the command does not parse"},
			{Name: "sh_valid", Doc: "Returns true if the command parses as valid shell syntax"},
		},
		Outro: lines(
			"`sh_commands` matches the program actually run (`\"curl\" in sh_commands(cmd)` is false",
			"for `echo curl`, true for `bash -c 'curl x'`). `sh_words` matches a token in any",
			"position (`\"rm\" in sh_words(cmd)` is true for `sudo rm -rf`, false for `echo \"rm -rf\"`).",
			"`sh_argv` preserves per-command argv structure; combine with `read_file` to also scan",
			"script files the command executes (see the patterns reference).",
			"",
			"Inline script bodies attached to a shell interpreter (`sh`, `bash`, `zsh`, `dash`,",
			"`ksh`, `mksh`, matched by basename) are re-parsed as shell, up to 5 levels deep,",
			"even behind common wrappers (`sudo`, `doas`, `env`, `nice`, `ionice`, `nohup`,",
			"`setsid`, `stdbuf`, `timeout`, `xargs`, `command`, `chroot`, `flock`, `exec`).",
			"Two kinds of bodies are re-parsed:",
			"",
			"- **`-c` payloads**: when a `-c` flag appears anywhere in the interpreter's",
			"  arguments, *every* operand is re-parsed (over-scan) — flag quirks like",
			"  `bash -c -x '...'` cannot hide a payload, at the cost of option values and",
			"  positional parameters appearing as extra command names.",
			"- **Heredoc/herestring bodies** (`<<`, `<<-`, `<<<`): re-parsed only when they",
			"  are the program source (no `-c`, and no operand or an `-s` flag). Heredocs",
			"  that are merely stdin data (`bash script.sh <<EOF`) are not.",
			"",
			"Quoting resolves one level per parse like a real shell, so `bash -c \"bash -c \\\"rm x\\\"\"`",
			"unwraps correctly. Word literalization is host-independent: parameter/arithmetic",
			"expansions resolve to `\"\"`, command/process substitutions are dropped from words",
			"(their inner commands are still visited), tilde stays literal. A fully non-literal",
			"word (`$VAR`, `$(...)`, `<(...)`) comes out as `\"\"` — in `sh_argv` that marks a",
			"runtime-generated operand, which structural rules can deny fail-closed (see the",
			"patterns reference).",
			"",
			"These are strong heuristics, not a guarantee: only static tokens are inspected, so",
			"runtime-constructed commands (`$(echo rm) -rf`, `eval`, base64 payloads),",
			"non-argument positions (assignment RHS, redirect targets), shells reached outside",
			"argv (`su -c`, `find -exec`, `ssh host '...'`), and variable-assembled payloads are",
			"not surfaced. Heredocs on non-shell commands (`cat <<EOF`) are intentionally not",
			"re-parsed. On-disk scripts are not read implicitly — compose `read_file`. Treat this",
			"as a robust first line of defense, not a sandbox.",
		),
	},
	{
		Name: "Environment",
		Funcs: []celFuncDoc{
			{Name: "env", Doc: "Returns the value of an environment variable (empty string if unset)"},
		},
	},
	{
		Name: "Glob",
		Funcs: []celFuncDoc{
			{Name: "glob_exists", Doc: "Returns true if the glob pattern matches at least one file. Uses `filepath.Glob` (no `**` recursive support)"},
		},
	},
	{
		Name: "String / Path",
		Funcs: []celFuncDoc{
			{Name: "trim_prefix", Doc: "Remove prefix from string"},
			{Name: "trim_suffix", Doc: "Remove suffix from string"},
			{Name: "path_base", Doc: "Last element of path (`filepath.Base`)"},
			{Name: "path_dir", Doc: "Directory of path (`filepath.Dir`)"},
			{Name: "path_ext", Doc: "File extension (`filepath.Ext`)"},
			{Name: "path_clean", Doc: "Clean path (`filepath.Clean`)"},
			{Name: "path_join", Doc: "Join path elements (`filepath.Join`)"},
			{Name: "quote", Doc: "Double-quote a string (Go `%q` format)"},
			{Name: "squote", Doc: "Single-quote a string (shell-safe with escaped inner quotes)"},
			{Name: "indent", Doc: "Indent each non-empty line by N spaces"},
		},
	},
	{
		Name: "Map",
		Funcs: []celFuncDoc{
			{Name: "keys", Sig: "keys(map) -> list(string)", Doc: "Sorted list of map keys"},
			{Name: "values", Sig: "values(map) -> list(dyn)", Doc: "List of map values (ordered by sorted keys)"},
			{Name: "to_entries", Sig: "to_entries(map) -> list({\"key\": string, \"value\": dyn})", Doc: "Convert map to list of key-value entries"},
			{Name: "from_entries", Sig: "from_entries(list) -> map", Doc: "Convert list of `{\"key\": k, \"value\": v}` entries to map"},
			{Name: "has_key", Sig: "has_key(map, string) -> bool", Doc: "Check if map contains a key"},
		},
	},
	{
		Name: "List",
		Funcs: []celFuncDoc{
			{Name: "min", Sig: "min(list) -> dyn", Doc: "Minimum value in list (elements must be comparable)"},
			{Name: "max", Sig: "max(list) -> dyn", Doc: "Maximum value in list (elements must be comparable)"},
		},
	},
	{
		Name: "Transcript",
		Intro: lines(
			"Flatten real Claude Code transcript entries (loaded via `transcript.load: true`)",
			"so `when` expressions don't have to navigate the raw JSONL shape. In real",
			"transcripts, tool calls live inside assistant entries as `message.content[]`",
			"blocks of type `tool_use`, results arrive later as `tool_result` blocks plus a",
			"top-level `toolUseResult`, and non-message lines (`mode`,",
			"`file-history-snapshot`, ...) are interleaved — these helpers skip all of that",
			"safely.",
		),
		Funcs: []celFuncDoc{
			{Name: "tool_calls", Sig: "tool_calls(transcript) -> list({\"id\": string, \"name\": string, \"input\": map, \"result\": map})", Doc: "Tool calls in transcript order, each joined with its result by `tool_use_id` when one exists. `result` (absent if the call has no result yet) has `is_error` (bool), `content` (what the model saw), and the fields of `toolUseResult` (e.g. `stdout`/`stderr` for Bash). With `transcript.order: reverse`, `tool_calls(transcript)[0]` is the most recent call"},
			{Name: "user_messages", Sig: "user_messages(transcript) -> list(string)", Doc: "Text of real user messages in transcript order. Skips meta entries (`isMeta: true`) and tool_result-only entries; text blocks within one message are joined with newlines"},
		},
	},
	{
		Name: "Semver",
		Funcs: []celFuncDoc{
			{Name: "semver_compare", Doc: "Check if version (2nd arg) satisfies constraint (1st arg). Uses Masterminds/semver syntax (e.g. `\">= 1.0.0\"`, `\"~1.2\"`, `\"^2.0\"`)"},
		},
	},
	{
		Name: "Encoding",
		Funcs: []celFuncDoc{
			{Name: "to_json", Doc: "Serialize any value to JSON string"},
			{Name: "from_json", Doc: "Parse JSON string to CEL value"},
		},
	},
	{
		Name: "Default",
		Funcs: []celFuncDoc{
			{Name: "default", Doc: "Returns 2nd arg if non-empty, otherwise 1st arg. Empty = `\"\"`, `false`, `0`, `nil`, `[]`, `{}`, or error"},
		},
	},
	{
		Name: "Crypto",
		Funcs: []celFuncDoc{
			{Name: "sha256sum", Doc: "SHA-256 hex digest of a string"},
		},
	},
}

// stdExtDoc documents one cel-go extension enabled in baseEnvOptions.
// TestStdExtDocsComplete keeps LibName in sync with the env's Libraries().
type stdExtDoc struct {
	LibName string // cel-go singleton library name, e.g. "cel.lib.ext.strings"
	Title   string
	Doc     string
}

var stdExtDocs = []stdExtDoc{
	{
		LibName: "cel.lib.ext.strings",
		Title:   "ext.Strings()",
		Doc:     "String manipulation: `charAt`, `indexOf`, `lastIndexOf`, `join`, `lowerAscii`, `upperAscii`, `replace`, `split`, `substring`, `trim`, `reverse`, `quote`.",
	},
	{
		LibName: "cel.lib.ext.lists",
		Title:   "ext.Lists() (v3)",
		Doc:     "List operations: `slice`, `flatten`, `sort`, `distinct`, `range`.",
	},
	{
		LibName: "cel.lib.ext.sets",
		Title:   "ext.Sets()",
		Doc:     "Set operations on lists: `sets.contains`, `sets.intersects`, `sets.equivalent`.",
	},
	{
		LibName: "cel.lib.ext.math",
		Title:   "ext.Math()",
		Doc:     "Math functions: `math.greatest`, `math.least`, `math.ceil`, `math.floor`, `math.round`, `math.abs`, `math.sign`, `math.isNaN`, `math.isInf`, `math.bitAnd`, `math.bitOr`, `math.bitXor`, `math.bitNot`, `math.bitShiftLeft`, `math.bitShiftRight`.",
	},
	{
		LibName: "cel.lib.ext.encoders",
		Title:   "ext.Encoders()",
		Doc:     "Encoding: `base64.encode`, `base64.decode`.",
	},
	{
		LibName: "cel.lib.ext.regex",
		Title:   "ext.Regex()",
		Doc:     "Regex: `re.capture`, `re.captureN`.",
	},
	{
		LibName: "cel.lib.ext.cel.bindings",
		Title:   "ext.Bindings()",
		Doc:     "Variable binding: `cel.bind(var, expr, body)` — bind intermediate results to avoid repeated computation.",
	},
	{
		LibName: "cel.lib.ext.comprev2",
		Title:   "ext.TwoVarComprehensions()",
		Doc:     "Two-variable comprehension macros: `transformList`, `transformMap`, `existsAll` with two iterators.",
	},
	{
		LibName: "cel.lib.optional",
		Title:   "cel.OptionalTypes()",
		Doc:     "Optional value handling: `optional.of`, `optional.none`, `optional.ofNonZeroValue`, `.hasValue()`, `.value()`, `.or()`, `.orValue()`.",
	},
}

// formatStdExtList renders the README one-liner listing the enabled standard
// CEL extensions, derived from stdExtDocs.
func formatStdExtList() []byte {
	titles := make([]string, len(stdExtDocs))
	for i, e := range stdExtDocs {
		titles[i] = e.Title
	}
	return []byte("Standard CEL extensions are also enabled: " + backtickJoin(titles) + ".\n")
}

var celBuiltinsSection = lines(
	"## Built-in CEL Operations",
	"",
	"Always available without extensions:",
	"- Comparison: `==`, `!=`, `<`, `<=`, `>`, `>=`",
	"- Logic: `&&`, `||`, `!`",
	"- Arithmetic: `+`, `-`, `*`, `/`, `%`",
	"- String: `.contains()`, `.startsWith()`, `.endsWith()`, `.matches()` (RE2 regex), `.size()`",
	"- List: `in`, `.size()`, `.exists()`, `.all()`, `.filter()`, `.map()`",
	"- Map: `has()`, `in`",
	"- Ternary: `condition ? a : b`",
)

func generateCELFunctionDocs() ([]byte, error) {
	env, err := NewCELEnv()
	if err != nil {
		return nil, err
	}
	fns := env.Functions()
	var b strings.Builder
	b.WriteString(generatedHeader("gendocs_cel.go"))
	b.WriteString("\n")
	b.WriteString("# CEL Functions Reference\n\n")
	b.WriteString("## Custom hocage Functions\n\n")
	for _, g := range celFuncGroups {
		fmt.Fprintf(&b, "### %s\n\n", g.Name)
		if g.Intro != "" {
			b.WriteString(g.Intro)
			b.WriteString("\n\n")
		}
		b.WriteString("| Function | Signature | Description |\n")
		b.WriteString("|----------|-----------|-------------|\n")
		for _, f := range g.Funcs {
			fn := fns[f.Name]
			if fn == nil {
				return nil, fmt.Errorf("celFuncGroups documents %q, which is not registered in the CEL env", f.Name)
			}
			sig := f.Sig
			if sig == "" {
				sig = derivedSignature(fn)
			}
			fmt.Fprintf(&b, "| `%s` | `%s` | %s |\n", f.Name, sig, f.Doc)
		}
		b.WriteString("\n")
		if g.Outro != "" {
			b.WriteString(g.Outro)
			b.WriteString("\n\n")
		}
	}
	b.WriteString("## Standard CEL Extensions\n\nhocage enables these cel-go standard extensions:\n\n")
	for _, e := range stdExtDocs {
		fmt.Fprintf(&b, "### %s\n%s\n\n", e.Title, e.Doc)
	}
	b.WriteString(celBuiltinsSection)
	b.WriteString("\n")
	return []byte(b.String()), nil
}

// derivedSignature renders a function's overloads in doc notation, e.g.
// "file_exists(string) -> bool". Multiple overloads are joined with "<br>".
func derivedSignature(fn *decls.FunctionDecl) string {
	var sigs []string
	for _, o := range fn.OverloadDecls() {
		args := make([]string, len(o.ArgTypes()))
		for i, t := range o.ArgTypes() {
			args[i] = t.String()
		}
		var sig string
		if o.IsMemberFunction() {
			sig = fmt.Sprintf("%s.%s(%s) -> %s", args[0], fn.Name(), strings.Join(args[1:], ", "), o.ResultType())
		} else {
			sig = fmt.Sprintf("%s(%s) -> %s", fn.Name(), strings.Join(args, ", "), o.ResultType())
		}
		sigs = append(sigs, sig)
	}
	return strings.Join(sigs, "<br>")
}
