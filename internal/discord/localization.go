package discord

type gameMode struct {
	Name  string
	Color int
}

type level struct {
	Name string `json:"Name"`
	Key  string `json:"Key"`
	Size int    `json:"Size"`
}

var (
	layers = map[int]string{
		16:  "Infantry",
		32:  "Alternative",
		64:  "Standard",
		128: "Large",
	}

	gameModes = map[string]gameMode{
		"gpm_cq": {
			Name:  "Assault & Secure",
			Color: 0x4284F5,
		},
		"gpm_cnc": {
			Name:  "Command & Control",
			Color: 0xD1F542,
		},
		"gpm_coop": {
			Name:  "Co-Operation",
			Color: 0x42F596,
		},
		"gpm_insurgency": {
			Name:  "Insurgency",
			Color: 0xF54242,
		},
		"gpm_skirmish": {
			Name:  "Skirmish",
			Color: 0x858585,
		},
		"gpm_vehicles": {
			Name:  "Vehicle Warfare",
			Color: 0xF542DD,
		},
		"gpm_gungame": {
			Name:  "Gungame",
			Color: 0x04646A,
		},
	}
)
