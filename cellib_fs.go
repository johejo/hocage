package main

import (
	"io"
	"os"
	"unicode/utf8"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
)

type fsLib struct{}

func (l *fsLib) CompileOptions() []cel.EnvOption {
	return []cel.EnvOption{
		cel.Function("file_exists",
			cel.Overload("file_exists_string",
				[]*cel.Type{cel.StringType},
				cel.BoolType,
				cel.UnaryBinding(fileExistsImpl),
			),
		),
		cel.Function("dir_exists",
			cel.Overload("dir_exists_string",
				[]*cel.Type{cel.StringType},
				cel.BoolType,
				cel.UnaryBinding(dirExistsImpl),
			),
		),
		cel.Function("is_symlink",
			cel.Overload("is_symlink_string",
				[]*cel.Type{cel.StringType},
				cel.BoolType,
				cel.UnaryBinding(isSymlinkImpl),
			),
		),
		cel.Function("read_file",
			cel.Overload("read_file_string",
				[]*cel.Type{cel.StringType},
				cel.StringType,
				cel.UnaryBinding(readFileImpl),
			),
		),
	}
}

func (l *fsLib) ProgramOptions() []cel.ProgramOption {
	return nil
}

func fileExistsImpl(arg ref.Val) ref.Val {
	path, ok := arg.Value().(string)
	if !ok {
		return types.Bool(false)
	}
	info, err := os.Stat(path)
	if err != nil {
		return types.Bool(false)
	}
	return types.Bool(!info.IsDir())
}

func dirExistsImpl(arg ref.Val) ref.Val {
	path, ok := arg.Value().(string)
	if !ok {
		return types.Bool(false)
	}
	info, err := os.Stat(path)
	if err != nil {
		return types.Bool(false)
	}
	return types.Bool(info.IsDir())
}

func isSymlinkImpl(arg ref.Val) ref.Val {
	path, ok := arg.Value().(string)
	if !ok {
		return types.Bool(false)
	}
	info, err := os.Lstat(path)
	if err != nil {
		return types.Bool(false)
	}
	return types.Bool(info.Mode()&os.ModeSymlink != 0)
}

// maxReadFileSize caps read_file at 1 MiB. Oversize files return "" rather
// than a truncated prefix, which could re-parse as valid but wrong shell.
const maxReadFileSize = 1 << 20

// readFileImpl returns the file contents as UTF-8 text. Any failure —
// missing file, directory, permission error, oversize, invalid UTF-8 —
// returns "" (fail-open). Symlinks are followed.
func readFileImpl(arg ref.Val) ref.Val {
	path, ok := arg.Value().(string)
	if !ok {
		return types.String("")
	}
	f, err := os.Open(path)
	if err != nil {
		return types.String("")
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxReadFileSize+1))
	if err != nil || len(data) > maxReadFileSize || !utf8.Valid(data) {
		return types.String("")
	}
	return types.String(data)
}
