package landxml

import (
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"strconv"

	"lsurvey/internal/geom"
	"lsurvey/internal/project"
)

type document struct {
	XMLName      xml.Name     `xml:"LandXML"`
	XMLNS        string       `xml:"xmlns,attr"`
	Version      string       `xml:"version,attr"`
	Units        units        `xml:"Units"`
	CgPoints     cgPoints     `xml:"CgPoints"`
	PlanFeatures planFeatures `xml:"PlanFeatures"`
}

type units struct {
	Metric metric `xml:"Metric"`
}

type metric struct {
	LinearUnit string `xml:"linearUnit,attr"`
	AreaUnit   string `xml:"areaUnit,attr"`
}

type cgPoints struct {
	Points []cgPoint `xml:"CgPoint"`
}

type cgPoint struct {
	Name     string    `xml:"name,attr"`
	Code     string    `xml:"code,attr,omitempty"`
	Value    string    `xml:",chardata"`
	Metadata *metadata `xml:"Feature,omitempty"`
}

type planFeatures struct {
	Features []planFeature `xml:"PlanFeature"`
}

type planFeature struct {
	Name      string    `xml:"name,attr"`
	Code      string    `xml:"code,attr,omitempty"`
	CoordGeom coordGeom `xml:"CoordGeom"`
	Metadata  *metadata `xml:"Feature,omitempty"`
}

type coordGeom struct {
	Lines []line `xml:"Line"`
}

type line struct {
	Start endpoint `xml:"Start"`
	End   endpoint `xml:"End"`
}

type endpoint struct {
	Ref   string `xml:"pntRef,attr"`
	Value string `xml:",chardata"`
}

type metadata struct {
	Code       string     `xml:"code,attr"`
	Properties []property `xml:"Property"`
}

type property struct {
	Label string `xml:"label,attr"`
	Value string `xml:"value,attr"`
}

func ExportFile(path string, p *project.Project) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	err = Write(f, p)
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	return err
}

func Write(w io.Writer, p *project.Project) error {
	if p.Units["distance"] != "m" {
		return fmt.Errorf("LandXML export currently requires distance=m")
	}
	doc := document{
		XMLNS: "http://www.landxml.org/schema/LandXML-1.2", Version: "1.2",
		Units: units{Metric: metric{LinearUnit: "meter", AreaUnit: "squareMeter"}},
	}
	for _, pt := range p.SortedPoints() {
		point := cgPoint{Name: pt.ID, Code: pt.Code, Value: ordinate(pt)}
		if group, ok := p.Groups[pt.GroupID]; ok {
			point.Metadata = groupMetadata(group)
		}
		doc.CgPoints.Points = append(doc.CgPoints.Points, point)
	}
	for _, feature := range p.SortedFeatures() {
		pf := planFeature{Name: feature.ID, Code: feature.Code}
		for _, segment := range p.FeatureSegments(feature) {
			pf.CoordGeom.Lines = append(pf.CoordGeom.Lines, line{
				Start: endpoint{Ref: segment.From, Value: ordinate(p.Points[segment.From])},
				End:   endpoint{Ref: segment.To, Value: ordinate(p.Points[segment.To])},
			})
		}
		if group, ok := p.Groups[feature.GroupID]; ok {
			pf.Metadata = groupMetadata(group)
		}
		doc.PlanFeatures.Features = append(doc.PlanFeatures.Features, pf)
	}
	if _, err := io.WriteString(w, xml.Header); err != nil {
		return err
	}
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	return enc.Encode(doc)
}

func groupMetadata(group project.Group) *metadata {
	return &metadata{Code: "lsurvey", Properties: []property{
		{Label: "group", Value: group.ID},
		{Label: "layer", Value: group.Layer},
		{Label: "color", Value: strconv.Itoa(group.Color)},
	}}
}

func ordinate(pt geom.Point) string {
	z := 0.0
	if pt.Elevation != nil {
		z = *pt.Elevation
	}
	return fmt.Sprintf("%g %g %g", pt.Northing, pt.Easting, z)
}
