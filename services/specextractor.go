package services

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"project/models"
	"project/types"
)

var (
	specKeyNorm     = regexp.MustCompile(`[^a-z0-9]+`)
	reRAM           *regexp.Regexp
	rePhoneStockage *regexp.Regexp
	rePCStockageGB  *regexp.Regexp
	rePCStockageTB  *regexp.Regexp
	reEcran         *regexp.Regexp
	reProcesseur    *regexp.Regexp
	reVolume        *regexp.Regexp
	reSPF           *regexp.Regexp
	rePuissance     *regexp.Regexp
	reTension       *regexp.Regexp
	reBatterie      *regexp.Regexp
	rePression      *regexp.Regexp
	reCapacite      *regexp.Regexp
	reLargeurCoupe  *regexp.Regexp
)

var brandKeywords = map[string]string{
	"iphone":    "Apple",
	"macbook":   "Apple",
	"imac":      "Apple",
	"ipad":      "Apple",
	"apple":     "Apple",
	"galaxy":    "Samsung",
	"samsung":   "Samsung",
	"redmi":     "Xiaomi",
	"poco":      "Xiaomi",
	"xiaomi":    "Xiaomi",
	"huawei":    "Huawei",
	"oppo":      "Oppo",
	"oneplus":   "OnePlus",
	"thinkpad":  "Lenovo",
	"ideapad":   "Lenovo",
	"lenovo":    "Lenovo",
	"pavilion":  "HP",
	"elitebook": "HP",
	"probook":   "HP",
	"hp ":       "HP",
	"xps":       "Dell",
	"inspiron":  "Dell",
	"latitude":  "Dell",
	"dell":      "Dell",
	"rog":       "Asus",
	"zenbook":   "Asus",
	"vivobook":  "Asus",
	"asus":      "Asus",
	"aspire":    "Acer",
	"nitro":     "Acer",
	"acer":      "Acer",
	"nokia":     "Nokia",
	"pixel":     "Google",
	"google":    "Google",
	"sony":      "Sony",
	"lg ":       "LG",
	"motorola":  "Motorola",
	"surface":   "Microsoft",
	"microsoft": "Microsoft",
	// Santé & Beauté brands
	"l'oreal":        "L'Oréal",
	"loreal":         "L'Oréal",
	"nivea":          "Nivea",
	"garnier":        "Garnier",
	"vichy":          "Vichy",
	"la roche-posay": "La Roche-Posay",
	"laroche posay":  "La Roche-Posay",
	"bioderma":       "Bioderma",
	"avene":          "Avène",
	"cerave":         "CeraVe",
	"uriage":         "Uriage",
	"sisley":         "Sisley",
	"clarins":        "Clarins",
	"lancome":        "Lancôme",
	"chanel":         "Chanel",
	"dior":           "Dior",
	"yves rocher":    "Yves Rocher",
	"yvesrocher":     "Yves Rocher",
	"sanoflore":      "Sanoflore",
	"klorane":        "Klorane",
	"rené furterer":  "René Furterer",
	"furterer":       "René Furterer",
	"ducray":         "Ducray",
	"mustela":        "Mustela",
	"mixa":           "Mixa",
	"eucerin":        "Eucerin",
	"cetaphil":       "Cetaphil",
	"topicrem":       "Topicrem",
	"nymphea":        "Nymphea",
	"so'bio étic":    "So'Bio Étic",
	"so bio etic":    "So'Bio Étic",
	"celimax":        "CELIMAX",
	"essence":        "ESSENCE",
	"catrice":        "Catrice",
	"maybelline":     "Maybelline",
	"nyx":            "NYX",
	"revlon":         "Revlon",
	"bourjois":       "Bourjois",
	"rimmel":         "Rimmel",
	"max factor":     "Max Factor",
	"mac":            "MAC",
	"benefit":        "Benefit",
	"the ordinary":   "The Ordinary",
	"ordinary":       "The Ordinary",
	// Jardinage & Bricolage brands
	"bosch":          "BOSCH",
	"ingco":          "INGCO",
	"valex":          "VALEX",
	"harden":         "HARDEN",
	"total tools":    "TOTAL",
	"makita":         "MAKITA",
	"dewalt":         "DEWALT",
	"altuna":         "ALTUNA",
	"virutex":        "VIRUTEX",
	"black & decker": "Black & Decker",
	"black and decker": "Black & Decker",
	"ryobi":          "RYOBI",
	"stanley":        "Stanley",
	"facom":          "Facom",
	"pro'skit":       "Pro'sKit",
	"proskit":        "Pro'sKit",
}

