package types

type Category string
type Subcategory string

const (
	CategoryInformatique         Category = "Informatique"
	CategoryTelephonie           Category = "Téléphonie"
	CategorySanteBeaute          Category = "Santé & Beauté"
	CategoryElectromenager       Category = "Électroménager"
	CategoryJardinageBricolage   Category = "Jardinage & Bricolage"
	CategoryUncategorized        Category = "Uncategorized"
)

const (
	SubcatComposants                  Subcategory = "Composants"
	SubcatOrdinateurs                 Subcategory = "Ordinateurs"
	SubcatReseauxConnectivite         Subcategory = "Réseaux et connectivité"
	SubcatPeripheriques               Subcategory = "Périphériques"
	SubcatStockages                   Subcategory = "Stockages"
	SubcatSmartphones                 Subcategory = "Smartphones"
	SubcatTelephonesPortables         Subcategory = "Téléphones portables"
	SubcatAccessoires                 Subcategory = "Accessoires"
	SubcatTelephonesFixes             Subcategory = "Téléphones fixes"
	SubcatSmartwatch                  Subcategory = "Smartwatch"
	SubcatSoinsVisage                 Subcategory = "Soins visage"
	SubcatMaquillage                  Subcategory = "Maquillage"
	SubcatCheveux                     Subcategory = "Cheveux"
	SubcatHygieneToilette             Subcategory = "Hygiène & toilette"
	SubcatParapharmacieSante          Subcategory = "Parapharmacie & santé"
	SubcatBebeMaman                   Subcategory = "Bébé & maman"
	SubcatParfumsCoffrets             Subcategory = "Parfums & coffrets"
	SubcatSanteBeauteAutres           Subcategory = "Santé & beauté autres"
	SubcatAspirateurs                 Subcategory = "Aspirateurs"
	SubcatMachineALaver               Subcategory = "Machine à laver"
	SubcatSecheLinge                  Subcategory = "Sèche-linge"
	SubcatLaveVaisselle               Subcategory = "Lave-vaisselle"
	SubcatFours                       Subcategory = "Fours"
	SubcatChauffageClimatisation      Subcategory = "Chauffage & Climatisation"
	SubcatUncategorized               Subcategory = "Uncategorized"
	SubcatOutillageJardinMotorise     Subcategory = "Outillage jardin motorisé"
	SubcatOutillageElectroportatif    Subcategory = "Outillage électroportatif"
	SubcatArrosagePompes             Subcategory = "Arrosage & pompes"
	SubcatOutillageMain              Subcategory = "Outillage à main"
)

var SubcategoryToCategory map[Subcategory]Category

func init() {
	SubcategoryToCategory = make(map[Subcategory]Category, 32)
	for cat, subs := range CategorySubcategories {
		for _, sub := range subs {
			SubcategoryToCategory[sub] = cat
		}
	}
}

var CategorySubcategories = map[Category][]Subcategory{
	CategoryInformatique: {
		SubcatComposants,
		SubcatOrdinateurs,
		SubcatReseauxConnectivite,
		SubcatPeripheriques,
		SubcatStockages,
	},
	CategoryTelephonie: {
		SubcatTelephonesPortables,
		SubcatSmartphones,
		SubcatAccessoires,
		SubcatTelephonesFixes,
		SubcatSmartwatch,
	},
	CategorySanteBeaute: {
		SubcatSoinsVisage,
		SubcatMaquillage,
		SubcatCheveux,
		SubcatHygieneToilette,
		SubcatParapharmacieSante,
		SubcatBebeMaman,
		SubcatParfumsCoffrets,
		SubcatSanteBeauteAutres,
	},
	CategoryElectromenager: {
		SubcatAspirateurs,
		SubcatMachineALaver,
		SubcatSecheLinge,
		SubcatLaveVaisselle,
		SubcatFours,
		SubcatChauffageClimatisation,
	},
	CategoryJardinageBricolage: {
		SubcatOutillageJardinMotorise,
		SubcatOutillageElectroportatif,
		SubcatArrosagePompes,
		SubcatOutillageMain,
	},
	CategoryUncategorized: {
		SubcatUncategorized,
	},
}
