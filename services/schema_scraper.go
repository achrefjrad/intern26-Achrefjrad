package services

import (
	"log"
	"regexp"
	"strings"
	"time"
	"unicode"

	"project/types"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
)

// -------- Types --------

type MerchantURL struct {
	Merchant string `json:"merchant"`
	URL      string `json:"url"`
}

type SubcategorySource struct {
	Category    string            `json:"category"`
	Subcategory types.Subcategory `json:"subcategory"`
	Sources     []MerchantURL     `json:"sources"`
}

type CategorySchema struct {
	Subcategory string              `json:"subcategory"`
	Filters     map[string][]string `json:"filters"`
}

// -------- Category-Merchant Mapping --------

var subcategorySources = []SubcategorySource{
	// ======= Informatique =======
	{Category: "Informatique", Subcategory: types.SubcatOrdinateurs, Sources: []MerchantURL{
		{Merchant: "tunisianet", URL: "https://www.tunisianet.com.tn/301-pc-portable-tunisie"},
		{Merchant: "mytek", URL: "https://www.mytek.tn/informatique/ordinateurs-portables/pc-portable.html"},
		{Merchant: "sbs", URL: "https://www.sbsinformatique.com/pc-portable-tunisie"},
		{Merchant: "wiki", URL: "https://wiki.tn/pc-portables"},
	}},
	{Category: "Informatique", Subcategory: types.SubcatComposants, Sources: []MerchantURL{
		{Merchant: "tunisianet", URL: "https://www.tunisianet.com.tn/406-composant-informatique"},
		{Merchant: "mytek", URL: "https://www.mytek.tn/informatique/composants-informatique.html"},
		{Merchant: "sbs", URL: "https://www.sbsinformatique.com/composants-tunisie"},
		{Merchant: "wiki", URL: "https://wiki.tn/composants-pc-de-bureau"},
	}},
	{Category: "Informatique", Subcategory: types.SubcatReseauxConnectivite, Sources: []MerchantURL{
		{Merchant: "tunisianet", URL: "https://www.tunisianet.com.tn/438-reseau"},
		{Merchant: "mytek", URL: "https://www.mytek.tn/reseaux-securite/reseaux.html"},
		{Merchant: "sbs", URL: "https://www.sbsinformatique.com/reseau-securite-tunisie"},
		{Merchant: "wiki", URL: "https://wiki.tn/reseau-securite"},
	}},
	{Category: "Informatique", Subcategory: types.SubcatPeripheriques, Sources: []MerchantURL{
		{Merchant: "tunisianet", URL: "https://www.tunisianet.com.tn/700-accessoires-et-peripheriques"},
		{Merchant: "mytek", URL: "https://www.mytek.tn/informatique/peripheriques-accessoires.html"},
		{Merchant: "sbs", URL: "https://www.sbsinformatique.com/peripheriques-tunisie"},
		{Merchant: "wiki", URL: "https://wiki.tn/accessoires-ordinateur"},
	}},
	{Category: "Informatique", Subcategory: types.SubcatStockages, Sources: []MerchantURL{
		{Merchant: "tunisianet", URL: "https://www.tunisianet.com.tn/669-disques-internes"},
		{Merchant: "mytek", URL: "https://www.mytek.tn/informatique/stockage.html"},
		{Merchant: "sbs", URL: "https://www.sbsinformatique.com/stockage-tunisie"},
		{Merchant: "wiki", URL: "https://wiki.tn/stockage"},
	}},

	// ======= Téléphonie =======
	{Category: "Téléphonie", Subcategory: types.SubcatSmartphones, Sources: []MerchantURL{
		{Merchant: "tunisianet", URL: "https://www.tunisianet.com.tn/596-smartphone-tunisie"},
		{Merchant: "mytek", URL: "https://www.mytek.tn/telephonie-tunisie/smartphone-mobile-tunisie.html"},
		{Merchant: "sbs", URL: "https://www.sbsinformatique.com/telephonie-tablette-tunisie"},
		{Merchant: "wiki", URL: "https://wiki.tn/smartphones"},
	}},
	{Category: "Téléphonie", Subcategory: types.SubcatTelephonesPortables, Sources: []MerchantURL{
		{Merchant: "tunisianet", URL: "https://www.tunisianet.com.tn/377-telephone-portable-tunisie"},
		{Merchant: "mytek", URL: "https://www.mytek.tn/telephonie-tunisie/smartphone-mobile-tunisie.html"},
		{Merchant: "sbs", URL: "https://www.sbsinformatique.com/telephone-portable-tunisie"},
		{Merchant: "wiki", URL: "https://wiki.tn/telephones-portables"},
	}},
	{Category: "Téléphonie", Subcategory: types.SubcatAccessoires, Sources: []MerchantURL{
		{Merchant: "tunisianet", URL: "https://www.tunisianet.com.tn/378-accessoire-telephonie-mobile-tunisie"},
		{Merchant: "mytek", URL: "https://www.mytek.tn/telephonie-tunisie/accessoires-telephonie.html"},
		{Merchant: "sbs", URL: "https://www.sbsinformatique.com/accessoires-telephonie-tunisie"},
		{Merchant: "wiki", URL: "https://wiki.tn/accessoires-telephone"},
	}},
	{Category: "Téléphonie", Subcategory: types.SubcatTelephonesFixes, Sources: []MerchantURL{
		{Merchant: "tunisianet", URL: "https://www.tunisianet.com.tn/462-telephone-fixe"},
		{Merchant: "mytek", URL: "https://www.mytek.tn/telephonie-tunisie/telephone-fixe.html"},
		{Merchant: "sbs", URL: ""}, // SBS does not maintain a dedicated home phone category
		{Merchant: "wiki", URL: "https://wiki.tn/telephone-fixe-fax"},
	}},
	{Category: "Téléphonie", Subcategory: types.SubcatSmartwatch, Sources: []MerchantURL{
		{Merchant: "tunisianet", URL: "https://www.tunisianet.com.tn/650-smartwatch"},
		{Merchant: "mytek", URL: "https://www.mytek.tn/telephonie-tunisie/smartwatch.html"},
		{Merchant: "sbs", URL: "https://www.sbsinformatique.com/montre-connectee-tunisie"},
		{Merchant: "wiki", URL: "https://wiki.tn/montre-connectee"},
	}},

	// ======= Santé & Beauté =======
	{Category: "Santé & Beauté", Subcategory: types.SubcatSoinsVisage, Sources: []MerchantURL{
		{Merchant: "beautystore", URL: "https://beautystore.tn/215-soin-visage"},
		{Merchant: "fatales", URL: "https://www.fatales.tn/426-soins-visage"},
	}},
	{Category: "Santé & Beauté", Subcategory: types.SubcatMaquillage, Sources: []MerchantURL{
		{Merchant: "beautystore", URL: "https://beautystore.tn/210-maquillage"},
		{Merchant: "fatales", URL: "https://www.fatales.tn/417-maquillage"},
	}},
	{Category: "Santé & Beauté", Subcategory: types.SubcatCheveux, Sources: []MerchantURL{
		{Merchant: "beautystore", URL: "https://beautystore.tn/272-cheveux"},
		{Merchant: "fatales", URL: "https://www.fatales.tn/425-soins-cheveux"},
	}},
	{Category: "Santé & Beauté", Subcategory: types.SubcatParfumsCoffrets, Sources: []MerchantURL{
		{Merchant: "beautystore", URL: "https://beautystore.tn/363-coffrets"},
		{Merchant: "fatales", URL: "https://www.fatales.tn/506-coffret"},
	}},
	// NOTE: Fatales (PrestaShop) has no .facet sidebar on any page despite being a
	// PrestaShop store; only BeautyStore provides filters for Santé subcategories.
	{Category: "Santé & Beauté", Subcategory: types.SubcatHygieneToilette, Sources: []MerchantURL{
		{Merchant: "beautystore", URL: "https://beautystore.tn/205-soin-corps"},
	}},
	{Category: "Santé & Beauté", Subcategory: types.SubcatParapharmacieSante, Sources: []MerchantURL{
		{Merchant: "beautystore", URL: "https://beautystore.tn/330-solaire"},
	}},
	{Category: "Santé & Beauté", Subcategory: types.SubcatBebeMaman, Sources: []MerchantURL{
		{Merchant: "beautystore", URL: "https://beautystore.tn/403-soin-bebe-et-enfant"},
	}},
	{Category: "Santé & Beauté", Subcategory: types.SubcatSanteBeauteAutres, Sources: []MerchantURL{
		{Merchant: "beautystore", URL: "https://beautystore.tn/330-solaire"},
	}},

	// ======= Électroménager =======
	{Category: "Électroménager", Subcategory: types.SubcatMachineALaver, Sources: []MerchantURL{
		{Merchant: "tunisianet", URL: "https://www.tunisianet.com.tn/528-machine-a-laver"},
		{Merchant: "mytek", URL: "https://www.mytek.tn/electromenager/lavage/machine-a-laver.html"},
		{Merchant: "bigdepot", URL: "https://bigdepot.tn/74-machine-a-laver"},
	}},
	{Category: "Électroménager", Subcategory: types.SubcatSecheLinge, Sources: []MerchantURL{
		{Merchant: "tunisianet", URL: "https://www.tunisianet.com.tn/741-seche-linge"},
		{Merchant: "mytek", URL: "https://www.mytek.tn/electromenager/lavage/seche-linge.html"},
		{Merchant: "bigdepot", URL: "https://bigdepot.tn/77-seche-linge"},
	}},
	{Category: "Électroménager", Subcategory: types.SubcatLaveVaisselle, Sources: []MerchantURL{
		{Merchant: "tunisianet", URL: "https://www.tunisianet.com.tn/541-lave-vaisselle-tunisie"},
		{Merchant: "mytek", URL: "https://www.mytek.tn/electromenager/lavage/lave-vaisselle.html"},
		{Merchant: "bigdepot", URL: "https://bigdepot.tn/75-lave-vaisselle"},
	}},
	{Category: "Électroménager", Subcategory: types.SubcatFours, Sources: []MerchantURL{
		{Merchant: "tunisianet", URL: "https://www.tunisianet.com.tn/737-four-encastrable"},
		{Merchant: "mytek", URL: "https://www.mytek.tn/electromenager/cuisson/four-encastrable.html"},
		{Merchant: "bigdepot", URL: "https://bigdepot.tn/81-four-encastrable"},
	}},
	{Category: "Électroménager", Subcategory: types.SubcatChauffageClimatisation, Sources: []MerchantURL{
		{Merchant: "tunisianet", URL: "https://www.tunisianet.com.tn/457-climatiseur-tunisie-chaud-froid"},
		{Merchant: "mytek", URL: "https://www.mytek.tn/electromenager/climatisation.html"},
		{Merchant: "bigdepot", URL: "https://bigdepot.tn/83-climatiseur"},
	}},
	{Category: "Électroménager", Subcategory: types.SubcatAspirateurs, Sources: []MerchantURL{
		{Merchant: "tunisianet", URL: "https://www.tunisianet.com.tn/558-aspirateur-tunisie-vapeur"},
		{Merchant: "mytek", URL: "https://www.mytek.tn/electromenager/hygiene-et-soin-maison/aspirateur.html"},
		{Merchant: "bigdepot", URL: "https://bigdepot.tn/88-aspirateur"},
	}},

	// ======= Jardinage & Bricolage =======
	{Category: "Jardinage & Bricolage", Subcategory: types.SubcatOutillageJardinMotorise, Sources: []MerchantURL{
		{Merchant: "bigdepot", URL: "https://bigdepot.tn/91-outillage-de-jardinage"},
		{Merchant: "egm", URL: "https://egm.tn/nos-categories/jardinage/outil-de-jardin"},
	}},
	{Category: "Jardinage & Bricolage", Subcategory: types.SubcatOutillageElectroportatif, Sources: []MerchantURL{
		{Merchant: "bigdepot", URL: "https://bigdepot.tn/31-outillage-electroportatif"},
		{Merchant: "egm", URL: "https://egm.tn/nos-categories/outil-electro-portatif"},
	}},
	{Category: "Jardinage & Bricolage", Subcategory: types.SubcatArrosagePompes, Sources: []MerchantURL{
		{Merchant: "bigdepot", URL: "https://bigdepot.tn/93-arrosage-et-pompe"},
		{Merchant: "egm", URL: "https://egm.tn/nos-categories/jardinage/pompe-et-arrosage"},
	}},
	{Category: "Jardinage & Bricolage", Subcategory: types.SubcatOutillageMain, Sources: []MerchantURL{
		// Both BigDepot and EGM were removed because neither site renders sidebar filters
		// on their category pages. BigDepot's Electrive theme does not include the
		// PrestaShop faceted search module; EGM is a Next.js app with no filter UI.
		// This subcategory falls back to ES aggregation facets from categorySpecFields.
	}},
}

