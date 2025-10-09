package internal

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/webhook"
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
	MapLayer  int
	StartTime int64
	EndTime   int64
}

type DiscordWebhook struct {
	client   webhook.Client
	typToURL map[string]string
}

func NewDiscordWebhook(webhookURL string, typToURL map[string]string) (*DiscordWebhook, error) {
	wh, err := webhook.NewWithURL(webhookURL)
	if err != nil {
		return nil, err
	}

	return &DiscordWebhook{
		client:   wh,
		typToURL: typToURL,
	}, nil
}

func (h *DiscordWebhook) Send(round Round) error {
	builder := discord.NewWebhookMessageCreateBuilder()
	row := make(discord.ActionRowComponent, 0)

	for typ, artifact := range round {
		filename := filepath.Base(artifact.Path)
		switch typ {
		case config.ArtifactTypeBF2Demo:
			row = append(row, discord.NewLinkButton(
				"Download Battle Recorder",
				h.typToURL[typ.String()]+"/"+filename,
			))
		case config.ArtifactTypePRDemo:
			file, err := os.Open(artifact.Path)
			if err != nil {
				return err
			}

			defer file.Close()

			builder.AddFile(filename, "", file)

			row = append(row, discord.NewLinkButton(
				"Download Tracker",
				h.typToURL[typ.String()]+"/"+filename,
			), discord.NewLinkButton(
				"View Tracker",
				h.typToURL[trackerType]+filename,
			))
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

			builder.AddEmbeds(discord.Embed{
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
			})
		}
	}

	builder.SetContainerComponents(row)

	_, err := h.client.CreateMessage(builder.Build())
	return err
}
