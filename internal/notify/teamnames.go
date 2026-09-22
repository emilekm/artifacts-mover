package notify

var teamNames = map[string]map[string]map[int][2]string{}

func sourceTeamNameTable(s *Summary) {
	if s.Team1Name != nil && s.Team2Name != nil {
		return
	}
	if s.MapName == nil || s.MapMode == nil || s.MapLayer == nil {
		return
	}

	byMode, ok := teamNames[*s.MapName]
	if !ok {
		return
	}
	byLayer, ok := byMode[*s.MapMode]
	if !ok {
		return
	}
	names, ok := byLayer[*s.MapLayer]
	if !ok {
		return
	}

	setIfNil(&s.Team1Name, names[0])
	setIfNil(&s.Team2Name, names[1])
}