// -------- Shared helpers --------

var ignoredFilterTitles = map[string]bool{
	"en stock": true,
	"prix":     true,
}

var reWhitespace = regexp.MustCompile(`\s+`)
var reCountSuffix = regexp.MustCompile(`\s+\(\d+\)\s*$`)
var reTrailingCount = regexp.MustCompile(`\s+\d+\s*$`)

// Normalization regexes (compiled once)
var reStorageGB = regexp.MustCompile(`(?i)^(\d+)\s*(go|gb)$`)
var reStorageTB = regexp.MustCompile(`(?i)^(\d+)\s*(to|tb)$`)
var reWarranty = regexp.MustCompile(`(?i)^(\d+)\s*(an?s?|year?s?)?$`)
var reInch = regexp.MustCompile(`(?i)^([\d.,]+)\s*(pouces?|inch|inches|″|'')?\s*$`)
var reMAh = regexp.MustCompile(`(?i)^(\d+)\s*(mah)$`)

// -------- Normalization --------

func toTitle(s string) string {
	lower := strings.ToLower(s)
	prev := ' '
	chars := []rune(lower)
	for i, r := range chars {
		if prev == ' ' {
			chars[i] = unicode.ToTitle(r)
		}
		prev = r
	}
	return string(chars)
}

func normalizeGroupTitle(title string) string {
	t := strings.TrimSpace(title)
	if t == "" {
		return ""
	}
	lower := strings.ToLower(t)

	// Marque: exact match or "brand"
	if lower == "marque" || lower == "marques" || lower == "brand" {
		return "Marque"
	}

	// Stockage: exact match, equals "disque dur", or contains "stockage"
	if lower == "stockage" || lower == "disque dur" || lower == "capacité de stockage" || lower == "capacite de stockage" || strings.Contains(lower, "stockage") {
		return "Stockage"
	}

	// Mémoire / RAM: contains "mémoire", "memoire", or "ram"
	if strings.Contains(lower, "mémoire") || strings.Contains(lower, "memoire") || strings.Contains(lower, "ram") {
		return "Mémoire"
	}

	// Taille d'écran: contains "taille" and ("ecran" / "écran")
	if strings.Contains(lower, "taille") && (strings.Contains(lower, "ecran") || strings.Contains(lower, "écran")) {
		return "Taille de l'écran"
	}

	// Default: Title Case
	return toTitle(t)
}

