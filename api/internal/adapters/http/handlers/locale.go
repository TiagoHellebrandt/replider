package handlers

import (
	"strings"

	"github.com/gin-gonic/gin"
)

// supportedLocales is the closed set of locales the server actively
// renders user-facing copy in. Anything else falls back to the default.
var supportedLocales = map[string]struct{}{
	"pt": {},
	"en": {},
}

const defaultLocale = "pt"

// AcceptLocale reads the `Accept-Language` header and returns one of
// the supported locale codes (`pt` / `en`). Defaults to `pt` because
// the product's primary audience and the agent personas themselves
// are Brazilian.
//
// We only need a coarse pt/en split for now, so the parser is
// deliberately simple: take the first language tag, normalise to
// lowercase, strip the region (e.g. `pt-BR` → `pt`), and look it up.
// Quality factors (`q=0.8`) are ignored.
func AcceptLocale(c *gin.Context) string {
	raw := c.GetHeader("Accept-Language")
	if raw == "" {
		return defaultLocale
	}
	// Take the first comma-separated tag.
	first := strings.SplitN(raw, ",", 2)[0]
	// Drop quality factor and surrounding whitespace.
	first = strings.TrimSpace(strings.SplitN(first, ";", 2)[0])
	// Strip region: `pt-BR` / `pt_BR` → `pt`.
	first = strings.ToLower(first)
	if i := strings.IndexAny(first, "-_"); i > 0 {
		first = first[:i]
	}
	if _, ok := supportedLocales[first]; ok {
		return first
	}
	return defaultLocale
}
