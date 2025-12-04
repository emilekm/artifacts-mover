package discord

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/emilekm/artifacts-mover/internal"
	"github.com/emilekm/artifacts-mover/internal/config"
)

//go:generate go run ./assets/scripts/generate_assets.go

type discordSession interface {
	ChannelMessageSendComplex(channelID string, msg *discordgo.MessageSend, opts ...discordgo.RequestOption) (*discordgo.Message, error)
}

const (
	trackerType = "tracker"

	embedDescriptionFmt = `**_%s, %s_**

Duration: %d minutes
Started: <t:%d:R> | <t:%d:F>
Ended: <t:%d:R> | <t:%d:F>`
)

type player struct {
	Name  string
	Score int
}

type jsonSummary struct {
	MapName  string
	MapMode  string
	MapLayer int

	Team1Name    string
	Team2Name    string
	Team1Tickets int
	Team2Tickets int

	StartTime int64
	EndTime   int64
	Players   []player
}

type Client struct {
	session   discordSession
	channelID string
	typToURL  map[string]string
}

func New(session discordSession, channelID string, typToURL map[string]string) (*Client, error) {
	return &Client{
		session:   session,
		channelID: channelID,
		typToURL:  typToURL,
	}, nil
}

func (w *Client) Send(ctx context.Context, round internal.Round) error {
	msg := &discordgo.MessageSend{
		Files: make([]*discordgo.File, 0),
	}

	row := discordgo.ActionsRow{}

	for typ, artifact := range round {
		filename := filepath.Base(artifact.Path)
		switch typ {
		case config.ArtifactTypeBF2Demo:
			row.Components = append(row.Components, discordgo.Button{
				Label: "Download Battle Recorder",
				URL:   w.typToURL[typ.String()] + "/" + filename,
				Style: discordgo.LinkButton,
			})
		case config.ArtifactTypePRDemo:
			file, err := os.Open(artifact.Path)
			if err != nil {
				return err
			}

			defer file.Close()

			msg.Files = append(msg.Files, &discordgo.File{
				Name:   filename,
				Reader: file,
			})

			row.Components = append(row.Components, discordgo.Button{
				Label: "Download Tracker",
				URL:   w.typToURL[typ.String()] + "/" + filename,
				Style: discordgo.LinkButton,
			}, discordgo.Button{
				Label: "View Tracker",
				URL:   w.typToURL[trackerType] + filename,
				Style: discordgo.LinkButton,
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

			imgReader, err := createImage(&summary)
			if err != nil {
				return err
			}

			imageFilename := "summary.png"

			msg.Files = append(msg.Files, &discordgo.File{
				Name:   imageFilename,
				Reader: imgReader,
			})

			timestamp, err := time.Unix(summary.EndTime, 0).MarshalText()
			if err != nil {
				return err
			}

			mapDetails, ok := levels[summary.MapName]
			if !ok {
				mapDetails = level{
					Name: summary.MapName,
					Size: 0,
				}
			}

			msg.Embeds = append(msg.Embeds, &discordgo.MessageEmbed{
				Title: fmt.Sprintf("%s (%d km)", mapDetails.Name, mapDetails.Size),
				Color: gameModes[summary.MapMode].Color,
				Description: fmt.Sprintf(
					embedDescriptionFmt,
					gameModes[summary.MapMode].Name,
					layers[summary.MapLayer],
					(summary.EndTime-summary.StartTime)/60,
					summary.StartTime,
					summary.StartTime,
					summary.EndTime,
					summary.EndTime,
				),
				Timestamp: string(timestamp),
				Image: &discordgo.MessageEmbedImage{
					URL: "attachment://" + imageFilename,
				},
			})
		}
	}

	msg.Components = []discordgo.MessageComponent{row}

	_, err := w.session.ChannelMessageSendComplex(w.channelID, msg, discordgo.WithContext(ctx))
	return err
}
