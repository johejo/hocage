package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestFileExists(t *testing.T) {
	env, err := NewCELEnv()
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(filePath, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		expr string
		want bool
	}{
		{"existing file", `file_exists("` + filePath + `")`, true},
		{"nonexistent file", `file_exists("` + filepath.Join(dir, "nope.txt") + `")`, false},
		{"directory is not a file", `file_exists("` + dir + `")`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prg, err := CompileCEL(env, tt.expr)
			if err != nil {
				t.Fatal(err)
			}
			got, err := EvalCELBool(prg, map[string]any{}, nil)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsSymlink(t *testing.T) {
	env, err := NewCELEnv()
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	target := filepath.Join(dir, "target.txt")
	if err := os.WriteFile(target, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link.txt")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		expr string
		want bool
	}{
		{"symlink", `is_symlink("` + link + `")`, true},
		{"regular file", `is_symlink("` + target + `")`, false},
		{"nonexistent", `is_symlink("` + filepath.Join(dir, "nope.txt") + `")`, false},
		{"directory", `is_symlink("` + dir + `")`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prg, err := CompileCEL(env, tt.expr)
			if err != nil {
				t.Fatal(err)
			}
			got, err := EvalCELBool(prg, map[string]any{}, nil)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestReadFile(t *testing.T) {
	env := mustNewCELEnv(t)

	dir := t.TempDir()
	write := func(name string, data []byte) string {
		t.Helper()
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, data, 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}

	scriptPath := write("script.sh", []byte("#!/bin/bash\nrm -rf /\n"))
	exactPath := write("exact.bin", bytes.Repeat([]byte("a"), maxReadFileSize))
	oversizePath := write("oversize.bin", bytes.Repeat([]byte("a"), maxReadFileSize+1))
	binaryPath := write("binary.bin", []byte{0xff, 0xfe, 0x00})
	linkPath := filepath.Join(dir, "link.sh")
	if err := os.Symlink(scriptPath, linkPath); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		expr string
		want any
	}{
		{"existing file", `read_file("` + scriptPath + `")`, "#!/bin/bash\nrm -rf /\n"},
		{"missing file", `read_file("` + filepath.Join(dir, "nope.sh") + `")`, ""},
		{"directory", `read_file("` + dir + `")`, ""},
		{"exactly at size cap", `read_file("` + exactPath + `").size()`, int64(maxReadFileSize)},
		{"over size cap", `read_file("` + oversizePath + `")`, ""},
		{"invalid utf-8", `read_file("` + binaryPath + `")`, ""},
		{"symlink followed", `read_file("` + linkPath + `")`, "#!/bin/bash\nrm -rf /\n"},
		{"composed with sh_commands", `sh_commands(read_file("` + scriptPath + `"))`, []string{"rm"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prg := mustCompile(t, env, tt.expr)
			got := mustEval(t, prg, map[string]any{})
			if want, ok := tt.want.([]string); ok {
				assertStringList(t, got, want)
				return
			}
			if got != tt.want {
				t.Errorf("got %v (%T), want %v (%T)", got, got, tt.want, tt.want)
			}
		})
	}
}

func TestReadFileOk(t *testing.T) {
	env := mustNewCELEnv(t)

	dir := t.TempDir()
	write := func(name string, data []byte) string {
		t.Helper()
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, data, 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}

	scriptPath := write("script.sh", []byte("#!/bin/bash\nrm -rf /\n"))
	emptyPath := write("empty.sh", nil)
	oversizePath := write("oversize.bin", bytes.Repeat([]byte("a"), maxReadFileSize+1))
	binaryPath := write("binary.bin", []byte{0xff, 0xfe, 0x00})
	missingPath := filepath.Join(dir, "nope.sh")
	linkPath := filepath.Join(dir, "link.sh")
	if err := os.Symlink(scriptPath, linkPath); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		expr string
		want bool
	}{
		{"existing file", `read_file_ok("` + scriptPath + `")`, true},
		{"empty file", `read_file_ok("` + emptyPath + `")`, true},
		{"missing file", `read_file_ok("` + missingPath + `")`, false},
		{"directory", `read_file_ok("` + dir + `")`, false},
		{"over size cap", `read_file_ok("` + oversizePath + `")`, false},
		{"invalid utf-8", `read_file_ok("` + binaryPath + `")`, false},
		{"symlink followed", `read_file_ok("` + linkPath + `")`, true},
		// The fail-closed compose from the patterns reference.
		{"fail closed on missing", `!read_file_ok("` + missingPath + `") || "rm" in sh_commands(read_file("` + missingPath + `"))`, true},
		{"fail closed on clean script", `!read_file_ok("` + emptyPath + `") || "rm" in sh_commands(read_file("` + emptyPath + `"))`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prg := mustCompile(t, env, tt.expr)
			if got := mustEval(t, prg, map[string]any{}); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDirExists(t *testing.T) {
	env, err := NewCELEnv()
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(filePath, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		expr string
		want bool
	}{
		{"existing dir", `dir_exists("` + dir + `")`, true},
		{"nonexistent dir", `dir_exists("` + filepath.Join(dir, "nope") + `")`, false},
		{"file is not a dir", `dir_exists("` + filePath + `")`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prg, err := CompileCEL(env, tt.expr)
			if err != nil {
				t.Fatal(err)
			}
			got, err := EvalCELBool(prg, map[string]any{}, nil)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}
