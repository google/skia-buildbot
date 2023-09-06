package config

// SkSkottieConfig contains Web-UI values delivered by the frontend.
type SkSkottieConfig struct {
	PublicSiteDomain   string `json:"public_site_domain"`   // The public Skottie site domain (e.g. "skottie.skia.org").
	InternalSiteDomain string `json:"internal_site_domain"` // The internal Skottie site domain (e.g. "skottie-internal.corp.goog").
	TenorSiteDomain    string `json:"tenor_site_domain"`    // The tenor Skottie site domain (e.g. "skottie-tenor.corp.goog").
}
