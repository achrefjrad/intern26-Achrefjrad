package scrapers

import (
	"net/url"
	"strings"
)

func normalizeURL(raw string) string {
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	u.Path = strings.TrimSuffix(u.Path, "/")
	u.RawQuery = ""
	u.Fragment = ""
	u.Scheme = "https"
	return u.String()
}
