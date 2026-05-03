package internal

import "github.com/emilekm/go-prbf2/prdemo"

// ExtractTickets reads the final ticket counts from a prdemo file.
// It returns the counts recorded immediately before the RoundEnd message.
func ExtractTickets(prDemoPath string) (team1, team2 int16, err error) {
	demo, err := prdemo.NewDemoReaderFromFile(prDemoPath)
	if err != nil {
		return 0, 0, err
	}

	for demo.Next() {
		msg, err := demo.GetMessage()
		if err != nil {
			return 0, 0, err
		}

		switch msg.Type {
		case prdemo.TicketsTeam1Type:
			var t prdemo.Tickets
			if err := msg.Decode(&t); err != nil {
				continue
			}
			if t.Tickets < 0 {
				t.Tickets = 0
			}
			team1 = t.Tickets
		case prdemo.TicketsTeam2Type:
			var t prdemo.Tickets
			if err := msg.Decode(&t); err != nil {
				continue
			}
			if t.Tickets < 0 {
				t.Tickets = 0
			}
			team2 = t.Tickets
		case prdemo.RoundEndType:
			return team1, team2, nil
		}
	}

	return team1, team2, nil
}
