package project

import (
	"encoding/json"
	"fmt"
)

// decodeProject is a read-only bridge for legacy JSON documents. New stores do not use this format.
func decodeProject(data []byte) (*Project, error) {
	var p Project
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, err
	}
	if p.SchemaVersion < 1 || p.SchemaVersion > CurrentSchemaVersion {
		return nil, fmt.Errorf("unsupported schema_version %d", p.SchemaVersion)
	}
	// Reject invalid precision before legacy defaults can normalise it.
	if p.Display.Precision != nil && (*p.Display.Precision < 0 || *p.Display.Precision > 6) {
		return nil, fmt.Errorf("invalid display precision")
	}
	p.ensure()
	if err := p.Validate(); err != nil {
		return nil, err
	}
	return &p, nil
}
