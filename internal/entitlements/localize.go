package entitlements

import (
	"golang.org/x/text/language"
)

// Localizer resolves a translated value from a locale→string map based on a
// request's language preferences, falling back to the configured default
// language.
type Localizer struct {
	prefs []string
	def   string
}

// NewLocalizer builds a Localizer from an Accept-Language header value and the
// default language. The header is parsed into an ordered preference list; for
// each entry both the full tag (e.g. "es-ES") and its base language (e.g. "es")
// are kept, so config keyed by either form matches. A missing or malformed
// header yields no preferences, so resolution falls back to defaultLanguage.
func NewLocalizer(acceptLanguage, defaultLanguage string) Localizer {
	var prefs []string
	if acceptLanguage != "" {
		if tags, _, err := language.ParseAcceptLanguage(acceptLanguage); err == nil {
			for _, t := range tags {
				prefs = append(prefs, t.String())
				if base, conf := t.Base(); conf != language.No {
					prefs = append(prefs, base.String())
				}
			}
		}
	}
	return Localizer{prefs: prefs, def: defaultLanguage}
}

// Pick returns the best translation from m: the first preferred locale present,
// else the default-language value, else "". A nil/empty map yields "".
func (l Localizer) Pick(m map[string]string) string {
	for _, p := range l.prefs {
		if v, ok := m[p]; ok && v != "" {
			return v
		}
	}
	if v, ok := m[l.def]; ok {
		return v
	}
	return ""
}

// DefaultLanguage returns the configured fallback language.
func (s *Service) DefaultLanguage() string {
	return s.ent.DefaultLanguage
}