func init() {
	reRAM = regexp.MustCompile(`(?i)(?:mémoire(?:\s+vive)?|ram)\s+(\d+(?:[.,]\d+)?)\s*(?:Go|GB)`)
	rePhoneStockage = regexp.MustCompile(`(?i)(?:stockage|capacité(?:\s+de\s+stockage)?|mémoire\s+interne)\s+(\d+(?:[.,]\d+)?)\s*(?:Go|GB)`)
	rePCStockageGB = regexp.MustCompile(`(?i)(?:disque(?:\s+dur)?|ssd|stockage)\s+(\d+(?:[.,]\d+)?)\s*(?:Go|GB)`)
	rePCStockageTB = regexp.MustCompile(`(?i)(?:disque(?:\s+dur)?|hdd|stockage)\s+(\d+(?:[.,]\d+)?)\s*(?:To|TB)`)
	reEcran = regexp.MustCompile(`(?i)(?:écran|ecran)\s+(\d+(?:[.,]\d+)?)\s*(?:pouces?|″|")`)
	reProcesseur = regexp.MustCompile(`(?i)(intel\s+core\s+i[3579]|amd\s+ryzen\s+[3579]|apple\s+m[1234]|celeron|pentium|snapdragon)`)
	reVolume = regexp.MustCompile(`(?i)(\d+(?:[.,]\d+)?)\s*(ml|l|g|oz|kg)`)
	reSPF = regexp.MustCompile(`(?i)(?:spf|ip|fps)\s*(\d+\+?)`)
	rePuissance = regexp.MustCompile(`(?i)(\d+(?:\.\d+)?)\s*(W|kW|Ch|HP)\b`)
	reTension = regexp.MustCompile(`(?i)(\d+)\s*V\b`)
	reBatterie = regexp.MustCompile(`(?i)(\d+(?:\.\d+)?)\s*Ah\b`)
	rePression = regexp.MustCompile(`(?i)(\d+(?:\.\d+)?)\s*(?:bar|bars)\b`)
	reCapacite = regexp.MustCompile(`(?i)(\d+(?:\.\d+)?)\s*(L|Litres|Kg)\b`)
	reLargeurCoupe = regexp.MustCompile(`(?i)(\d+)\s*(cm|mm)\b`)
}

func NormalizeSpecKey(raw string) string {
	s := strings.ToLower(strings.TrimSpace(raw))
	s = strings.ReplaceAll(s, "é", "e")
	s = strings.ReplaceAll(s, "è", "e")
	s = strings.ReplaceAll(s, "ê", "e")
	s = strings.ReplaceAll(s, "ë", "e")
	s = strings.ReplaceAll(s, "à", "a")
	s = strings.ReplaceAll(s, "â", "a")
	s = strings.ReplaceAll(s, "ä", "a")
	s = strings.ReplaceAll(s, "ù", "u")
	s = strings.ReplaceAll(s, "û", "u")
	s = strings.ReplaceAll(s, "ü", "u")
	s = strings.ReplaceAll(s, "ô", "o")
	s = strings.ReplaceAll(s, "ö", "o")
	s = strings.ReplaceAll(s, "î", "i")
	s = strings.ReplaceAll(s, "ï", "i")
	s = strings.ReplaceAll(s, "ç", "c")
	s = specKeyNorm.ReplaceAllString(s, "_")
	return strings.Trim(s, "_")
}

func extractSpecNumber(description string, re *regexp.Regexp) (float64, bool) {
	m := re.FindStringSubmatch(description)
	if m == nil {
		return 0, false
	}
	raw := strings.Replace(m[1], ",", ".", 1)
	f, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, false
	}
	return f, true
}

func formatSpec(f float64, unit string) string {
	if f == float64(int(f)) {
		return fmt.Sprintf("%d %s", int(f), unit)
	}
	return fmt.Sprintf("%.1f %s", f, unit)
}