func normalizeFilterValue(groupTitle, rawValue string) string {
	v := strings.TrimSpace(rawValue)
	v = reWhitespace.ReplaceAllString(v, " ")

	// Memory/RAM: "8go" / "8 gb" → "8 Go"
	v = reStorageGB.ReplaceAllString(v, "${1} Go")
	// Storage: "1to" / "1 tb" → "1 To"
	v = reStorageTB.ReplaceAllString(v, "${1} To")

	// Warranty: group-title must contain "garantie"
	if strings.Contains(strings.ToLower(groupTitle), "garantie") {
		if matches := reWarranty.FindStringSubmatch(v); len(matches) > 0 {
			num := matches[1]
			if num == "1" {
				v = "1 an"
			} else {
				v = num + " ans"
			}
		}
	}

	// Battery capacity: "10000mah" / "5000 Mah" → "10000 mAh"
	v = reMAh.ReplaceAllString(v, "$1 mAh")

	// Screen sizes: "15.6 pouces" / "15.6''" → "15.6\""
	v = reInch.ReplaceAllString(v, "$1\"")

	// Title-case for consistent dedup display
	v = toTitle(v)

	return v
}

// -------- Scraper: Tunisianet (custom PrestaShop module .af_filter) --------

func scrapeTunisianetFilters(url string) (map[string][]string, error) {
	filters := make(map[string][]string)
	seen := make(map[string]bool)

	c := colly.NewCollector(
		colly.AllowedDomains("www.tunisianet.com.tn", "tunisianet.com.tn"),
		colly.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"),
	)

	c.OnHTML(".af_filter", func(e *colly.HTMLElement) {
		titleEl := e.DOM.Find(".af_subtitle").First()
		if titleEl.Length() == 0 {
			return
		}
		rawTitle := strings.TrimSpace(titleEl.Text())
		if rawTitle == "" {
			return
		}
		normalized := strings.ToLower(strings.TrimSpace(reWhitespace.ReplaceAllString(rawTitle, " ")))
		if ignoredFilterTitles[normalized] {
			return
		}
		if seen[normalized] {
			return
		}
		seen[normalized] = true

		var values []string
		seenVals := make(map[string]bool)
		e.DOM.Find(".af_filter_content li").Each(func(_ int, s *goquery.Selection) {
			valSel := s.Find(".af_value")
			if valSel.Length() > 0 {
				s = valSel
			}
			v := strings.TrimSpace(s.Text())
			if v == "" {
				return
			}
			v = reWhitespace.ReplaceAllString(v, " ")
			v = reCountSuffix.ReplaceAllString(v, "")
			v = reTrailingCount.ReplaceAllString(v, "")
			v = strings.TrimSpace(v)
			if v == "" {
				return
			}
			lowV := strings.ToLower(v)
			if seenVals[lowV] {
				return
			}
			seenVals[lowV] = true
			values = append(values, v)
		})

		if len(values) > 0 {
			filters[rawTitle] = values
		}
	})

	c.OnError(func(r *colly.Response, err error) {
		log.Printf("[schema-scraper] request error for %s: %v", url, err)
	})

	if err := c.Visit(url); err != nil {
		return nil, err
	}
	c.Wait()
	return filters, nil
}

