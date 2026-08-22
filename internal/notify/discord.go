package notify

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/emilekm/artifacts-mover/internal/config"
	applog "github.com/emilekm/artifacts-mover/internal/log"
	"github.com/emilekm/artifacts-mover/internal/types"
)

const (
	embedDescriptionFmt = `**_%s, %s_**

Duration: %d minutes
Started: <t:%d:R> | <t:%d:F>
Ended: <t:%d:R> | <t:%d:F>`
)

type discordSession interface {
	ChannelMessageSendComplex(channelID string, msg *discordgo.MessageSend, opts ...discordgo.RequestOption) (*discordgo.Message, error)
}

type DiscordNotifier struct {
	logger     *slog.Logger
	session    discordSession
	channelID  string
	remoteURLs config.RemoteURLs
}

func NewDiscordNotifier(logger *slog.Logger, session discordSession, conf config.Discord) *DiscordNotifier {
	return &DiscordNotifier{
		logger:     logger,
		session:    session,
		channelID:  conf.ChannelID,
		remoteURLs: conf.URLS,
	}
}

func (n *DiscordNotifier) Notify(ctx context.Context, msgID *string, round types.Round) (string, error) {
	summaryFile, err := os.Open(round[types.ArtifactTypeSummary].Path)
	if err != nil {
		return "", err
	}

	var jsonSummary JSONSummary
	err = json.NewDecoder(summaryFile).Decode(&jsonSummary)
	if err != nil {
		return "", err
	}

	summary := Summary{
		JSONSummary: jsonSummary,
	}

	summary.PRDemoPath = round[types.ArtifactTypePRDemo].Path
	summary.RemoteRefs = n.refs(round)

	if t1, t2, err := extractTickets(summary.PRDemoPath); err != nil {
		n.logger.LogAttrs(
			ctx, slog.LevelError,
			"discord_notifier: failed to extract tickets",
			applog.Path(summary.PRDemoPath),
			applog.Error(err),
		)
	} else {
		summary.Team1Tickets = int(t1)
		summary.Team2Tickets = int(t2)
	}

	prDemo, err := os.Open(summary.PRDemoPath)
	if err != nil {
		return "", err
	}
	defer prDemo.Close()

	summary.PRDemoFile = prDemo

	summary.Image, err = createImage(&summary)
	if err != nil {
		n.logger.LogAttrs(
			ctx, slog.LevelError,
			"discord_notifier: failed to generate image",
			applog.Error(err),
		)
	}

	return n.send(ctx, &summary)
}

// refs builds the remote links from the local filenames. They are known before
// the upload finishes, and stay dead until it does.
func (n *DiscordNotifier) refs(artifacts map[types.ArtifactType]types.Artifact) config.RemoteURLs {
	var refs config.RemoteURLs

	if prDemo, ok := artifacts[types.ArtifactTypePRDemo]; ok {
		refs.PRDemo = fmt.Sprintf(n.remoteURLs.PRDemo, filepath.Base(prDemo.Path))
		refs.TrackerViewer = fmt.Sprintf(n.remoteURLs.TrackerViewer, filepath.Base(prDemo.Path))
	}
	if bf2Demo, ok := artifacts[types.ArtifactTypeBF2Demo]; ok {
		refs.BF2Demo = fmt.Sprintf(n.remoteURLs.BF2Demo, bf2Demo.Path)
	}

	return refs
}

func linkButtons(refs config.RemoteURLs) discordgo.ActionsRow {
	row := discordgo.ActionsRow{}

	if refs.BF2Demo != "" {
		row.Components = append(row.Components, discordgo.Button{
			Label: "Download Battle Recorder",
			URL:   refs.BF2Demo,
			Style: discordgo.LinkButton,
		})
	}

	if refs.PRDemo != "" {
		row.Components = append(row.Components, discordgo.Button{
			Label: "Download Tracker",
			URL:   refs.PRDemo,
			Style: discordgo.LinkButton,
		})
	}

	if refs.TrackerViewer != "" {
		row.Components = append(row.Components, discordgo.Button{
			Label: "View Tracker",
			URL:   refs.TrackerViewer,
			Style: discordgo.LinkButton,
		})
	}

	return row
}

func (n *DiscordNotifier) send(ctx context.Context, summary *Summary) error {
	msg := &discordgo.MessageSend{
		Files: make([]*discordgo.File, 0),
	}

	row := linkButtons(summary.RemoteRefs)

	if summary.PRDemoFile != nil {
		msg.Files = append(msg.Files, &discordgo.File{
			Name:   filepath.Base(summary.PRDemoPath),
			Reader: summary.PRDemoFile,
		})
	}

	timestamp, err := time.Unix(summary.EndTime, 0).MarshalText()
	if err != nil {
		n.logger.LogAttrs(
			ctx, slog.LevelWarn,
			"discord_notifier: failed to marshal endtime",
			slog.Int64("end_time", summary.EndTime),
			applog.Error(err),
		)
	}

	mapDetails, ok := levels[summary.MapName]
	if !ok {
		mapDetails = level{
			Name: summary.MapName,
			Size: 0,
		}
	}

	embed := &discordgo.MessageEmbed{
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
	}

	if summary.Image != nil {
		imageFilename := "summary.png"
		msg.Files = append(msg.Files, &discordgo.File{
			Name:   imageFilename,
			Reader: summary.Image,
		})

		embed.Image = &discordgo.MessageEmbedImage{
			URL: "attachment://" + imageFilename,
		}
	}

	msg.Embeds = append(msg.Embeds, embed)

	msg.Components = []discordgo.MessageComponent{row}

	_, err = n.session.ChannelMessageSendComplex(n.channelID, msg, discordgo.WithContext(ctx))
	return err
}
