package internal

var factionsLayersModes = struct {
	Factions map[string]string
	Layers   map[string]struct {
		Name  string
		Short string
	}
	GameModes map[string]struct {
		Name  string
		Color int
		Short string
	}
	MapNames map[string]struct {
		Name       string
		ImageUrl   string
		GalleryUrl string
	}
}{
	Factions: map[string]string{
		"US":          "USMC",
		"ARG82":       "Argentina",
		"RU":          "Russia",
		"ARF":         "ARF",
		"CF":          "Canada",
		"CH":          "PLA",
		"CHinsurgent": "Militia",
		"FR":          "France",
		"fsa":         "FSA",
		"GB":          "Britain",
		"GB82":        "Britain",
		"HAMAS":       "Hamas",
		"IDF":         "Israel",
		"GER":         "Germany",
		"MEC":         "MEC",
		"MEInsurgent": "Insurgent",
		"PL":          "Poland",
		"TALIBAN":     "Taliban",
		"USA":         "US Army",
		"vnnva":       "NVA",
		"vnusa":       "US Army",
		"vnusmc":      "USMC",
		"NL":          "Netherlands",
		"ww2usa":      "US ARMY",
		"ww2ger":      "Wehrmacht",
	},
	Layers: map[string]struct {
		Name  string
		Short string
	}{
		"16": {
			Name:  "Infantry",
			Short: "INF",
		},
		"32": {
			Name:  "Alternative",
			Short: "ALT",
		},
		"64": {
			Name:  "Standard",
			Short: "STD",
		},
		"128": {
			Name:  "Large",
			Short: "LRG",
		},
	},
	GameModes: map[string]struct {
		Name  string
		Color int
		Short string
	}{
		"gpm_cq": {
			Name:  "Assault & Secure",
			Color: 0x4284F5,
			Short: "AAS",
		},
		"gpm_cnc": {
			Name:  "Command & Control",
			Color: 0xD1F542,
			Short: "CNC",
		},
		"gpm_coop": {
			Name:  "Co-Operation",
			Color: 0x42F596,
			Short: "COOP",
		},
		"gpm_insurgency": {
			Name:  "Insurgency",
			Color: 0xF54242,
			Short: "INS",
		},
		"gpm_skirmish": {
			Name:  "Skirmish",
			Color: 0x858585,
			Short: "SKR",
		},
		"gpm_vehicles": {
			Name:  "Vehicle Warfare",
			Color: 0xF542DD,
			Short: "VW",
		},
		"gpm_gungame": {
			Name:  "Gungame",
			Color: 0x04646A,
			Short: "GG",
		},
	},
	MapNames: map[string]struct {
		Name       string
		ImageUrl   string
		GalleryUrl string
	}{
		"adak": {
			Name:       "Adak - BETA (4km)",
			ImageUrl:   "https://www.realitymod.com/mapgallery/images/maps/adak-beta/tile.jpg",
			GalleryUrl: "https://www.realitymod.com/mapgallery/#!/adak-beta/",
		},
		"albasrah_2": {
			Name:       "Al Basrah (2km)",
			ImageUrl:   "https://www.realitymod.com/mapgallery/images/maps/albasrah/tile.jpg",
			GalleryUrl: "https://www.realitymod.com/mapgallery/#!/albasrah/",
		},
		"asad_khal": {
			Name:       "Asad Khal (1km)",
			ImageUrl:   "https://www.realitymod.com/mapgallery/images/maps/asadkhal/tile.jpg",
			GalleryUrl: "https://www.realitymod.com/mapgallery/#!/asadkhal/",
		},
		"ascheberg": {
			Name:       "Asheberg - BETA (4km)",
			ImageUrl:   "https://www.realitymod.com/mapgallery/images/maps/ascheberg-beta/tile.jpg",
			GalleryUrl: "https://www.realitymod.com/mapgallery/#!/ascheberg-beta/",
		},
		"assault_on_grozny": {
			Name:       "Assault on Grozny (2km)",
			ImageUrl:   "https://www.realitymod.com/mapgallery/images/maps/assaultongrozny/tile.jpg",
			GalleryUrl: "https://www.realitymod.com/mapgallery/#!/assaultongrozny/",
		},
		"assault_on_mestia": {
			Name:       "Assault on Mestia (1km)",
			ImageUrl:   "https://www.realitymod.com/mapgallery/images/maps/assaultonmestia/tile.jpg",
			GalleryUrl: "https://www.realitymod.com/mapgallery/#!/assaultonmestia/",
		},
		"bamyan": {
			Name:       "Bamyan (4km)",
			ImageUrl:   "https://www.realitymod.com/mapgallery/images/maps/bamyan/tile.jpg",
			GalleryUrl: "https://www.realitymod.com/mapgallery/#!/bamyan/",
		},
		"battle_of_ia_drang": {
			Name:       "Battle of Ia Drang (2km)",
			ImageUrl:   "https://www.realitymod.com/mapgallery/images/maps/battleofiadrang/tile.jpg",
			GalleryUrl: "https://www.realitymod.com/mapgallery/#!/battleofiadrang/",
		},
		"beirut": {
			Name:       "Beirut (2km)",
			ImageUrl:   "https://www.realitymod.com/mapgallery/images/maps/beirut/tile.jpg",
			GalleryUrl: "https://www.realitymod.com/mapgallery/#!/beirut/",
		},
		"bijar_canyons": {
			Name:       "Bijar Crayons (4km)",
			ImageUrl:   "https://www.realitymod.com/mapgallery/images/maps/bijarcanyons/tile.jpg",
			GalleryUrl: "https://www.realitymod.com/mapgallery/#!/bijarcanyons/",
		},
		"black_gold": {
			Name:       "Black Gold (4km)",
			ImageUrl:   "https://www.realitymod.com/mapgallery/images/maps/blackgold/tile.jpg",
			GalleryUrl: "https://www.realitymod.com/mapgallery/#!/blackgold/",
		},
		"brecourt_assault": {
			Name:       "Brecourt Assault (2km)",
			ImageUrl:   "https://www.realitymod.com/mapgallery/images/maps/brecourtassault/tile.jpg",
			GalleryUrl: "https://www.realitymod.com/mapgallery/#!/brecourtassault/",
		},
		"burning_sands": {
			Name:       "Burning Sands (4km)",
			ImageUrl:   "https://www.realitymod.com/mapgallery/images/maps/burningsands/tile.jpg",
			GalleryUrl: "https://www.realitymod.com/mapgallery/#!/burningsands/",
		},
		"carentan": {
			Name:       "Carentan (2km)",
			ImageUrl:   "https://www.realitymod.com/mapgallery/images/maps/carentan/tile.jpg",
			GalleryUrl: "https://www.realitymod.com/mapgallery/#!/carentan/",
		},
		"charlies_point": {
			Name:       "Charlie's Point (2km)",
			ImageUrl:   "https://www.realitymod.com/mapgallery/images/maps/charliespoint/tile.jpg",
			GalleryUrl: "https://www.realitymod.com/mapgallery/#!/charliespoint/",
		},
		"dovre": {
			Name:       "Dovre (2km)",
			ImageUrl:   "https://www.realitymod.com/mapgallery/images/maps/dovre/tile.jpg",
			GalleryUrl: "https://www.realitymod.com/mapgallery/#!/dovre/",
		},
		"dovre_winter": {
			Name:       "Dovre Winter (2km)",
			ImageUrl:   "https://www.realitymod.com/mapgallery/images/maps/dovrewinter/tile.jpg",
			GalleryUrl: "https://www.realitymod.com/mapgallery/#!/dovrewinter/",
		},
		"dragon_fly": {
			Name:       "Dragon Fly (2km)",
			ImageUrl:   "https://www.realitymod.com/mapgallery/images/maps/dragonfly/tile.jpg",
			GalleryUrl: "https://www.realitymod.com/mapgallery/#!/dragonfly/",
		},
		"fallujah_west": {
			Name:       "Fallujah West (1km)",
			ImageUrl:   "https://www.realitymod.com/mapgallery/images/maps/fallujahwest/tile.jpg",
			GalleryUrl: "https://www.realitymod.com/mapgallery/#!/fallujahwest/",
		},
		"fields_of_kassel": {
			Name:       "Fields of Kassel - BETA (4km)",
			ImageUrl:   "https://www.realitymod.com/mapgallery/images/maps/fieldsofkassel-beta/tile.jpg",
			GalleryUrl: "https://www.realitymod.com/mapgallery/#!/fieldsofkassel-beta/",
		},
		"fools_road": {
			Name:       "Fools Road (2km)",
			ImageUrl:   "https://www.realitymod.com/mapgallery/images/maps/foolsroad/tile.jpg",
			GalleryUrl: "https://www.realitymod.com/mapgallery/#!/foolsroad/",
		},
		"gaza_2": {
			Name:       "Gaza (1km)",
			ImageUrl:   "https://www.realitymod.com/mapgallery/images/maps/gaza/tile.jpg",
			GalleryUrl: "https://www.realitymod.com/mapgallery/#!/gaza/",
		},
		"goose_green": {
			Name:       "Goose Green (2km)",
			ImageUrl:   "https://www.realitymod.com/mapgallery/images/maps/goosegreen/tile.jpg",
			GalleryUrl: "https://www.realitymod.com/mapgallery/#!/goosegreen/",
		},
		"hades_peak": {
			Name:       "Hades Peak (4km)",
			ImageUrl:   "https://www.realitymod.com/mapgallery/images/maps/goosegreen/tile.jpg",
			GalleryUrl: "https://www.realitymod.com/mapgallery/#!/hadespeak/",
		},
		"hill_488": {
			Name:       "Hill 488 (1km)",
			ImageUrl:   "https://www.realitymod.com/mapgallery/images/maps/hill488/tile.jpg",
			GalleryUrl: "https://www.realitymod.com/mapgallery/#!/hill488/",
		},
		"iron_ridge": {
			Name:       "Iron Ridge (2km)",
			ImageUrl:   "https://www.realitymod.com/mapgallery/images/maps/ironridge/tile.jpg",
			GalleryUrl: "https://www.realitymod.com/mapgallery/#!/ironridge/",
		},
		"jabal": {
			Name:       "Jabal Al Burj (2km)",
			ImageUrl:   "https://www.realitymod.com/mapgallery/images/maps/jabalalburj/tile.jpg",
			GalleryUrl: "https://www.realitymod.com/mapgallery/#!/jabalalburj/",
		},
		"kafar_halab": {
			Name:       "Kafr Halab (2km)",
			ImageUrl:   "https://www.realitymod.com/mapgallery/images/maps/kafrhalab/tile.jpg",
			GalleryUrl: "https://www.realitymod.com/mapgallery/#!/kafrhalab/",
		},
		"karbala": {
			Name:       "Karbala (2km)",
			ImageUrl:   "https://www.realitymod.com/mapgallery/images/maps/karbala/tile.jpg",
			GalleryUrl: "https://www.realitymod.com/mapgallery/#!/karbala/",
		},
		"kashan_desert": {
			Name:       "Kashan Desert (4km)",
			ImageUrl:   "https://www.realitymod.com/mapgallery/images/maps/kashandesert/tile.jpg",
			GalleryUrl: "https://www.realitymod.com/mapgallery/#!/karbala/",
		},
		"khamisiyah": {
			Name:       "Khamisiyah (4km)",
			ImageUrl:   "https://www.realitymod.com/mapgallery/images/maps/khamisiyah/tile.jpg",
			GalleryUrl: "https://www.realitymod.com/mapgallery/#!/khamisiyah/",
		},
		"kokan": {
			Name:       "Kokan (2km)",
			ImageUrl:   "https://www.realitymod.com/mapgallery/images/maps/kokan/tile.jpg",
			GalleryUrl: "https://www.realitymod.com/mapgallery/#!/kokan/",
		},
		"korbach_offensive": {
			Name:       "Korbach Offensive - BETA (4km)",
			ImageUrl:   "https://www.realitymod.com/mapgallery/images/maps/korbachoffensive-beta/tile.jpg",
			GalleryUrl: "https://www.realitymod.com/mapgallery/#!/korbachoffensive-beta/",
		},
		"korengal": {
			Name:       "Korengal Valley (1km)",
			ImageUrl:   "https://www.realitymod.com/mapgallery/images/maps/korengalvalley/tile.jpg",
			GalleryUrl: "https://www.realitymod.com/mapgallery/#!/korengalvalley/",
		},
		"kozelsk": {
			Name:       "Kozelsk (2km)",
			ImageUrl:   "https://www.realitymod.com/mapgallery/images/maps/kozelsk/tile.jpg",
			GalleryUrl: "https://www.realitymod.com/mapgallery/#!/kozelsk/",
		},
		"kunar_province": {
			Name:       "Kunar Province - BETA (4Km)",
			ImageUrl:   "https://www.realitymod.com/mapgallery/images/maps/kunarprovince-beta/tile.jpg",
			GalleryUrl: "https://www.realitymod.com/mapgallery/#!/kunarprovince-beta/",
		},
		"lashkar_valley": {
			Name:       "Lashkar Valley (2km)",
			ImageUrl:   "https://www.realitymod.com/mapgallery/images/maps/lashkarvalley/tile.jpg",
			GalleryUrl: "https://www.realitymod.com/mapgallery/#!/lashkarvalley/",
		},
		"masirah": {
			Name:       "Masirah (4km)",
			ImageUrl:   "https://www.realitymod.com/mapgallery/images/maps/masirah/tile.jpg",
			GalleryUrl: "https://www.realitymod.com/mapgallery/#!/masirah/",
		},
		"merville": {
			Name:       "Merville (1km)",
			ImageUrl:   "https://www.realitymod.com/mapgallery/images/maps/merville/tile.jpg",
			GalleryUrl: "https://www.realitymod.com/mapgallery/#!/merville/",
		},
		"musa_qala": {
			Name:       "Musa Qala - BETA (2km)",
			ImageUrl:   "https://www.realitymod.com/mapgallery/images/maps/musaqala-beta/tile.jpg",
			GalleryUrl: "https://www.realitymod.com/mapgallery/#!/musaqala-beta/",
		},
		"muttrah_city_2": {
			Name:       "Muttrah City (2km)",
			ImageUrl:   "https://www.realitymod.com/mapgallery/images/maps/muttrahcity/tile.jpg",
			GalleryUrl: "https://www.realitymod.com/mapgallery/#!/muttrahcity/",
		},
		"nuijamaa": {
			Name:       "Nuijamaa (2km)",
			ImageUrl:   "https://www.realitymod.com/mapgallery/images/maps/nuijamaa/tile.jpg",
			GalleryUrl: "https://www.realitymod.com/mapgallery/#!/nuijamaa/",
		},
		"omaha_beach": {
			Name:       "Omaha Beach (2km)",
			ImageUrl:   "https://www.realitymod.com/mapgallery/images/maps/omahabeach/tile.jpg",
			GalleryUrl: "https://www.realitymod.com/mapgallery/#!/omahabeach/",
		},
		"op_barracuda": {
			Name:       "Operation Barracuda (2km)",
			ImageUrl:   "https://www.realitymod.com/mapgallery/images/maps/operationbarracuda/tile.jpg",
			GalleryUrl: "https://www.realitymod.com/mapgallery/#!/operationbarracuda/",
		},
		"operation_bobcat": {
			Name:       "Operation Bobcat (4km)",
			ImageUrl:   "https://www.realitymod.com/mapgallery/images/maps/operationbobcat/tile.jpg",
			GalleryUrl: "https://www.realitymod.com/mapgallery/#!/operationbobcat/",
		},
		"operation_falcon": {
			Name:       "Operation Falcon (2km)",
			ImageUrl:   "https://www.realitymod.com/mapgallery/images/maps/operationfalcon/tile.jpg",
			GalleryUrl: "https://www.realitymod.com/mapgallery/#!/operationfalcon/",
		},
		"operation_ghost_train": {
			Name:       "Operation Ghost Train (1km)",
			ImageUrl:   "https://www.realitymod.com/mapgallery/images/maps/operationghosttrain/tile.jpg",
			GalleryUrl: "https://www.realitymod.com/mapgallery/#!/operationghosttrain/",
		},
		"operation_marlin": {
			Name:       "Operation Marlin (2km)",
			ImageUrl:   "https://www.realitymod.com/mapgallery/images/maps/operationmarlin/tile.jpg",
			GalleryUrl: "https://www.realitymod.com/mapgallery/#!/operationmarlin/",
		},
		"operation_soul_rebel": {
			Name:       "Operation Soul Rebel (4km)",
			ImageUrl:   "https://www.realitymod.com/mapgallery/images/maps/operationsoulrebel/tile.jpg",
			GalleryUrl: "https://www.realitymod.com/mapgallery/#!/operationsoulrebel/",
		},
		"operation_thunder": {
			Name:       "Operation Thunder - BETA (4km)",
			ImageUrl:   "https://www.realitymod.com/mapgallery/images/maps/operationthunder-beta/tile.jpg",
			GalleryUrl: "https://www.realitymod.com/mapgallery/#!/operationthunder-beta/",
		},
		"outpost": {
			Name:       "Outpost (2km)",
			ImageUrl:   "https://www.realitymod.com/mapgallery/images/maps/outpost/tile.jpg",
			GalleryUrl: "https://www.realitymod.com/mapgallery/#!/outpost/",
		},
		"pavlovsk_bay": {
			Name:       "Pavlovsk Bay (4km)",
			ImageUrl:   "https://www.realitymod.com/mapgallery/images/maps/pavlovskbay/tile.jpg",
			GalleryUrl: "https://www.realitymod.com/mapgallery/#!/pavlovskbay/",
		},
		"qwai1": {
			Name:       "Qwai River (2km)",
			ImageUrl:   "https://www.realitymod.com/mapgallery/images/maps/qwairiver/tile.jpg",
			GalleryUrl: "https://www.realitymod.com/mapgallery/#!/qwairiver/",
		},
		"ramiel": {
			Name:       "Ramiel (2km)",
			ImageUrl:   "https://www.realitymod.com/mapgallery/images/maps/ramiel/tile.jpg",
			GalleryUrl: "https://www.realitymod.com/mapgallery/#!/ramiel/",
		},
		"ras_el_masri_2": {
			Name:       "Ras el Masri (2km)",
			ImageUrl:   "https://www.realitymod.com/mapgallery/images/maps/raselmasri/tile.jpg",
			GalleryUrl: "https://www.realitymod.com/mapgallery/#!/raselmasri/",
		},
		"reichswald": {
			Name:       "Reichswald (1km)",
			ImageUrl:   "https://www.realitymod.com/mapgallery/images/maps/reichswald/tile.jpg",
			GalleryUrl: "https://www.realitymod.com/mapgallery/#!/reichswald/",
		},
		"route": {
			Name:       "Route E-106 (2km)",
			ImageUrl:   "https://www.realitymod.com/mapgallery/images/maps/routee-106/tile.jpg",
			GalleryUrl: "https://www.realitymod.com/mapgallery/#!/routee-106/",
		},
		"saaremaa": {
			Name:       "Saaremaa (4km)",
			ImageUrl:   "https://www.realitymod.com/mapgallery/images/maps/saaremaa/tile.jpg",
			GalleryUrl: "https://www.realitymod.com/mapgallery/#!/saaremaa/",
		},
		"sahel": {
			Name:       "Sahel (2km)",
			ImageUrl:   "https://www.realitymod.com/mapgallery/images/maps/sahel/tile.jpg",
			GalleryUrl: "https://www.realitymod.com/mapgallery/#!/sahel/",
		},
		"sbeneh_outskirts": {
			Name:       "Sbeneh Outskirts (2km)",
			ImageUrl:   "https://www.realitymod.com/mapgallery/images/maps/sbenehoutskirts/tile.jpg",
			GalleryUrl: "https://www.realitymod.com/mapgallery/#!/sbenehoutskirts/",
		},
		"shahadah": {
			Name:       "Shahadah (2km)",
			ImageUrl:   "https://www.realitymod.com/mapgallery/images/maps/shahadah/tile.jpg",
			GalleryUrl: "https://www.realitymod.com/mapgallery/#!/shahadah/",
		},
		"shijiavalley": {
			Name:       "Shijia Valley (4km)",
			ImageUrl:   "https://www.realitymod.com/mapgallery/images/maps/shijiavalley/tile.jpg",
			GalleryUrl: "https://www.realitymod.com/mapgallery/#!/shijiavalley/",
		},
		"silent_eagle": {
			Name:       "Silent Eagle (4km)",
			ImageUrl:   "https://www.realitymod.com/mapgallery/images/maps/silenteagle/tile.jpg",
			GalleryUrl: "https://www.realitymod.com/mapgallery/#!/silenteagle/",
		},
		"tad_sae": {
			Name:       "Tad Sae Offensive (1km)",
			ImageUrl:   "https://www.realitymod.com/mapgallery/images/maps/tadsaeoffensive/tile.jpg",
			GalleryUrl: "https://www.realitymod.com/mapgallery/#!/tadsaeoffensive/",
		},
		"test_airfield": {
			Name:       "Test Airfield (4km)",
			ImageUrl:   "none",
			GalleryUrl: "none",
		},
		"test_bootcamp": {
			Name:       "Test Bootcamp (2km)",
			ImageUrl:   "none",
			GalleryUrl: "none",
		},
		"the_falklands": {
			Name:       "The Falklands (8km)",
			ImageUrl:   "https://www.realitymod.com/mapgallery/images/maps/thefalklands/tile.jpg",
			GalleryUrl: "https://www.realitymod.com/mapgallery/#!/thefalklands/",
		},
		"ulyanovsk": {
			Name:       "Ulyanovsk (2km)",
			ImageUrl:   "https://www.realitymod.com/mapgallery/images/maps/ulyanovsk/tile.jpg",
			GalleryUrl: "https://www.realitymod.com/mapgallery/#!/ulyanovsk/",
		},
		"vadso_city": {
			Name:       "Vadso City (4km)",
			ImageUrl:   "https://www.realitymod.com/mapgallery/images/maps/vadsocity/tile.jpg",
			GalleryUrl: "https://www.realitymod.com/mapgallery/#!/vadsocity/",
		},
		"wanda_shan": {
			Name:       "Wanda Shan (4km)",
			ImageUrl:   "https://www.realitymod.com/mapgallery/images/maps/wandashan/tile.jpg",
			GalleryUrl: "https://www.realitymod.com/mapgallery/#!/wandashan/",
		},
		"xiangshan": {
			Name:       "Xiangshan (4km)",
			ImageUrl:   "https://www.realitymod.com/mapgallery/images/maps/xiangshan/tile.jpg",
			GalleryUrl: "https://www.realitymod.com/mapgallery/#!/xiangshan/",
		},
		"quan-seasonal": {
			Name:       "Quan seasonal (2km)",
			ImageUrl:   "https://www.realitymod.com/mapgallery/images/maps/quan-seasonal/tile.jpg",
			GalleryUrl: "https://www.realitymod.com/mapgallery/#!/quan-seasonal/",
		},
		"icebound": {
			Name:       "Icebound seasonal (4km)",
			ImageUrl:   "https://www.realitymod.com/mapgallery/images/maps/icebound-seasonal/tile.jpg",
			GalleryUrl: "https://www.realitymod.com/mapgallery/#!/icebound-seasonal/",
		},
		"yamalia": {
			Name:       "Yamalia (4km)",
			ImageUrl:   "https://www.realitymod.com/mapgallery/images/maps/yamalia/tile.jpg",
			GalleryUrl: "https://www.realitymod.com/mapgallery/#!/yamalia/",
		},
		"deagle5": {
			Name:       "Deagle5 (0km)",
			ImageUrl:   "https://www.realitymod.com/mapgallery/images/maps/deagle5/tile.jpg",
			GalleryUrl: "https://www.realitymod.com/mapgallery/#!/deagle5",
		},
		"moon": {
			Name:       "Moon (4km)",
			ImageUrl:   "https://www.realitymod.com/mapgallery/images/maps/moon/tile.jpg",
			GalleryUrl: "https://www.realitymod.com/mapgallery/#!/moon/",
		},
		"road_to_damascus": {
			Name:       "Road to Damascus (4km)",
			ImageUrl:   "https://www.realitymod.com/mapgallery/images/maps/roadtodamascus-beta/tile.jpg",
			GalleryUrl: "https://www.realitymod.com/mapgallery/#!/roadtodamascus-beta/",
		},
		"zakho": {
			Name:       "Zakho - BETA (4km)",
			ImageUrl:   "https://www.realitymod.com/mapgallery/images/maps/zakho-beta/tile.jpg",
			GalleryUrl: "https://www.realitymod.com/mapgallery/#!/zakho-beta/",
		},
		"shipment": {
			Name:       "Shipment (0km)",
			ImageUrl:   "https://www.realitymod.com/mapgallery/images/maps/shipment/tile.jpg",
			GalleryUrl: "https://www.realitymod.com/mapgallery/#!/shipment/",
		},
	},
}