// -------- Scraper: PrestaShop 1.7+ stores (BeautyStore, Fatales, SBS, BigDepot) --------

func scrapePrestaShopFilters(url string, allowedDomains ...string) (map[string][]string, error) {
	if len(allowedDomains) == 0 {
		allowedDomains = []string{"beautystore.tn", "www.beautystore.tn"}
	}

	filters := make(map[string][]string)
	seen := make(map[string]bool)

	c := colly.NewCollector(
		colly.AllowedDomains(allowedDomains...),
		colly.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
		colly.AllowURLRevisit(),
	)
	c.SetRequestTimeout(30 * time.Second)
	c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Delay:       1 * time.Second,
		RandomDelay: 2 * time.Second,
	})

	c.OnRequest(func(r *colly.Request) {
		r.Headers.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
		r.Headers.Set("Accept-Language", "fr-FR,fr;q=0.9,en-US;q=0.8,en;q=0.7")
	})

	c.OnHTML(".facet", func(e *colly.HTMLElement) {
		titleEl := e.DOM.Find(".facet-title").First()
		if titleEl.Length() == 0 {
			return
		}
		rawTitle := strings.TrimSpace(titleEl.Text())
		if rawTitle == "" {
			return
		}
		normalized := strings.ToLower(strings.TrimSpace(reWhitespace.ReplaceAllString(rawTitle, " ")))
		if ignoredFilterTitles[normalized] {
			return
		}
		if seen[normalized] {
			return
		}
		seen[normalized] = true

		var values []string
		seenVals := make(map[string]bool)
		e.DOM.Find(".facet-label").Each(func(_ int, s *goquery.Selection) {
			a := s.Find("a")
			if a.Length() > 0 {
				s = a
			}
			v := strings.TrimSpace(s.Text())
			if v == "" {
				return
			}
			v = reWhitespace.ReplaceAllString(v, " ")
			v = reCountSuffix.ReplaceAllString(v, "")
			v = reTrailingCount.ReplaceAllString(v, "")
			v = strings.TrimSpace(v)
			if v == "" {
				return
			}
			lowV := strings.ToLower(v)
			if seenVals[lowV] {
				return
			}
			seenVals[lowV] = true
			values = append(values, v)
		})

		if len(values) > 0 {
			filters[rawTitle] = values
		}
	})

	c.OnError(func(r *colly.Response, err error) {
		log.Printf("[schema-scraper] request error for %s: %v", url, err)
	})

	if err := c.Visit(url); err != nil {
		return nil, err
	}
	c.Wait()
	return filters, nil
}

