package bot

import (
	"fmt"
	"system-monitor-bot/pkg/logger"
	"time"

	"github.com/bwmarrin/discordgo"
)

func (sm *SystemMonitor) registerSlashCommands(s *discordgo.Session) {
	commands := []*discordgo.ApplicationCommand{
		{
			Name:        "temp",
			Description: "Display current system temperatures",
		},
		{
			Name:        "ports",
			Description: "Show network ports and connections",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionBoolean,
					Name:        "all",
					Description: "Show all connections (default: listening only)",
					Required:    false,
				},
			},
		},
		{
			Name:        "alerts",
			Description: "Configure temperature alerts for this channel",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "action",
					Description: "Enable or disable temperature alerts",
					Required:    true,
					Choices: []*discordgo.ApplicationCommandOptionChoice{
						{Name: "enable", Value: "enable"},
						{Name: "disable", Value: "disable"},
					},
				},
			},
		},
		{
			Name:        "status",
			Description: "Show bot status and system information",
		},
	}

	guildID := sm.config.Discord.GuildID
	for _, cmd := range commands {
		_, err := s.ApplicationCommandCreate(s.State.User.ID, guildID, cmd)
		if err != nil {
			logger.Error("❌ Failed to register command", cmd.Name, err)
		} else {
			logger.Info("✅ Successfully registered command:", cmd.Name)
		}
	}
}

func (sm *SystemMonitor) handleTemperatureCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})

	sensors, err := sm.tempMonitor.GetSensors()
	if err != nil {
		sm.sendError(s, i, "Failed to read temperature sensors", err)
		return
	}

	if len(sensors) == 0 {
		sm.sendError(s, i, "No temperature sensors found", fmt.Errorf("ensure lm-sensors is installed and configured"))
		return
	}

	embed := sm.embedBuilder.BuildTemperature(sensors)
	s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
		Embeds: []*discordgo.MessageEmbed{embed},
	})
}

func (sm *SystemMonitor) handlePortsCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})

	showAll := false
	if len(i.ApplicationCommandData().Options) > 0 {
		showAll = i.ApplicationCommandData().Options[0].BoolValue()
	}

	ports, err := sm.netMonitor.GetPorts(showAll)
	if err != nil {
		sm.sendError(s, i, "Failed to read network ports", err)
		return
	}

	if len(ports) == 0 {
		s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
			Content: "🔍 No network ports found",
		})
		return
	}

	embed := sm.embedBuilder.BuildPorts(ports, showAll)
	s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
		Embeds: []*discordgo.MessageEmbed{embed},
	})
}

func (sm *SystemMonitor) handleAlertsCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	action := i.ApplicationCommandData().Options[0].StringValue()
	channelID := i.ChannelID

	var response string
	if action == "enable" {
		sm.alertChannels[channelID] = true
		response = fmt.Sprintf("✅ **Temperature alerts enabled** for this channel!\n\n"+
			"🚨 Critical alerts: %.1f°C and above\n"+
			"⚠️ Warning alerts: %.1f°C and above\n"+
			"🔄 Check interval: %v",
			sm.config.Thresholds.Critical, sm.config.Thresholds.Warning, sm.config.Monitor.Interval)
	} else {
		delete(sm.alertChannels, channelID)
		response = "❌ **Temperature alerts disabled** for this channel."
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{Content: response},
	})
}

func (sm *SystemMonitor) handleStatusCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	embed := &discordgo.MessageEmbed{
		Title:       "🖥️ System Monitor Status",
		Description: "Real-time server monitoring with lm-sensors and network analysis",
		Color:       0x00ff00,
		Timestamp:   time.Now().Format(time.RFC3339),
		Footer: &discordgo.MessageEmbedFooter{
			Text: "System Monitor Bot",
		},
	}

	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
		Name: "🌡️ Temperature Monitoring",
		Value: fmt.Sprintf("**Interval**: %v\n**Warning**: %.1f°C\n**Critical**: %.1f°C",
			sm.config.Monitor.Interval, sm.config.Thresholds.Warning, sm.config.Thresholds.Critical),
		Inline: true,
	})

	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
		Name:   "📢 Alert Channels",
		Value:  fmt.Sprintf("%d channels configured", len(sm.alertChannels)),
		Inline: true,
	})

	lastAlert := "Never"
	if !sm.lastAlert.IsZero() {
		lastAlert = fmt.Sprintf("<t:%d:R>", sm.lastAlert.Unix())
	}
	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
		Name:   "⏰ Last Alert",
		Value:  lastAlert,
		Inline: true,
	})

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
		},
	})
}
