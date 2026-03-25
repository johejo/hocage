# CEL Functions Reference

## Custom hocage Functions

### Filesystem

| Function | Signature | Description |
|----------|-----------|-------------|
| `file_exists` | `file_exists(string) -> bool` | Returns true if path exists and is a file (not directory) |
| `dir_exists` | `dir_exists(string) -> bool` | Returns true if path exists and is a directory |

### Git

| Function | Signature | Description |
|----------|-----------|-------------|
| `git_tracked` | `git_tracked(string) -> bool` | Returns true if the file is tracked by git (`git ls-files`) |

### Environment

| Function | Signature | Description |
|----------|-----------|-------------|
| `env` | `env(string) -> string` | Returns the value of an environment variable (empty string if unset) |

### Glob

| Function | Signature | Description |
|----------|-----------|-------------|
| `glob_exists` | `glob_exists(string) -> bool` | Returns true if the glob pattern matches at least one file. Uses `filepath.Glob` (no `**` recursive support) |

### String / Path

| Function | Signature | Description |
|----------|-----------|-------------|
| `trim_prefix` | `trim_prefix(string, string) -> string` | Remove prefix from string |
| `trim_suffix` | `trim_suffix(string, string) -> string` | Remove suffix from string |
| `path_base` | `path_base(string) -> string` | Last element of path (`filepath.Base`) |
| `path_dir` | `path_dir(string) -> string` | Directory of path (`filepath.Dir`) |
| `path_ext` | `path_ext(string) -> string` | File extension (`filepath.Ext`) |
| `path_clean` | `path_clean(string) -> string` | Clean path (`filepath.Clean`) |
| `path_join` | `path_join(list(string)) -> string` | Join path elements (`filepath.Join`) |
| `quote` | `quote(string) -> string` | Double-quote a string (Go `%q` format) |
| `squote` | `squote(string) -> string` | Single-quote a string (shell-safe with escaped inner quotes) |
| `indent` | `indent(int, string) -> string` | Indent each non-empty line by N spaces |

### Map

| Function | Signature | Description |
|----------|-----------|-------------|
| `keys` | `keys(map) -> list(string)` | Sorted list of map keys |
| `values` | `values(map) -> list(dyn)` | List of map values (ordered by sorted keys) |
| `to_entries` | `to_entries(map) -> list({"key": string, "value": dyn})` | Convert map to list of key-value entries |
| `from_entries` | `from_entries(list) -> map` | Convert list of `{"key": k, "value": v}` entries to map |
| `has_key` | `has_key(map, string) -> bool` | Check if map contains a key |

### List

| Function | Signature | Description |
|----------|-----------|-------------|
| `min` | `min(list) -> dyn` | Minimum value in list (elements must be comparable) |
| `max` | `max(list) -> dyn` | Maximum value in list (elements must be comparable) |

### Semver

| Function | Signature | Description |
|----------|-----------|-------------|
| `semver_compare` | `semver_compare(string, string) -> bool` | Check if version (2nd arg) satisfies constraint (1st arg). Uses Masterminds/semver syntax (e.g. `">= 1.0.0"`, `"~1.2"`, `"^2.0"`) |

### Encoding

| Function | Signature | Description |
|----------|-----------|-------------|
| `to_json` | `to_json(dyn) -> string` | Serialize any value to JSON string |
| `from_json` | `from_json(string) -> dyn` | Parse JSON string to CEL value |

### Default

| Function | Signature | Description |
|----------|-----------|-------------|
| `default` | `default(dyn, dyn) -> dyn` | Returns 2nd arg if non-empty, otherwise 1st arg. Empty = `""`, `false`, `0`, `nil`, `[]`, `{}`, or error |

### Crypto

| Function | Signature | Description |
|----------|-----------|-------------|
| `sha256sum` | `sha256sum(string) -> string` | SHA-256 hex digest of a string |

## Standard CEL Extensions

hocage enables these cel-go standard extensions:

### ext.Strings()
String manipulation: `charAt`, `indexOf`, `lastIndexOf`, `join`, `lowerAscii`, `upperAscii`, `replace`, `split`, `substring`, `trim`, `reverse`, `quote`.

### ext.Lists() (v3)
List operations: `slice`, `flatten`, `sort`, `distinct`, `range`.

### ext.Sets()
Set operations on lists: `sets.contains`, `sets.intersects`, `sets.equivalent`.

### ext.Math()
Math functions: `math.greatest`, `math.least`, `math.ceil`, `math.floor`, `math.round`, `math.abs`, `math.sign`, `math.isNaN`, `math.isInf`, `math.bitAnd`, `math.bitOr`, `math.bitXor`, `math.bitNot`, `math.bitShiftLeft`, `math.bitShiftRight`.

### ext.Encoders()
Encoding: `base64.encode`, `base64.decode`.

### ext.Regex()
Regex: `re.capture`, `re.captureN`.

### ext.Bindings()
Variable binding: `cel.bind(var, expr, body)` — bind intermediate results to avoid repeated computation.

### ext.TwoVarComprehensions()
Two-variable comprehension macros: `transformList`, `transformMap`, `existsAll` with two iterators.

### cel.OptionalTypes()
Optional value handling: `optional.of`, `optional.none`, `optional.ofNonZeroValue`, `.hasValue()`, `.value()`, `.or()`, `.orValue()`.

## Built-in CEL Operations

Always available without extensions:
- Comparison: `==`, `!=`, `<`, `<=`, `>`, `>=`
- Logic: `&&`, `||`, `!`
- Arithmetic: `+`, `-`, `*`, `/`, `%`
- String: `.contains()`, `.startsWith()`, `.endsWith()`, `.matches()` (RE2 regex), `.size()`
- List: `in`, `.size()`, `.exists()`, `.all()`, `.filter()`, `.map()`
- Map: `has()`, `in`
- Ternary: `condition ? a : b`