// -------- Scraper: Magento 2 stores (MyTek) --------

func scrapeMagentoFilters(url string, allowedDomains ...string) (map[string][]string, error) {
	if len(allowedDomains) == 0 {
		allowedDomains = []string{"www.mytek.tn"}
	}

	filters := make(map[string][]string)
	seen := make(map[string]bool)

	c := colly.NewCollector(
		colly.AllowedDomains(allowedDomains...),
		colly.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
		colly.AllowURLRevisit(),
	)
	c.SetRequestTimeout(30 * time.Second)
	c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Delay:       1 * time.Second,
		RandomDelay: 2 * time.Second,
	})

	c.OnRequest(func(r *colly.Request) {
		r.Headers.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
		r.Headers.Set("Accept-Language", "fr-FR,fr;q=0.9,en-US;q=0.8,en;q=0.7")
	})

	c.OnHTML(".filter-options-item", func(e *colly.HTMLElement) {
		titleEl := e.DOM.Find(".filter-options-title").First()
		if titleEl.Length() == 0 {
			return
		}
		rawTitle := strings.TrimSpace(titleEl.Text())
		if rawTitle == "" {
			return
		}
		normalized := strings.ToLower(strings.TrimSpace(reWhitespace.ReplaceAllString(rawTitle, " ")))
		if ignoredFilterTitles[normalized] {
			return
		}
		if seen[normalized] {
			return
		}
		seen[normalized] = true

		var values []string
		seenVals := make(map[string]bool)
		e.DOM.Find(".items .item a").Each(func(_ int, s *goquery.Selection) {
			v := strings.TrimSpace(s.Text())
			if v == "" {
				return
			}
			v = reWhitespace.ReplaceAllString(v, " ")
			v = reCountSuffix.ReplaceAllString(v, "")
			v = reTrailingCount.ReplaceAllString(v, "")
			v = strings.TrimSpace(v)
			if v == "" {
				return
			}
			lowV := strings.ToLower(v)
			if seenVals[lowV] {
				return
			}
			seenVals[lowV] = true
			values = append(values, v)
		})
		if len(values) > 0 {
			filters[rawTitle] = values
		}
	})

	c.OnHTML(".filter-current a[href]", func(e *colly.HTMLElement) {
		// Active filters are not useful for schema; skip
	})

	c.OnError(func(r *colly.Response, err error) {
		log.Printf("[schema-scraper] request error for %s: %v", url, err)
	})

	if err := c.Visit(url); err != nil {
		return nil, err
	}
	c.Wait()
	return filters, nil
}

