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
	logger.Info("Creating new SystemMonitor instance...")
	logger.Info("Creating Discord session with bot token")

	session, err := discordgo.New("Bot " + cfg.Discord.Token)
	if err != nil {
		logger.Error("Failed to create Discord session:", err)
		return nil, fmt.Errorf("failed to create Discord session: %w", err)
	}
	logger.Info("Discord session created successfully")

	logger.Info("Initializing temperature monitor...")
	tempMonitor := monitor.NewTemperatureMonitor(cfg.Thresholds.Critical, cfg.Thresholds.Warning)

	logger.Info("Initializing network monitor...")
	netMonitor := monitor.NewNetworkMonitor()

	logger.Info("Initializing embed builder...")
	embedBuilder := embed.NewBuilder(cfg.Thresholds.Critical, cfg.Thresholds.Warning)

	sm := &SystemMonitor{
		discord:       session,
		config:        cfg,
		tempMonitor:   tempMonitor,
		netMonitor:    netMonitor,
		embedBuilder:  embedBuilder,
		alertChannels: make(map[string]bool),
	}

	logger.Info("SystemMonitor instance created successfully")
	return sm, nil
}

func (sm *SystemMonitor) Start() error {
	logger.Info("Starting SystemMonitor...")

	// Configure Discord session
	logger.Info("Adding Discord event handlers...")
	sm.discord.AddHandler(sm.onReady)
	sm.discord.AddHandler(sm.onInteraction)

	logger.Info("Setting Discord intents to Guilds")
	sm.discord.Identify.Intents = discordgo.IntentsGuilds

	// Start Discord connection
	logger.Info("Opening Discord connection...")
	if err := sm.discord.Open(); err != nil {
		logger.Error("Failed to open Discord connection:", err)
		return fmt.Errorf("failed to open Discord connection: %w", err)
	}
	logger.Info("Discord connection opened successfully")

	// Start background monitoring
	logger.Info("Starting background temperature monitoring goroutine...")
	go sm.startTemperatureMonitoring()

	logger.Info("SystemMonitor started successfully")
	return nil
}

func (sm *SystemMonitor) Stop() {
	logger.Info("Stopping SystemMonitor...")
	if sm.discord != nil {
		logger.Info("Closing Discord connection...")
		err := sm.discord.Close()
		if err != nil {
			logger.Error("Error closing Discord connection:", err)
		} else {
			logger.Info("Discord connection closed successfully")
		}
	}
	logger.Info("SystemMonitor stopped")
}

func (sm *SystemMonitor) startTemperatureMonitoring() {
	logger.Info("Temperature monitoring goroutine started")
	logger.Info("Creating ticker with interval:", sm.config.Monitor.Interval)

	ticker := time.NewTicker(sm.config.Monitor.Interval)
	defer func() {
		logger.Info("Stopping temperature monitoring ticker")
		ticker.Stop()
	}()

	logger.Info("Temperature monitoring started")

	for {
		select {
		case <-ticker.C:
			logger.Info("Temperature monitoring cycle started")

			sensors, err := sm.tempMonitor.GetSensors()
			if err != nil {
				logger.Error("Temperature monitoring failed:", err)
				continue
			}

			if len(sensors) == 0 {
				logger.Warn("No temperature sensors found in this cycle")
				continue
			}

			logger.Info("Processing", len(sensors), "temperature sensors")

			// Find highest temperature
			var maxSensor monitor.TemperatureSensor
			for _, sensor := range sensors {
				if sensor.Temperature > maxSensor.Temperature {
					maxSensor = sensor
				}
			}

			logger.Info("Highest temperature found:", maxSensor.Temperature, "°C from sensor:", maxSensor.Name)

			// Check for alert conditions
			if maxSensor.Status == monitor.TempCritical {
				logger.Warn("CRITICAL temperature detected:", maxSensor.Temperature, "°C")
				sm.sendTemperatureAlert("🚨 CRITICAL", sensors, "⚠️ **IMMEDIATE ACTION REQUIRED** - System temperature critical!")
			} else if maxSensor.Status == monitor.TempWarning {
				logger.Warn("WARNING temperature detected:", maxSensor.Temperature, "°C")
				sm.sendTemperatureAlert("⚠️ WARNING", sensors, "🔥 System temperature elevated - monitor closely")
			} else {
				logger.Info("All temperatures normal. Max temp:", maxSensor.Temperature, "°C")
			}
		}
	}
}

type AlertData struct {
	Level   string
	Sensors []monitor.TemperatureSensor
	Message string
}

func (sm *SystemMonitor) sendTemperatureAlert(level string, sensors []monitor.TemperatureSensor, message string) {
	logger.Info("Processing temperature alert:", level)

	// Check cooldown
	timeSinceLastAlert := time.Since(sm.lastAlert)
	if timeSinceLastAlert < sm.config.Monitor.AlertCooldown {
		logger.Info("Alert suppressed - cooldown active. Time since last:", timeSinceLastAlert, "Required:", sm.config.Monitor.AlertCooldown)
		return
	}

	if len(sm.alertChannels) == 0 {
		logger.Warn("No alert channels configured - alert not sent")
		return
	}

	logger.Info("Sending alerts to", len(sm.alertChannels), "configured channels")

	alertData := AlertData{
		Level:   level,
		Sensors: sensors,
		Message: message,
	}

	logger.Info("Building alert embed...")
	embed := sm.embedBuilder.BuildAlert(alertData.Level, alertData.Sensors, alertData.Message)

	// Send to all configured channels
	successCount := 0
	errorCount := 0
	for channelID := range sm.alertChannels {
		logger.Info("Sending alert to channel:", channelID)
		_, err := sm.discord.ChannelMessageSendEmbed(channelID, embed)
		if err != nil {
			logger.Error("Failed to send alert to channel", channelID, "error:", err)
			delete(sm.alertChannels, channelID) // Remove invalid channels
			errorCount++
		} else {
			logger.Info("Alert sent successfully to channel:", channelID)
			successCount++
		}
	}

	logger.Info("Alert sending complete. Success:", successCount, "Errors:", errorCount)
	sm.lastAlert = time.Now()
	logger.Info("Last alert time updated to:", sm.lastAlert)
}
