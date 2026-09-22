package notify

import (
	"context"
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

type discordSession interface {
	ChannelMessageSendComplex(channelID string, msg *discordgo.MessageSend, opts ...discordgo.RequestOption) (*discordgo.Message, error)
	ChannelMessageEditComplex(m *discordgo.MessageEdit, options ...discordgo.RequestOption) (*discordgo.Message, error)
	ChannelMessageDelete(channelID, messageID string, options ...discordgo.RequestOption) (err error)
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

func (n *DiscordNotifier) PatchButtons(ctx context.Context, msgID string, round types.Round) error {
	return n.patchButtons(ctx, msgID, round)
}

func (n *DiscordNotifier) Notify(ctx context.Context, round types.Round) (string, error) {
	summary, err := n.prepareSummary(ctx, round)
	if err != nil {
		return "", err
	}
	return n.send(ctx, summary, "")
}

func (n *DiscordNotifier) NotifyReserved(ctx context.Context, msgID string, round types.Round) error {
	summary, err := n.prepareSummary(ctx, round)
	if err != nil {
		return err
	}
	_, err = n.send(ctx, summary, msgID)
	return err
}

func (n *DiscordNotifier) ReserveMessageID(ctx context.Context, timestamp time.Time) (string, error) {
	msg := &discordgo.MessageSend{
		Embeds: []*discordgo.MessageEmbed{
			&discordgo.MessageEmbed{
				Title:       "Round summary",
				Type:        discordgo.EmbedTypeRich,
				Description: fmt.Sprintf("Summary for round at %q is not available.", timestamp.Format(time.DateTime)),
			},
		},
	}

	resp, err := n.session.ChannelMessageSendComplex(n.channelID, msg, discordgo.WithContext(ctx))
	if err != nil {
		return "", err
	}

	return resp.ID, nil
}

func (n *DiscordNotifier) RemoveMessage(ctx context.Context, msgID string) error {
	return n.session.ChannelMessageDelete(n.channelID, msgID, discordgo.WithContext(ctx))
}

func (n *DiscordNotifier) prepareSummary(ctx context.Context, round types.Round) (*Summary, error) {
	summary := BuildSummary(ctx, n.logger, round)
	summary.RemoteRefs = n.refs(round)

	var err error
	summary.Image, err = createImage(summary)
	if err != nil {
		n.logger.LogAttrs(
			ctx, slog.LevelError,
			"discord_notifier: failed to generate image",
			applog.Error(err),
		)
	}

	return summary, nil
}

func (n *DiscordNotifier) patchButtons(ctx context.Context, msgID string, round types.Round) error {
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
		return err
	}

	return nil
}

// refs builds the remote links from the local filenames. They are known before
// the upload finishes, and stay dead until it does.
func (n *DiscordNotifier) refs(round types.Round) RemoteRefs {
	var refs RemoteRefs

	prDemo := round[types.ArtifactTypePRDemo]
	refs.PRDemo = Ref{
		Enabled: prDemo.Uploaded,
		URL:     fmt.Sprintf(n.remoteURLs.PRDemo, filepath.Base(prDemo.Path)),
	}
	refs.TrackerViewer = Ref{
		Enabled: prDemo.Uploaded,
		URL:     fmt.Sprintf(n.remoteURLs.TrackerViewer, filepath.Base(prDemo.Path)),
	}

	bf2Demo := round[types.ArtifactTypeBF2Demo]
	refs.BF2Demo = Ref{
		Enabled: bf2Demo.Uploaded,
		URL:     fmt.Sprintf(n.remoteURLs.BF2Demo, filepath.Base(bf2Demo.Path)),
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

type discordMsg struct {
	Embeds     []*discordgo.MessageEmbed    `json:"embeds"`
	Components []discordgo.MessageComponent `json:"components"`
	Files      []*discordgo.File            `json:"-"`
}

func (n *DiscordNotifier) send(ctx context.Context, summary *Summary, msgID string) (string, error) {
	msg := &discordMsg{
		Files: make([]*discordgo.File, 0),
	}

	row := linkButtons(summary.RemoteRefs)

	if summary.PRDemoPath != "" {
		prDemoFile, err := os.Open(summary.PRDemoPath)
		if err != nil {
			return "", err
		}
		defer prDemoFile.Close()
		msg.Files = append(msg.Files, &discordgo.File{
			Name:   filepath.Base(summary.PRDemoPath),
			Reader: prDemoFile,
		})
	}

	embed := n.buildEmbed(ctx, summary)

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

	if msgID != "" {
		_, err := n.session.ChannelMessageEditComplex(&discordgo.MessageEdit{
			ID:         msgID,
			Channel:    n.channelID,
			Components: &msg.Components,
			Files:      msg.Files,
			Embeds:     &msg.Embeds,
		}, discordgo.WithContext(ctx))
		return msgID, err
	}

	result, err := n.session.ChannelMessageSendComplex(n.channelID, &discordgo.MessageSend{
		Components: msg.Components,
		Files:      msg.Files,
		Embeds:     msg.Embeds,
	}, discordgo.WithContext(ctx))
	if err != nil {
		return "", err
	}
	return result.ID, nil
}

// buildEmbed renders whatever of the summary is known. MapName/MapMode/
// MapLayer/StartTime/EndTime may each be nil, so the title, color and
// description all degrade gracefully instead of assuming they're set.
func (n *DiscordNotifier) buildEmbed(ctx context.Context, summary *Summary) *discordgo.MessageEmbed {
	title := "Round summary"
	var color int

	if summary.MapName != nil {
		mapDetails, ok := levels[*summary.MapName]
		if !ok {
			mapDetails = level{Name: *summary.MapName}
		}
		title = fmt.Sprintf("%s (%d km)", mapDetails.Name, mapDetails.Size)
	}
	if summary.MapMode != nil {
		color = gameModes[*summary.MapMode].Color
	}

	embed := &discordgo.MessageEmbed{
		Title:       title,
		Type:        discordgo.EmbedTypeRich,
		Color:       color,
		Description: buildDescription(summary),
	}

	if summary.EndTime != nil {
		timestamp, err := time.Unix(*summary.EndTime, 0).MarshalText()
		if err != nil {
			n.logger.LogAttrs(
				ctx, slog.LevelWarn,
				"discord_notifier: failed to marshal endtime",
				slog.Int64("end_time", *summary.EndTime),
				applog.Error(err),
			)
		} else {
			embed.Timestamp = string(timestamp)
		}
	}

	return embed
}

func buildDescription(summary *Summary) string {
	var mode, layer string
	if summary.MapMode != nil {
		mode = gameModes[*summary.MapMode].Name
	}
	if summary.MapLayer != nil {
		layer = layers[*summary.MapLayer]
	}

	header := fmt.Sprintf("**_%s, %s_**", mode, layer)

	switch {
	case summary.StartTime != nil && summary.EndTime != nil:
		return fmt.Sprintf(
			"%s\n\nDuration: %d minutes\nStarted: <t:%d:R> | <t:%d:F>\nEnded: <t:%d:R> | <t:%d:F>",
			header,
			(*summary.EndTime-*summary.StartTime)/60,
			*summary.StartTime, *summary.StartTime,
			*summary.EndTime, *summary.EndTime,
		)
	case summary.StartTime != nil:
		return fmt.Sprintf(
			"%s\n\nStarted: <t:%d:R> | <t:%d:F>",
			header,
			*summary.StartTime, *summary.StartTime,
		)
	default:
		return header
	}
}
