package notify

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/emilekm/artifacts-mover/internal/config"
	applog "github.com/emilekm/artifacts-mover/internal/log"
	"github.com/emilekm/artifacts-mover/internal/types"
)

const (
	msgIDPrefix         = "__reserved"
	embedDescriptionFmt = `**_%s, %s_**

Duration: %d minutes
Started: <t:%d:R> | <t:%d:F>
Ended: <t:%d:R> | <t:%d:F>`
)

type discordSession interface {
	ChannelMessageSendComplex(channelID string, msg *discordgo.MessageSend, opts ...discordgo.RequestOption) (*discordgo.Message, error)
	ChannelMessageEditComplex(m *discordgo.MessageEdit, options ...discordgo.RequestOption) (*discordgo.Message, error)
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
	if msgID != nil && !strings.HasPrefix(*msgID, msgIDPrefix) {
		return n.patchButtons(ctx, *msgID, round)
	}

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
		PRDemoPath:  round[types.ArtifactTypePRDemo].Path,
		RemoteRefs:  n.refs(round),
	}

	if t1, t2, err := extractTickets(summary.PRDemoPath); err != nil {
		n.logger.LogAttrs(
			ctx, slog.LevelWarn,
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
func (n *DiscordNotifier) ReserveMessageID(ctx context.Context, demoName string) (string, error) {
	msg := &discordgo.MessageSend{
		Embeds: []*discordgo.MessageEmbed{
			&discordgo.MessageEmbed{
				Title:       "Round summary",
				Type:        discordgo.EmbedTypeRich,
				Description: fmt.Sprintf("Summary for round %q is not available.", demoName),
			},
		},
	}

	resp, err := n.session.ChannelMessageSendComplex(n.channelID, msg, discordgo.WithContext(ctx))
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s-%s", msgIDPrefix, resp.ID), nil
}

func (n *DiscordNotifier) patchButtons(ctx context.Context, msgID string, round types.Round) (string, error) {
	components := []discordgo.MessageComponent{
		linkButtons(n.refs(round)),
	}

	msg := &discordgo.MessageEdit{
		ID:         msgID,
		Channel:    n.channelID,
		Components: &components,
	}

	_, err := n.session.ChannelMessageEditComplex(msg, discordgo.WithContext(ctx))
	if err != nil {
		return "", err
	}

	return msgID, nil
}

// refs builds the remote links from the local filenames. They are known before
// the upload finishes, and stay dead until it does.
func (n *DiscordNotifier) refs(round types.Round) RemoteRefs {
	var refs RemoteRefs

	if prDemo, ok := round[types.ArtifactTypePRDemo]; ok {
		enabled := false
		if prDemo.Uploaded {
			enabled = true
		}
		refs.PRDemo = Ref{
			Enabled: enabled,
			URL:     fmt.Sprintf(n.remoteURLs.PRDemo, filepath.Base(prDemo.Path)),
		}
		refs.TrackerViewer = Ref{
			Enabled: enabled,
			URL:     fmt.Sprintf(n.remoteURLs.TrackerViewer, filepath.Base(prDemo.Path)),
		}
	}
	if bf2Demo, ok := round[types.ArtifactTypeBF2Demo]; ok {
		enabled := false
		if bf2Demo.Uploaded {
			enabled = true
		}
		refs.BF2Demo = Ref{
			Enabled: enabled,
			URL:     fmt.Sprintf(n.remoteURLs.BF2Demo, bf2Demo.Path),
		}
	}

	return refs
}

var labels = [3]string{
	"Download Battle Recorder",
	"Download Tracker",
	"View Tracker",
}

func linkButtons(refs RemoteRefs) discordgo.ActionsRow {
	row := discordgo.ActionsRow{}

	for i, ref := range [3]Ref{
		refs.BF2Demo,
		refs.PRDemo,
		refs.TrackerViewer,
	} {
		row.Components = append(row.Components, discordgo.Button{
			Label:    labels[i],
			URL:      ref.URL,
			Style:    discordgo.LinkButton,
			Disabled: !ref.Enabled,
		})
	}

	return row
}

func (n *DiscordNotifier) send(ctx context.Context, summary *Summary) (string, error) {
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

	result, err := n.session.ChannelMessageSendComplex(n.channelID, msg, discordgo.WithContext(ctx))
	if err != nil {
		return "", err
	}
	return result.ID, nil
}
