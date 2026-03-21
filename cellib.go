package main

import "github.com/google/cel-go/cel"

type agcelLib struct{}

func AgcelLibrary() cel.EnvOption {
	return cel.Lib(&agcelLib{})
}

func (l *agcelLib) CompileOptions() []cel.EnvOption {
	var opts []cel.EnvOption
	opts = append(opts, (&fsLib{}).CompileOptions()...)
	opts = append(opts, (&gitLib{}).CompileOptions()...)
	opts = append(opts, (&envLib{}).CompileOptions()...)
	opts = append(opts, (&globLib{}).CompileOptions()...)
	return opts
}

func (l *agcelLib) ProgramOptions() []cel.ProgramOption {
	var opts []cel.ProgramOption
	opts = append(opts, (&fsLib{}).ProgramOptions()...)
	opts = append(opts, (&gitLib{}).ProgramOptions()...)
	opts = append(opts, (&envLib{}).ProgramOptions()...)
	opts = append(opts, (&globLib{}).ProgramOptions()...)
	return opts
}
