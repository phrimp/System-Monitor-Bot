package bot

import (
	"fmt"
	"system-monitor-bot/internal/config"
	"system-monitor-bot/internal/embed"
	"system-monitor-bot/internal/monitor"
	"system-monitor-bot/pkg/logger"
	"time"

	"github.com/bwmarrin/discordgo"
)

type SystemMonitor struct {
	discord       *discordgo.Session
	config        *config.Config
	tempMonitor   *monitor.TemperatureMonitor
	netMonitor    *monitor.NetworkMonitor
	embedBuilder  *embed.Builder
	alertChannels map[string]bool
	lastAlert     time.Time
}

func New(cfg *config.Config) (*SystemMonitor, error) {
	session, err := discordgo.New("Bot " + cfg.Discord.Token)
	if err != nil {
		return nil, fmt.Errorf("failed to create Discord session: %w", err)
	}

	return &SystemMonitor{
		discord:       session,
		config:        cfg,
		tempMonitor:   monitor.NewTemperatureMonitor(cfg.Thresholds.Critical, cfg.Thresholds.Warning),
		netMonitor:    monitor.NewNetworkMonitor(),
		embedBuilder:  embed.NewBuilder(cfg.Thresholds.Critical, cfg.Thresholds.Warning),
		alertChannels: make(map[string]bool),
	}, nil
}

func (sm *SystemMonitor) Start() error {
	// Configure Discord session
	sm.discord.AddHandler(sm.onReady)
	sm.discord.AddHandler(sm.onInteraction)
	sm.discord.Identify.Intents = discordgo.IntentsGuilds

	// Start Discord connection
	if err := sm.discord.Open(); err != nil {
		return fmt.Errorf("failed to open Discord connection: %w", err)
	}

	// Start background monitoring
	go sm.startTemperatureMonitoring()

	return nil
}

func (sm *SystemMonitor) Stop() {
	if sm.discord != nil {
		sm.discord.Close()
	}
}

func (sm *SystemMonitor) startTemperatureMonitoring() {
	ticker := time.NewTicker(sm.config.Monitor.Interval)
	defer ticker.Stop()

	logger.Info("🔄 Temperature monitoring started")

	for range ticker.C {
		sensors, err := sm.tempMonitor.GetSensors()
		if err != nil {
			logger.Error("❌ Temperature monitoring failed:", err)
			continue
		}

		if len(sensors) == 0 {
			continue
		}

		// Find highest temperature
		var maxSensor monitor.TemperatureSensor
		for _, sensor := range sensors {
			if sensor.Temperature > maxSensor.Temperature {
				maxSensor = sensor
			}
		}

		// Check for alert conditions
		if maxSensor.Status == monitor.TempCritical {
			sm.sendTemperatureAlert("🚨 CRITICAL", sensors, "⚠️ **IMMEDIATE ACTION REQUIRED** - System temperature critical!")
		} else if maxSensor.Status == monitor.TempWarning {
			sm.sendTemperatureAlert("⚠️ WARNING", sensors, "🔥 System temperature elevated - monitor closely")
		}
	}
}

type AlertData struct {
	Level   string
	Sensors []monitor.TemperatureSensor
	Message string
}

func (sm *SystemMonitor) sendTemperatureAlert(level string, sensors []monitor.TemperatureSensor, message string) {
	// Check cooldown
	if time.Since(sm.lastAlert) < sm.config.Monitor.AlertCooldown {
		logger.Info("🔇 Alert suppressed - cooldown active")
		return
	}

	if len(sm.alertChannels) == 0 {
		return
	}

	alertData := AlertData{
		Level:   level,
		Sensors: sensors,
		Message: message,
	}

	embed := sm.embedBuilder.BuildAlert(alertData.Level, alertData.Sensors, alertData.Message)

	// Send to all configured channels
	for channelID := range sm.alertChannels {
		_, err := sm.discord.ChannelMessageSendEmbed(channelID, embed)
		if err != nil {
			logger.Error("❌ Failed to send alert to channel", channelID, err)
			delete(sm.alertChannels, channelID) // Remove invalid channels
		}
	}

	sm.lastAlert = time.Now()
}
