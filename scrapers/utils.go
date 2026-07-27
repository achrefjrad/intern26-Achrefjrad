package scrapers

import (
	"fmt"
	"hash/fnv"
	"regexp"
	"strconv"
	"strings"

	"project/services"
	"project/types"
)

// ParseTNDPrice parses a Tunisian price string (DT/TND) into (float64, originalString).
// Handles non-breaking spaces, tabs, comma-as-decimal, "DT"/"TND" suffixes,
// and detects duplicated numeric patterns (e.g. "1 2991 299").
func ParseTNDPrice(s string) (float64, string) {
	raw := strings.TrimSpace(s)
	if raw == "" {
		return 0, ""
	}

	raw = strings.ReplaceAll(raw, "\u00a0", " ")
	raw = strings.ReplaceAll(raw, "\t", " ")
	reSpace := regexp.MustCompile(`\s+`)
	raw = strings.TrimSpace(reSpace.ReplaceAllString(raw, " "))

	cleaned := raw

	cleaned = strings.ReplaceAll(cleaned, "DT", "")
	cleaned = strings.ReplaceAll(cleaned, "TND", "")
	cleaned = strings.ReplaceAll(cleaned, "&nbsp;", "")
	cleaned = strings.TrimSpace(cleaned)

	cleaned = strings.ReplaceAll(cleaned, " ", "")

	// Detect duplicated numeric patterns (e.g. "1 2991 299" → "1 299")
	if len(cleaned) >= 2 && len(cleaned)%2 == 0 {
		half := len(cleaned) / 2
		if cleaned[:half] == cleaned[half:] {
			cleaned = cleaned[:half]
		}
	}

	// Last comma is the decimal separator
	if idx := strings.LastIndex(cleaned, ","); idx != -1 {
		cleaned = cleaned[:idx] + "." + cleaned[idx+1:]
	}

	reDigits := regexp.MustCompile(`[^\d.]`)
	cleaned = reDigits.ReplaceAllString(cleaned, "")

	if cleaned == "" {
		return 0, raw
	}

	val, err := strconv.ParseFloat(cleaned, 64)
	if err != nil {
		return 0, raw
	}

	return val, raw
}

// AbsImageURL converts a possibly-relative image URL to absolute.
// Handles "//" protocol-relative, absolute prefixes, and relative paths.
func AbsImageURL(baseURL, rawURL string) string {
	url := strings.TrimSpace(rawURL)
	if url == "" {
		return ""
	}
	if strings.HasPrefix(url, "https://") || strings.HasPrefix(url, "http://") {
		return url
	}
	if strings.HasPrefix(url, "//") {
		return "https:" + url
	}
	return baseURL + url
}

// GenerateScraperID creates a deterministic ID for a scraper product
// using FNV-32a hashing of the URL with the given merchant prefix.
func GenerateScraperID(prefix, rawURL string) string {
	normalized := strings.TrimSpace(strings.ToLower(rawURL))
	h := fnv.New32a()
	h.Write([]byte(normalized))
	return fmt.Sprintf("%s-%d", prefix, int(h.Sum32()))
}

// MapJardinageBricolageCategory maps breadcrumb text or URL path to a
// Jardinage & Bricolage subcategory using keyword matching.
func MapJardinageBricolageCategory(breadcrumbs []string, urlPath string) string {
	check := func(keywords ...string) bool {
		for _, crumb := range breadcrumbs {
			low := strings.ToLower(crumb)
			for _, kw := range keywords {
				if strings.Contains(low, kw) {
					return true
				}
			}
		}
		lowURL := strings.ToLower(urlPath)
		for _, kw := range keywords {
			if strings.Contains(lowURL, kw) {
				return true
			}
		}
		return false
	}

	switch {
	case check("tondeuse", "tronconneuse", "tronçonneuse", "debroussailleuse", "débroussailleuse", "taille-haie", "taille haie", "souffleur", "coupe-bordure", "motoculteur", "motobêche", "motobebe", "tariere", "tarière", "outillage jardin", "jardinage"):
		return string(types.SubcatOutillageJardinMotorise)
	case check("perceuse", "meuleuse", "scie", "ponceuse", "marteau perforateur", "visseuse", "perforateur", "burineur", "rainureuse", "defonceuse", "défonceuse", "electroportatif", "outillage pneumatique", "compresseur", "soudure"):
		return string(types.SubcatOutillageElectroportatif)
	case check("pulverisateur", "pulvérisateur", "pompe", "tuyau", "enrouleur", "arrosage", "nettoyeur haute pression", "nettoyeur hp", "jet haute pression", "jet hp"):
		return string(types.SubcatArrosagePompes)
	case check("secateur", "sécateur", "cisaille", "hache", "pelle", "tournevis", "pince", "marteau", "grattoir", "cutter", "niveau", "metre ruban", "mètre ruban", "serre-joint", "egoine", "e going", "scie egoing", "scie e going", "burin", "coupe-tube", "outillage main", "outillage-main", "outillage à main", "outillage a main"):
		return string(types.SubcatOutillageMain)
	}

	if cat, sub := services.AssignProductCategory(urlPath); sub != types.SubcatUncategorized && cat != types.CategoryUncategorized {
		return string(sub)
	}
	return string(types.SubcatOutillageMain)
}
