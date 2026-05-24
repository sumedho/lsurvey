package main

import (
	"fmt"
	"os"
	"runtime/debug"

	"lsurvey/internal/csvpoints"
	"lsurvey/internal/dxf"
	"lsurvey/internal/geojson"
	"lsurvey/internal/paths"
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
	if len(args) >= 4 && args[0] == "export" && args[1] == "dxf" {
		p, err := project.Load(args[2])
		if err != nil {
			return err
		}
		f, err := os.Create(paths.DXF(args[3]))
		if err != nil {
			return err
		}
		defer f.Close()
		return dxf.Write(f, p)
	}
	if len(args) >= 4 && args[0] == "export" && args[1] == "csv" {
		p, err := project.Load(args[2])
		if err != nil {
			return err
		}
		return csvpoints.ExportFile(paths.CSV(args[3]), p)
	}
	if len(args) >= 4 && args[0] == "export" && args[1] == "geojson" {
		p, err := project.Load(args[2])
		if err != nil {
			return err
		}
		return geojson.ExportFile(paths.GeoJSON(args[3]), p)
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
