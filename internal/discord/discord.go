package discord

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
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

type Client struct {
	session    discordSession
	channelID  string
	trackerURL string
}

func New(session discordSession, channelID string, typToURL map[string]string) (*Client, error) {
	return &Client{
		session:    session,
		channelID:  channelID,
		trackerURL: typToURL[trackerType],
	}, nil
}

func (w *Client) Send(ctx context.Context, summary internal.RoundSummary) error {
	msg := &discordgo.MessageSend{
		Files: make([]*discordgo.File, 0),
	}

	row := discordgo.ActionsRow{}
	buttons := make([]discordgo.Button, 0)

	if ref, ok := summary.RemoteRefs[config.ArtifactTypeBF2Demo]; ok {
		buttons = append(buttons, discordgo.Button{
			Label: "Download Battle Recorder",
			URL:   ref,
			Style: discordgo.LinkButton,
		})
	}

	if summary.PRDemoPath != "" {
		file, err := os.Open(summary.PRDemoPath)
		if err != nil {
			return err
		}
		defer file.Close()

		msg.Files = append(msg.Files, &discordgo.File{
			Name:   filepath.Base(summary.PRDemoPath),
			Reader: file,
		})

		if ref, ok := summary.RemoteRefs[config.ArtifactTypePRDemo]; ok {
			buttons = append(buttons, discordgo.Button{
				Label: "Download Tracker",
				URL:   ref,
				Style: discordgo.LinkButton,
			})
		}

		buttons = append(buttons, discordgo.Button{
			Label: "View Tracker",
			URL:   w.trackerURL + filepath.Base(summary.PRDemoPath),
			Style: discordgo.LinkButton,
		})
	}

	imgReader, err := createImage(&summary)
	if err != nil {
		slog.Warn("failed to create summary image", "err", err)
	} else {
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
			Type:  discordgo.EmbedTypeRich,
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

	slices.SortFunc(buttons, func(i, j discordgo.Button) int {
		if i.Label < j.Label {
			return 1
		} else if i.Label > j.Label {
			return -1
		}
		return 0
	})

	for _, button := range buttons {
		row.Components = append(row.Components, button)
	}

	msg.Components = []discordgo.MessageComponent{row}

	_, err = w.session.ChannelMessageSendComplex(w.channelID, msg, discordgo.WithContext(ctx))
	return err
}
