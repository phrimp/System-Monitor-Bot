// Updated internal/bot/handlers.go with memory command support

package bot

import (
	"fmt"
	"system-monitor-bot/pkg/logger"

	"github.com/bwmarrin/discordgo"
)

func (sm *SystemMonitor) onReady(s *discordgo.Session, event *discordgo.Ready) {
	logger.Info("Discord connection established successfully")
	logger.Info("Bot ready! Logged in as:", s.State.User.Username)
	logger.Info("Bot ID:", s.State.User.ID)
	logger.Info("Connected to", len(s.State.Guilds), "guilds")

	// Set bot status
	logger.Info("Setting bot status to: System Monitor Active")
	err := s.UpdateGameStatus(0, "⚡ System Monitor Active")
	if err != nil {
		logger.Error("Failed to set bot status:", err)
	} else {
		logger.Info("Bot status set successfully")
	}

	// Register slash commands
	logger.Info("Starting slash command registration")
	sm.registerSlashCommands(s)
	logger.Info("Bot initialization complete")
}

func (sm *SystemMonitor) onInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	commandName := i.ApplicationCommandData().Name
	userName := i.Member.User.Username
	userID := i.Member.User.ID
	channelID := i.ChannelID
	guildID := i.GuildID

	logger.Info("Received command:", commandName, "from user", userName, "("+userID+")")
	logger.Info("Command executed in channel:", channelID, "guild:", guildID)

	switch commandName {
	case "temp":
		logger.Info("Processing temperature command for user:", userName)
		sm.handleTemperatureCommand(s, i)
	case "ports":
		logger.Info("Processing ports command for user:", userName)
		sm.handlePortsCommand(s, i)
	case "memory":
		logger.Info("Processing memory command for user:", userName)
		sm.handleMemoryCommand(s, i)
	case "alerts":
		logger.Info("Processing alerts command for user:", userName)
		sm.handleAlertsCommand(s, i)
	case "status":
		logger.Info("Processing status command for user:", userName)
		sm.handleStatusCommand(s, i)
	default:
		logger.Warn("Unknown command received:", commandName, "from user:", userName)
	}
}

func (sm *SystemMonitor) sendError(s *discordgo.Session, i *discordgo.InteractionCreate, title string, err error) {
	logger.Error("Sending error response to user:", i.Member.User.Username, "- Title:", title, "Error:", err)
	errorMsg := fmt.Sprintf("❌ **%s**\n```\n%v\n```", title, err)
	_, followupErr := s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
		Content: errorMsg,
	})
	if followupErr != nil {
		logger.Error("Failed to send error followup message:", followupErr)
	} else {
		logger.Info("Error message sent successfully to user:", i.Member.User.Username)
	}
}