// -------- Scraper: WooCommerce stores (Wiki, EGM) --------

func scrapeWooCommerceFilters(url string, allowedDomains ...string) (map[string][]string, error) {
	if len(allowedDomains) == 0 {
		allowedDomains = []string{"wiki.tn"}
	}

	filters := make(map[string][]string)
	seen := make(map[string]bool)

	c := colly.NewCollector(
		colly.AllowedDomains(allowedDomains...),
		colly.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
		colly.AllowURLRevisit(),
	)
	c.SetRequestTimeout(30 * time.Second)
	c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Delay:       1 * time.Second,
		RandomDelay: 2 * time.Second,
	})

	c.OnRequest(func(r *colly.Request) {
		r.Headers.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
		r.Headers.Set("Accept-Language", "fr-FR,fr;q=0.9,en-US;q=0.8,en;q=0.7")
	})

	c.OnHTML(".widget_layered_nav, .widget_layered_nav_filters", func(e *colly.HTMLElement) {
		titleEl := e.DOM.Find(".widget-title").First()
		if titleEl.Length() == 0 {
			return
		}
		rawTitle := strings.TrimSpace(titleEl.Text())
		if rawTitle == "" {
			return
		}
		normalized := strings.ToLower(strings.TrimSpace(reWhitespace.ReplaceAllString(rawTitle, " ")))
		if ignoredFilterTitles[normalized] {
			return
		}
		if seen[normalized] {
			return
		}
		seen[normalized] = true

		var values []string
		seenVals := make(map[string]bool)
		e.DOM.Find("ul li a").Each(func(_ int, s *goquery.Selection) {
			v := strings.TrimSpace(s.Text())
			if v == "" {
				return
			}
			v = reWhitespace.ReplaceAllString(v, " ")
			v = reCountSuffix.ReplaceAllString(v, "")
			v = reTrailingCount.ReplaceAllString(v, "")
			v = strings.TrimSpace(v)
			if v == "" {
				return
			}
			lowV := strings.ToLower(v)
			if seenVals[lowV] {
				return
			}
			seenVals[lowV] = true
			values = append(values, v)
		})
		if len(values) > 0 {
			filters[rawTitle] = values
		}
	})

	c.OnError(func(r *colly.Response, err error) {
		log.Printf("[schema-scraper] request error for %s: %v", url, err)
	})

	if err := c.Visit(url); err != nil {
		return nil, err
	}
	c.Wait()
	return filters, nil
}

// -------- Merchant dispatcher --------