func ExtractSpecs(category types.Category, subcategory types.Subcategory, description string) map[string]string {
	specs := make(map[string]string)
	if strings.TrimSpace(description) == "" {
		return specs
	}

	switch subcategory {
	case types.SubcatSmartphones:
		if f, ok := extractSpecNumber(description, reRAM); ok {
			specs["RAM"] = formatSpec(f, "GB")
		}
		if f, ok := extractSpecNumber(description, rePhoneStockage); ok {
			specs["Stockage"] = formatSpec(f, "GB")
		}

	case types.SubcatOrdinateurs:
		if f, ok := extractSpecNumber(description, reRAM); ok {
			specs["RAM"] = formatSpec(f, "GB")
		}
		if f, ok := extractSpecNumber(description, rePCStockageTB); ok {
			specs["Stockage"] = formatSpec(f, "TB")
		} else if f, ok := extractSpecNumber(description, rePCStockageGB); ok {
			specs["Stockage"] = formatSpec(f, "GB")
		}
		if f, ok := extractSpecNumber(description, reEcran); ok {
			specs["Écran"] = formatSpec(f, "pouces")
		}
		if matches := reProcesseur.FindStringSubmatch(description); len(matches) > 1 && matches[1] != "" {
			raw := matches[1]
			parts := strings.Fields(raw)
			for i, p := range parts {
				lower := strings.ToLower(p)
				switch lower {
				case "i3", "i5", "i7", "i9":
					parts[i] = lower
				case "amd":
					parts[i] = "AMD"
				default:
					parts[i] = strings.ToUpper(string(p[0])) + lower[1:]
				}
			}
			specs["Processeur"] = strings.Join(parts, " ")
		}

	case types.SubcatSoinsVisage, types.SubcatCheveux, types.SubcatHygieneToilette, types.SubcatSanteBeauteAutres, types.SubcatBebeMaman, types.SubcatParfumsCoffrets, types.SubcatMaquillage:
		if m := reVolume.FindStringSubmatch(description); m != nil {
			specs["contenance"] = fmt.Sprintf("%s %s", m[1], strings.ToLower(m[2]))
		}
		if m := reSPF.FindStringSubmatch(description); m != nil {
			specs["spf"] = m[1]
		}

	case types.SubcatOutillageJardinMotorise, types.SubcatOutillageElectroportatif, types.SubcatArrosagePompes, types.SubcatOutillageMain:
		if m := rePuissance.FindStringSubmatch(description); m != nil {
			specs["puissance"] = strings.TrimSpace(m[1] + " " + strings.ToUpper(m[2]))
		}
		if m := reTension.FindStringSubmatch(description); m != nil {
			specs["tension"] = m[1] + "V"
		}
		if m := reBatterie.FindStringSubmatch(description); m != nil {
			specs["batterie"] = strings.TrimSpace(m[1] + " Ah")
		}
		if m := rePression.FindStringSubmatch(description); m != nil {
			specs["pression"] = strings.TrimSpace(m[1] + " bar")
		}
		if m := reCapacite.FindStringSubmatch(description); m != nil {
			specs["capacite"] = strings.TrimSpace(m[1] + " " + m[2])
		}
		if m := reLargeurCoupe.FindStringSubmatch(description); m != nil {
			specs["largeur_coupe"] = strings.TrimSpace(m[1] + " " + strings.ToLower(m[2]))
		}
	}

	return specs
}

func extractBrand(name string) string {
	lower := strings.ToLower(name)
	var bestBrand string
	var bestLen int
	for kw, brand := range brandKeywords {
		if strings.Contains(lower, kw) && len(kw) > bestLen {
			bestBrand = brand
			bestLen = len(kw)
		}
	}
	if bestBrand != "" {
		return bestBrand
	}

	// Fallback: extract first fully-uppercase word(s) from the start of the name
	// e.g. "CELIMAX Disques..." → "CELIMAX", "LA ROCHE-POSAY Crème..." → "LA ROCHE-POSAY"
	fields := strings.Fields(name)
	if len(fields) > 0 {
		var brandParts []string
		for _, f := range fields {
			cleaned := strings.Trim(f, ".,;:!?()[]{}«»\"'")
			if cleaned == "" {
				continue
			}
			// Stop at the first word that isn't fully uppercase (len > 1 to skip single-letter words like "À")
			isUpper := true
			for _, r := range cleaned {
				if unicode.IsLetter(r) && !unicode.IsUpper(r) {
					isUpper = false
					break
				}
			}
			if !isUpper || len(cleaned) <= 1 {
				break
			}
			brandParts = append(brandParts, cleaned)
		}
		if len(brandParts) > 0 {
			return strings.Join(brandParts, " ")
		}
	}

	return ""
}

var jardimageCategory = "Jardinage & Bricolage"

func NormalizeProductSpecs(specs map[string]string, category string) map[string]string {
	if specs == nil {
		return nil
	}
	isJardinage := category == jardimageCategory
	result := make(map[string]string, len(specs))
	for k, v := range specs {
		normalizedKey := k
		switch k {
		case "memoire":
			normalizedKey = "ram"
		case "disque_dur":
			normalizedKey = "stockage"
		case "systeme_d_exploitation", "systeme":
			normalizedKey = "os"
		case "taille_ecran":
			normalizedKey = "taille_de_l_ecran"
		case "volume":
			normalizedKey = "contenance"
		case "type_de_peaux":
			normalizedKey = "type_de_peau"
		case "besoin_cheveux":
			normalizedKey = "besoin"
		}
		if isJardinage && normalizedKey != "marque" {
			continue
		}
		result[normalizedKey] = v
	}
	return result
}

func EnrichProduct(p *models.Product) {
	if p.Subcategory == "" {
		cat, sub := AssignProductCategory(p.Name)
		p.Subcategory = string(sub)
		if p.Category == "" {
			p.Category = string(cat)
		}
	}

	cat := types.Category(p.Category)
	sub := types.Subcategory(p.Subcategory)

	text := p.Name
	if p.Description != "" {
		text += " " + p.Description
	}

	normalized := make(map[string]string)

	if p.Specs != nil && len(p.Specs) > 0 {
		for k, v := range p.Specs {
			normalized[NormalizeSpecKey(k)] = v
		}
	} else {
		specs := ExtractSpecs(cat, sub, text)
		for k, v := range specs {
			normalized[NormalizeSpecKey(k)] = v
		}
	}

	if brand := extractBrand(text); brand != "" {
		normalized["marque"] = brand
	}

	p.Specs = NormalizeProductSpecs(normalized, p.Category)
}
