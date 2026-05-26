package cogo

import (
	"fmt"
	"strings"

	"lsurvey/internal/project"
)

func execUnits(p *project.Project, f []string) (Result, error) {
	if len(f) < 2 {
		return Result{}, fmt.Errorf("usage: units key=value ...")
	}
	updates := make(map[string]string, len(f)-1)
	for _, arg := range f[1:] {
		k, v, ok := strings.Cut(arg, "=")
		if !ok {
			return Result{}, fmt.Errorf("unit argument %q must be key=value", arg)
		}
		updates[k] = v
	}
	for k, v := range updates {
		p.Units[k] = v
	}
	return Result{Message: "updated units"}, nil
}
