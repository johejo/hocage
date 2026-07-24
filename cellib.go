package main

import "github.com/google/cel-go/cel"

type hocageLib struct{}

func HocageLibrary() cel.EnvOption {
	return cel.Lib(&hocageLib{})
}

func hocageSubLibraries() []cel.Library {
	return []cel.Library{
		&fsLib{},
		&gitLib{},
		&envLib{},
		&globLib{},
		&mapLib{},
		&strLib{},
		&shLib{},
		&listLib{},
		&semverLib{},
		&encodingLib{},
		&defaultLib{},
		&cryptoLib{},
		&transcriptLib{},
	}
}

func (l *hocageLib) CompileOptions() []cel.EnvOption {
	var opts []cel.EnvOption
	for _, sub := range hocageSubLibraries() {
		opts = append(opts, sub.CompileOptions()...)
	}
	return opts
}

func (l *hocageLib) ProgramOptions() []cel.ProgramOption {
	var opts []cel.ProgramOption
	for _, sub := range hocageSubLibraries() {
		opts = append(opts, sub.ProgramOptions()...)
	}
	return opts
}
