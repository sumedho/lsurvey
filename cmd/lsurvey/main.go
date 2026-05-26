package main

import (
	"fmt"
	"os"
	"runtime/debug"

	"lsurvey/internal/app"
	"lsurvey/internal/project"
	"lsurvey/internal/tui"
)

var version = "dev"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) >= 4 && args[0] == "export" {
		return app.ExportProject(args[1], args[2], args[3])
	}

	path := ""
	p := project.New("untitled")
	if len(args) == 1 {
		path = args[0]
		loaded, err := project.Load(path)
		if err != nil {
			return err
		}
		p = loaded
	}
	return tui.RunWithVersion(p, path, resolveVersion())
}

func resolveVersion() string {
	if version != "" && version != "dev" {
		return version
	}
	info, ok := debug.ReadBuildInfo()
	if ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	if version == "" {
		return "dev"
	}
	return version
}
