package paths

import "strings"

const ProjectExt = ".srv"
const DXFExt = ".dxf"
const CSVExt = ".csv"
const GeoJSONExt = ".geojson"

func Project(path string) string {
	return ensureExt(path, ProjectExt)
}

func DXF(path string) string {
	return ensureExt(path, DXFExt)
}

func CSV(path string) string {
	return ensureExt(path, CSVExt)
}

func GeoJSON(path string) string {
	return ensureExt(path, GeoJSONExt)
}

func ensureExt(path, ext string) string {
	if path == "" {
		return path
	}
	if strings.EqualFold(path[len(path)-lenOrLess(len(ext), len(path)):], ext) {
		return path
	}
	return path + ext
}

func lenOrLess(a, b int) int {
	if a < b {
		return a
	}
	return b
}
