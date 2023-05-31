package config

// SkShadersConfig contains Web-UI values delivered by the frontend.
type SkShadersConfig struct {
	FiddleOrigin   string `json:"fiddle_origin"`   // The fiddle origin (e.g. "https://fiddle.skia.org").
	JsFiddleOrigin string `json:"jsfiddle_origin"` // The jsfiddle origin (e.g. "https://jsfiddle.skia.org").
}
