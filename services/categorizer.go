package services

import (
	"fmt"
	"project/types"
	"regexp"
	"strings"
)

type categoryRule struct {
	keywords    []string
	category    types.Category
	subcategory types.Subcategory
	regex       *regexp.Regexp
}

var categoryRules = []categoryRule{
	// --- Beauty subcategories (specific first) ---
	{[]string{"soin visage", "crème visage", "sérum", "anti-âge", "contour des yeux", "solaire", "soin solaire", "antirides", "fotoprotector", "écran solaire", "ecran solaire", "protection solaire", "crème solaire", "creme solaire", "hydratant", "gel nettoyant", "micellaire", "anti-taches", "anti-imperfections", "acide hyaluronique"}, types.CategorySanteBeaute, types.SubcatSoinsVisage, nil},
	{[]string{"maquillage", "fond de teint", "mascara", "rouge à lèvres", "eye-liner", "vernis", "fard", "palette", "crayon", "poudre", "bb crème", "bb creme", "cc crème", "blush", "gloss", "anti-cernes"}, types.CategorySanteBeaute, types.SubcatMaquillage, nil},
	{[]string{"cheveux", "shampooing", "après-shampooing", "soin capillaire", "coloration", "masque cheveux", "huile cheveux", "frisottis", "anti-frisottis", "shampoing", "démêlant", "lissage", "kératine", "keratine", "boucles"}, types.CategorySanteBeaute, types.SubcatCheveux, nil},
	{[]string{"bébé", "bebe", "enfant", "biberon", "couche", "poussette", "lait infantile", "nursing", "jouet", "peluche", "baby"}, types.CategorySanteBeaute, types.SubcatBebeMaman, nil},
	{[]string{"eau de parfum", "eau de toilette", "eau de cologne", "coffret parfum", "extrait de parfum", "parfum", "coffret"}, types.CategorySanteBeaute, types.SubcatParfumsCoffrets, nil},
	{[]string{"toilette", "hygiène", "savon", "shampoo", "gel", "déodorant", "dentifrice", "brosse à dents", "rasoir", "soin du corps", "lotion corporelle"}, types.CategorySanteBeaute, types.SubcatHygieneToilette, nil},
	{[]string{"tensiomètre", "thermomètre", "glucomètre", "oxymètre", "balance impédancemètre", "moniteur cardiaque", "brûlé cardio", "pharmacie", "médicament", "vitamine", "complément", "paracétamol", "traitement", "aspirine", "antihistaminique"}, types.CategorySanteBeaute, types.SubcatParapharmacieSante, nil},
	{[]string{"soin", "crème", "cosmétique", "huile essentielle", "démaquillant"}, types.CategorySanteBeaute, types.SubcatSanteBeauteAutres, nil},

	// --- Électroménager ---
	{[]string{"aspirateur", "robot aspirateur", "balai", "aspirateur balai", "aspirateur traîneau"}, types.CategoryElectromenager, types.SubcatAspirateurs, nil},
	{[]string{"machine à laver", "lave-linge", "lave linge", "lavante", "lave"}, types.CategoryElectromenager, types.SubcatMachineALaver, nil},
	{[]string{"sèche-linge", "seche linge", "sèche linge"}, types.CategoryElectromenager, types.SubcatSecheLinge, nil},
	{[]string{"lave-vaisselle", "lave vaisselle"}, types.CategoryElectromenager, types.SubcatLaveVaisselle, nil},
	{[]string{"four", "micro-ondes", "micro ondes", "cuisinière", "plaque cuisson", "plaques induction", "chauffe-eau", "bouilloire"}, types.CategoryElectromenager, types.SubcatFours, nil},
	{[]string{"climatiseur", "climatisation", "chauffage", "radiateur", "ventilateur", "électroménager", "électro"}, types.CategoryElectromenager, types.SubcatChauffageClimatisation, nil},

	// --- Téléphonie ---
	{[]string{"montre", "smartwatch", "apple watch", "fitness tracker", "bracelet connecté", "garmin", "polar"}, types.CategoryTelephonie, types.SubcatSmartwatch, nil},
	{[]string{"téléphone portable", "mobile", "nokia", "cellulaire", "gsm"}, types.CategoryTelephonie, types.SubcatTelephonesPortables, nil},
	{[]string{"téléphone fixe", "poste téléphonique", "combiné", "fixe sans fil", "dect"}, types.CategoryTelephonie, types.SubcatTelephonesFixes, nil},
	{[]string{"coque", "chargeur", "écouteur", "écouteurs", "support", "batterie externe", "power bank", "film", "verre trempé", "kit mains-libres", "câble usb"}, types.CategoryTelephonie, types.SubcatAccessoires, nil},
	{[]string{"smartphone", "iphone", "samsung galaxy", "huawei", "oppo", "xiaomi", "oneplus", "pixel"}, types.CategoryTelephonie, types.SubcatSmartphones, nil},

	// --- Informatique ---
	{[]string{"souris", "clavier", "tapis", "casque", "webcam", "ecran", "moniteur", "imprimante", "scanner", "haut-parleur", "enceinte", "microphone", "micro-casque", "kit piétonnier"}, types.CategoryInformatique, types.SubcatPeripheriques, nil},
	{[]string{"pc", "ordinateur", "laptop", "macbook", "desktop", "imac", "notebook", "ultrabook", "workstation", "serveur", "thinkpad", "pavilion", "elitebook", "probook", "ideapad", "surface", "chromebook", "aspire"}, types.CategoryInformatique, types.SubcatOrdinateurs, nil},
	{[]string{"routeur", "switch", "cable", "câble", "wifi", "ethernet", "modem", "hub", "répéteur", "access point", "hotspot", "lan", "rj45", "fibre", "adaptateur réseau", "carte réseau", "transceiver", "nas", "cloud", "protecteur"}, types.CategoryInformatique, types.SubcatReseauxConnectivite, nil},
	{[]string{"disque", "ssd", "hdd", "flash", "ram", "mémoire", "stockage", "disque dur", "clé usb", "carte sd", "nvme", "m\\.2", "ddr", "graveur", "lecteur"}, types.CategoryInformatique, types.SubcatStockages, nil},
	{[]string{"processeur", "cpu", "carte mère", "motherboard", "gpu", "carte graphique", "alimentation", "ventilateur", "boîtier", "refroidissement", "dissipateur", "watercooling"}, types.CategoryInformatique, types.SubcatComposants, nil},

	// --- Jardinage & Bricolage ---
	{[]string{"tondeuse", "tronçonneuse", "débroussailleuse", "taille-haie", "taille haie", "souffleur", "coupe-bordure", "motoculteur", "bêche", "motobêche"}, types.CategoryJardinageBricolage, types.SubcatOutillageJardinMotorise, nil},
	{[]string{"perceuse", "meuleuse", "scie", "ponceuse", "marteau perforateur", "visseuse", "perforateur", "burineur", "rainureuse", "défonceuse", "scie sauteuse", "scie circulaire", "scie à onglet", "scie sabre"}, types.CategoryJardinageBricolage, types.SubcatOutillageElectroportatif, nil},
	{[]string{"pulvérisateur", "pulverisateur", "pompe", "tuyau", "enrouleur", "arrosage", "nettoyeur haute pression", "nettoyeur hp", "jet haute pression", "jet hp", "récupérateur d'eau", "recuperateur d'eau"}, types.CategoryJardinageBricolage, types.SubcatArrosagePompes, nil},
	{[]string{"sécateur", "secateur", "cisaille", "hache", "pelle", "tournevis", "pince", "marteau", "grattoir", "cutter", "niveau à bulle", "niveau", "mètre ruban", "ruban", "serre-joint", "serre joint", "égoïne", "scie égoïne", "burin", "coupe-tube"}, types.CategoryJardinageBricolage, types.SubcatOutillageMain, nil},
}

func init() {
	for i := range categoryRules {
		var patternParts []string
		for _, kw := range categoryRules[i].keywords {
			escaped := regexp.QuoteMeta(kw)
			patternParts = append(patternParts, fmt.Sprintf(`\b%s\b`, escaped))
		}

		pattern := fmt.Sprintf(`(?i)(%s)`, strings.Join(patternParts, "|"))
		categoryRules[i].regex = regexp.MustCompile(pattern)
	}
}

func AssignProductCategory(productName string) (types.Category, types.Subcategory) {
	if strings.TrimSpace(productName) == "" {
		return types.CategoryUncategorized, types.SubcatUncategorized
	}

	for _, rule := range categoryRules {
		if rule.regex.MatchString(productName) {
			return rule.category, rule.subcategory
		}
	}

	return types.CategoryUncategorized, types.SubcatUncategorized
}
