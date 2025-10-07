package internal

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/emilekm/artifacts-mover/internal/config"
)

const (
	trackerType = "tracker"

	embedDescriptionFmt = `**_%s, %s_**

Duration: %d minutes
Started: <t:%d:R> | <t:%d:F>
Ended: <t:%d:R> | <t:%d:F>`
)

type jsonSummary struct {
	MapName   string
	MapMode   string
	MapLayer  string
	StartTime int64
	EndTime   int64
}

func (h *Handler) sendWebhook(round Round) error {
	if h.webhook == nil {
		return nil
	}

	msg := discord.WebhookMessageCreate{
		Files: make([]*discord.File, 0),
	}

	row := discord.ActionRowComponent{}

	for typ, artifact := range round {
		filename := filepath.Base(artifact.Path)
		switch typ {
		case config.ArtifactTypeBF2Demo:
			row = append(row, discord.ButtonComponent{
				Label: "Download Battle Recorder",
				Style: discord.ButtonStyleLink,
				URL:   h.typToURL[typ.String()] + "/" + filename,
			})
		case config.ArtifactTypePRDemo:
			file, err := os.Open(artifact.Path)
			if err != nil {
				return err
			}

			defer file.Close()

			msg.Files = []*discord.File{
				{
					Name:   filename,
					Reader: file,
				},
			}

			row = append(row, discord.ButtonComponent{
				Label: "Download Tracker",
				Style: discord.ButtonStyleLink,
				URL:   h.typToURL[typ.String()] + "/" + filename,
			}, discord.ButtonComponent{
				Label: "View Tracker",
				Style: discord.ButtonStyleLink,
				URL:   h.typToURL[trackerType] + filename,
			})
		case config.ArtifactTypeSummary:
			summaryContent, err := os.ReadFile(artifact.Path)
			if err != nil {
				return err
			}

			var summary jsonSummary
			if err := json.Unmarshal(summaryContent, &summary); err != nil {
				return err
			}

			timestamp := time.Unix(summary.EndTime, 0)

			msg.Embeds = []discord.Embed{
				{
					Title: factionsLayersModes.MapNames[summary.MapName].Name,
					Color: factionsLayersModes.GameModes[summary.MapMode].Color,
					Description: fmt.Sprintf(
						embedDescriptionFmt,
						factionsLayersModes.GameModes[summary.MapMode].Name,
						factionsLayersModes.Layers[summary.MapLayer].Name,
						(summary.EndTime-summary.StartTime)/60,
						summary.StartTime,
						summary.StartTime,
						summary.EndTime,
						summary.EndTime,
					),
					// TODO: add nice image
					// Image: &discord.EmbedResource{
					// 	URL: "attachment://" + imageFilename,
					// },
					Timestamp: &timestamp,
				},
			}

		}
	}

	_, err := h.webhook.CreateMessage(msg)
	return err
}
