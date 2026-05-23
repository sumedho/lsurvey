package project

const DefaultDisplayPrecision = 3

type DisplaySettings struct {
	Precision *int `json:"precision,omitempty"`
}

func (p *Project) DisplayPrecision() int {
	p.ensure()
	return clampPrecision(*p.Display.Precision)
}

func (p *Project) SetDisplayPrecision(precision int) {
	p.ensure()
	p.Display.Precision = intPtr(clampPrecision(precision))
}

func clampPrecision(precision int) int {
	if precision < 0 {
		return 0
	}
	if precision > 6 {
		return 6
	}
	return precision
}

func intPtr(value int) *int {
	return &value
}
