package discord

type layer struct {
	Name  string
	Short string
}

type gameMode struct {
	Name  string
	Color int
	Short string
}

var factionsLayersModes = struct {
	Factions  map[string]string
	Layers    map[int]layer
	GameModes map[string]gameMode
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
	Layers: map[int]layer{
		16: {
			Name:  "Infantry",
			Short: "INF",
		},
		32: {
			Name:  "Alternative",
			Short: "ALT",
		},
		64: {
			Name:  "Standard",
			Short: "STD",
		},
		128: {
			Name:  "Large",
			Short: "LRG",
		},
	},
	GameModes: map[string]gameMode{
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
}