func scrapeMerchantFilters(merchant, url string) (map[string][]string, error) {
	switch merchant {
	case "tunisianet":
		return scrapeTunisianetFilters(url)
	case "beautystore":
		return scrapePrestaShopFilters(url, "beautystore.tn", "www.beautystore.tn")
	case "fatales":
		return scrapePrestaShopFilters(url, "www.fatales.tn", "fatales.tn")
	case "sbs":
		return scrapePrestaShopFilters(url, "www.sbsinformatique.com", "sbsinformatique.com")
	case "bigdepot":
		return scrapePrestaShopFilters(url, "bigdepot.tn", "www.bigdepot.tn")
	case "mytek":
		return scrapeMagentoFilters(url, "www.mytek.tn", "mytek.tn")
	case "wiki":
		return scrapeWooCommerceFilters(url, "wiki.tn", "www.wiki.tn")
	case "egm":
		return scrapeWooCommerceFilters(url, "egm.tn", "www.egm.tn")
	default:
		log.Printf("[schema-scraper] unknown merchant %q, skipping", merchant)
		return nil, nil
	}
}

// -------- Merge / Deduplication --------

func MergeSubcategorySchemas(schemas []*CategorySchema) *CategorySchema {
	if len(schemas) == 0 {
		return nil
	}
	merged := &CategorySchema{
		Subcategory: schemas[0].Subcategory,
		Filters:     make(map[string][]string),
	}
	mergedKeys := make(map[string]string) // lowercased key -> preferred case

	for _, s := range schemas {
		if s == nil {
			continue
		}
		for key, vals := range s.Filters {
			canonicalTitle := normalizeGroupTitle(key)
			if canonicalTitle == "" {
				continue
			}
			normalizedKey := strings.ToLower(canonicalTitle)
			// Remember the first-seen casing for the key
			if _, exists := mergedKeys[normalizedKey]; !exists {
				mergedKeys[normalizedKey] = canonicalTitle
			}
			canonical := mergedKeys[normalizedKey]

			seenVals := make(map[string]bool)
			for _, v := range merged.Filters[canonical] {
				seenVals[strings.ToLower(v)] = true
			}
			for _, v := range vals {
				normalized := normalizeFilterValue(key, v)
				if normalized == "" {
					continue
				}
				normKey := strings.ToLower(normalized)
				if seenVals[normKey] {
					continue
				}
				seenVals[normKey] = true
				merged.Filters[canonical] = append(merged.Filters[canonical], normalized)
			}
		}
	}

	return merged
}

// -------- Entry point --------

func ScrapeFilterSchemas() map[types.Subcategory]map[string][]string {
	results := make(map[types.Subcategory]map[string][]string)

	for _, src := range subcategorySources {
		subcat := src.Subcategory
		if len(src.Sources) == 0 {
			continue
		}

		var schemas []*CategorySchema

		for _, ms := range src.Sources {
			filters, err := scrapeMerchantFilters(ms.Merchant, ms.URL)
			if err != nil {
				log.Printf("[schema-scraper] error scraping %s/%s from %s: %v", subcat, ms.Merchant, ms.URL, err)
				continue
			}
			if len(filters) == 0 {
				log.Printf("[schema-scraper] %s/%s: 0 filter groups from %s", subcat, ms.Merchant, ms.URL)
				continue
			}
			log.Printf("[schema-scraper] %s/%s: %d filter groups from %s", subcat, ms.Merchant, len(filters), ms.URL)
			schemas = append(schemas, &CategorySchema{
				Subcategory: string(subcat),
				Filters:     filters,
			})
		}

		if len(schemas) == 0 {
			if subcat == types.SubcatOutillageMain || string(subcat) == "Outillage à main" {
				log.Printf("[schema-scraper] Outillage à main: 0 schemas scraped (no merchants with sidebar filters)")
			}
			continue
		}

		merged := MergeSubcategorySchemas(schemas)
		if merged == nil || len(merged.Filters) == 0 {
			continue
		}

		results[subcat] = merged.Filters
		log.Printf("[schema-scraper] Subcategory '%s': successfully merged %d merchants into 1 schema (%d filter groups)",
			subcat, len(schemas), len(merged.Filters))

		// Validation: print exact filter group names for Outillage à main
		if subcat == "Outillage à main" || subcat == types.SubcatOutillageMain {
			names := make([]string, 0, len(merged.Filters))
			for name := range merged.Filters {
				names = append(names, name)
			}
			log.Printf("[schema-scraper] Outillage à main filter groups: %v", names)
		}
	}

	return results
}
