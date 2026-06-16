package main

import "github.com/google/cel-go/cel"

type hocageLib struct{}

func HocageLibrary() cel.EnvOption {
	return cel.Lib(&hocageLib{})
}

func (l *hocageLib) CompileOptions() []cel.EnvOption {
	var opts []cel.EnvOption
	opts = append(opts, (&fsLib{}).CompileOptions()...)
	opts = append(opts, (&gitLib{}).CompileOptions()...)
	opts = append(opts, (&envLib{}).CompileOptions()...)
	opts = append(opts, (&globLib{}).CompileOptions()...)
	opts = append(opts, (&mapLib{}).CompileOptions()...)
	opts = append(opts, (&strLib{}).CompileOptions()...)
	opts = append(opts, (&shLib{}).CompileOptions()...)
	opts = append(opts, (&listLib{}).CompileOptions()...)
	opts = append(opts, (&semverLib{}).CompileOptions()...)
	opts = append(opts, (&encodingLib{}).CompileOptions()...)
	opts = append(opts, (&defaultLib{}).CompileOptions()...)
	opts = append(opts, (&cryptoLib{}).CompileOptions()...)
	return opts
}

func (l *hocageLib) ProgramOptions() []cel.ProgramOption {
	var opts []cel.ProgramOption
	opts = append(opts, (&fsLib{}).ProgramOptions()...)
	opts = append(opts, (&gitLib{}).ProgramOptions()...)
	opts = append(opts, (&envLib{}).ProgramOptions()...)
	opts = append(opts, (&globLib{}).ProgramOptions()...)
	opts = append(opts, (&mapLib{}).ProgramOptions()...)
	opts = append(opts, (&strLib{}).ProgramOptions()...)
	opts = append(opts, (&shLib{}).ProgramOptions()...)
	opts = append(opts, (&listLib{}).ProgramOptions()...)
	opts = append(opts, (&semverLib{}).ProgramOptions()...)
	opts = append(opts, (&encodingLib{}).ProgramOptions()...)
	opts = append(opts, (&defaultLib{}).ProgramOptions()...)
	opts = append(opts, (&cryptoLib{}).ProgramOptions()...)
	return opts
}
