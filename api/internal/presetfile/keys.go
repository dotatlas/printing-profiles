package presetfile

import (
	"strconv"
	"strings"
)

// Setting is one key setting of a preset, ready for the table on the preset page.
type Setting struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Value string `json:"value"`
	Unit  string `json:"unit,omitempty"`
}

// keyTable lists the key settings in the order that the preset page shows them. keys holds the setting names in
// the PrusaSlicer format, then the OrcaSlicer format; the first name with a value wins, so a filament's own
// retraction comes before the printer's.
var keyTable = []struct {
	id, label, unit string
	keys            []string
}{
	{"layer_height", "Layer height", "mm", []string{"layer_height"}},
	{"first_layer_height", "First layer height", "mm", []string{"first_layer_height", "initial_layer_print_height"}},
	{"walls", "Walls", "", []string{"perimeters", "wall_loops"}},
	{"top_layers", "Top layers", "", []string{"top_solid_layers", "top_shell_layers"}},
	{"bottom_layers", "Bottom layers", "", []string{"bottom_solid_layers", "bottom_shell_layers"}},
	{"infill_density", "Infill density", "%", []string{"fill_density", "sparse_infill_density"}},
	{"infill_pattern", "Infill pattern", "", []string{"fill_pattern", "sparse_infill_pattern"}},
	{"outer_wall_speed", "Outer wall speed", "mm/s", []string{"external_perimeter_speed", "outer_wall_speed"}},
	{"infill_speed", "Infill speed", "mm/s", []string{"infill_speed", "sparse_infill_speed"}},
	{"nozzle_temperature_first_layer", "Nozzle temperature, first layer", "°C", []string{"first_layer_temperature", "nozzle_temperature_initial_layer"}},
	{"nozzle_temperature", "Nozzle temperature", "°C", []string{"temperature", "nozzle_temperature"}},
	{"bed_temperature_first_layer", "Bed temperature, first layer", "°C", []string{"first_layer_bed_temperature", "hot_plate_temp_initial_layer"}},
	{"bed_temperature", "Bed temperature", "°C", []string{"bed_temperature", "hot_plate_temp"}},
	{"fan_min", "Fan speed, minimum", "%", []string{"min_fan_speed", "fan_min_speed"}},
	{"fan_max", "Fan speed, maximum", "%", []string{"max_fan_speed", "fan_max_speed"}},
	{"filament_type", "Filament type", "", []string{"filament_type"}},
	{"retraction_length", "Retraction length", "mm", []string{"filament_retract_length", "filament_retraction_length", "retract_length", "retraction_length"}},
	{"retraction_speed", "Retraction speed", "mm/s", []string{"filament_retract_speed", "filament_retraction_speed", "retract_speed", "retraction_speed"}},
	{"nozzle_diameter", "Nozzle diameter", "mm", []string{"nozzle_diameter"}},
}

// keySettings picks the key settings from a file's settings. Parse calls it. A list value gives the first
// extruder's value, "nil" means "use the printer's value", and a unit goes only on a plain number ("50%" stays).
func keySettings(settings map[string]string) []Setting {
	var out []Setting
	for _, row := range keyTable {
		for _, key := range row.keys {
			value, _, _ := strings.Cut(settings[key], ",")
			value = strings.Trim(strings.TrimSpace(value), `"`)
			if value == "" || value == "nil" {
				continue
			}
			s := Setting{ID: row.id, Label: row.label, Value: value}
			if _, err := strconv.ParseFloat(value, 64); err == nil {
				s.Unit = row.unit
			}
			out = append(out, s)
			break
		}
	}
	return out
}
