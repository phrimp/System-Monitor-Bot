package bot

import (
	"fmt"
	"system-monitor-bot/pkg/logger"
	"time"

	"github.com/bwmarrin/discordgo"
)

func (sm *SystemMonitor) registerSlashCommands(s *discordgo.Session) {
	logger.Info("Starting slash command registration...")

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
			Name:        "memory",
			Description: "Display top 10 processes by memory usage",
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

	logger.Info("Registering", len(commands), "slash commands")
	guildID := sm.config.Discord.GuildID
	logger.Info("Target guild ID:", guildID)

	successCount := 0
	errorCount := 0

	for _, cmd := range commands {
		logger.Info("Registering command:", cmd.Name)
		_, err := s.ApplicationCommandCreate(s.State.User.ID, guildID, cmd)
		if err != nil {
			logger.Error("Failed to register command", cmd.Name, "error:", err)
			errorCount++
		} else {
			logger.Info("Successfully registered command:", cmd.Name)
			successCount++
		}
	}

	logger.Info("Command registration complete. Success:", successCount, "Errors:", errorCount)
}

func (sm *SystemMonitor) handleTemperatureCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	logger.Info("Handling temperature command for user:", i.Member.User.Username)

	logger.Info("Sending deferred response...")
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})
	if err != nil {
		logger.Error("Failed to send deferred response:", err)
		return
	}

	logger.Info("Getting temperature sensors...")
	sensors, err := sm.tempMonitor.GetSensors()
	if err != nil {
		logger.Error("Failed to get temperature sensors:", err)
		sm.sendError(s, i, "Failed to read temperature sensors", err)
		return
	}

	if len(sensors) == 0 {
		logger.Warn("No temperature sensors found")
		sm.sendError(s, i, "No temperature sensors found", fmt.Errorf("ensure lm-sensors is installed and configured"))
		return
	}

	logger.Info("Building temperature embed for", len(sensors), "sensors")
	embed := sm.embedBuilder.BuildTemperature(sensors)

	logger.Info("Sending temperature response...")
	_, err = s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
		Embeds: []*discordgo.MessageEmbed{embed},
	})
	if err != nil {
		logger.Error("Failed to send temperature response:", err)
	} else {
		logger.Info("Temperature command completed successfully for user:", i.Member.User.Username)
	}
}

func (sm *SystemMonitor) handlePortsCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	logger.Info("Handling ports command for user:", i.Member.User.Username)

	logger.Info("Sending deferred response...")
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})
	if err != nil {
		logger.Error("Failed to send deferred response:", err)
		return
	}

	showAll := false
	if len(i.ApplicationCommandData().Options) > 0 {
		showAll = i.ApplicationCommandData().Options[0].BoolValue()
		logger.Info("Show all connections parameter:", showAll)
	}

	logger.Info("Getting network ports with showAll:", showAll)
	ports, err := sm.netMonitor.GetPorts(showAll)
	if err != nil {
		logger.Error("Failed to get network ports:", err)
		sm.sendError(s, i, "Failed to read network ports", err)
		return
	}

	if len(ports) == 0 {
		logger.Info("No network ports found")
		_, err = s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
			Content: "🔍 No network ports found",
		})
		if err != nil {
			logger.Error("Failed to send no ports response:", err)
		}
		return
	}

	logger.Info("Building ports embed for", len(ports), "ports")
	embed := sm.embedBuilder.BuildPorts(ports, showAll)

	logger.Info("Sending ports response...")
	_, err = s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
		Embeds: []*discordgo.MessageEmbed{embed},
	})
	if err != nil {
		logger.Error("Failed to send ports response:", err)
	} else {
		logger.Info("Ports command completed successfully for user:", i.Member.User.Username)
	}
}

func (sm *SystemMonitor) handleMemoryCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	logger.Info("Handling memory command for user:", i.Member.User.Username)

	logger.Info("Sending deferred response...")
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})
	if err != nil {
		logger.Error("Failed to send deferred response:", err)
		return
	}

	logger.Info("Getting memory usage data...")
	processes, err := sm.memMonitor.GetTopProcesses()
	if err != nil {
		logger.Error("Failed to get memory usage:", err)
		sm.sendError(s, i, "Failed to read memory usage", err)
		return
	}

	if len(processes) == 0 {
		logger.Warn("No processes found")
		_, err = s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
			Content: "🔍 No processes found with memory usage",
		})
		if err != nil {
			logger.Error("Failed to send no processes response:", err)
		}
		return
	}

	logger.Info("Building memory embed for", len(processes), "processes")
	embed := sm.embedBuilder.BuildMemory(processes)

	logger.Info("Sending memory response...")
	_, err = s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
		Embeds: []*discordgo.MessageEmbed{embed},
	})
	if err != nil {
		logger.Error("Failed to send memory response:", err)
	} else {
		logger.Info("Memory command completed successfully for user:", i.Member.User.Username)
	}
}

func (sm *SystemMonitor) handleAlertsCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	logger.Info("Handling alerts command for user:", i.Member.User.Username)

	action := i.ApplicationCommandData().Options[0].StringValue()
	channelID := i.ChannelID

	logger.Info("Alert action:", action, "for channel:", channelID)

	var response string
	if action == "enable" {
		logger.Info("Enabling alerts for channel:", channelID)
		sm.alertChannels[channelID] = true
		response = fmt.Sprintf("✅ **Temperature alerts enabled** for this channel!\n\n"+
			"🚨 Critical alerts: %.1f°C and above\n"+
			"⚠️ Warning alerts: %.1f°C and above\n"+
			"🔄 Check interval: %v",
			sm.config.Thresholds.Critical, sm.config.Thresholds.Warning, sm.config.Monitor.Interval)
		logger.Info("Alerts enabled successfully. Total alert channels:", len(sm.alertChannels))
	} else {
		logger.Info("Disabling alerts for channel:", channelID)
		delete(sm.alertChannels, channelID)
		response = "❌ **Temperature alerts disabled** for this channel."
		logger.Info("Alerts disabled successfully. Total alert channels:", len(sm.alertChannels))
	}

	logger.Info("Sending alerts command response...")
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{Content: response},
	})
	if err != nil {
		logger.Error("Failed to send alerts response:", err)
	} else {
		logger.Info("Alerts command completed successfully for user:", i.Member.User.Username)
	}
}

func (sm *SystemMonitor) handleStatusCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	logger.Info("Handling status command for user:", i.Member.User.Username)

	logger.Info("Building status embed...")
	embed := &discordgo.MessageEmbed{
		Title:       "🖥️ System Monitor Status",
		Description: "Real-time server monitoring with lm-sensors, network analysis, and memory tracking",
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
		Name:   "💾 Memory Monitoring",
		Value:  "**Interval**: 30s\n**Top Processes**: 10\n**Auto Updates**: Enabled",
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

	// Add current memory status if available
	if len(sm.lastMemoryData) > 0 {
		topProcess := sm.lastMemoryData[0]
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "🔥 Top Memory Process",
			Value:  fmt.Sprintf("**%s**\n%.1f%% memory", topProcess.Command, topProcess.MemoryPercent),
			Inline: true,
		})
	}

	logger.Info("Sending status response...")
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
		},
	})
	if err != nil {
		logger.Error("Failed to send status response:", err)
	} else {
		logger.Info("Status command completed successfully for user:", i.Member.User.Username)
	}
}
