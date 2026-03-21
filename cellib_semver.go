package main

import (
	"github.com/Masterminds/semver/v3"
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
)

type semverLib struct{}

func (l *semverLib) CompileOptions() []cel.EnvOption {
	return []cel.EnvOption{
		cel.Function("semver_compare",
			cel.Overload("semver_compare_string_string",
				[]*cel.Type{cel.StringType, cel.StringType},
				cel.BoolType,
				cel.BinaryBinding(semverCompareImpl),
			),
		),
	}
}

func (l *semverLib) ProgramOptions() []cel.ProgramOption {
	return nil
}

func semverCompareImpl(lhs, rhs ref.Val) ref.Val {
	constraint, ok := lhs.Value().(string)
	if !ok {
		return types.NewErr("semver_compare: first argument must be string")
	}
	version, ok := rhs.Value().(string)
	if !ok {
		return types.NewErr("semver_compare: second argument must be string")
	}
	c, err := semver.NewConstraint(constraint)
	if err != nil {
		return types.NewErr("semver_compare: invalid constraint %q: %v", constraint, err)
	}
	v, err := semver.NewVersion(version)
	if err != nil {
		return types.NewErr("semver_compare: invalid version %q: %v", version, err)
	}
	return types.Bool(c.Check(v))
}
