package embed

import (
	"fmt"
	"system-monitor-bot/internal/monitor"
	"time"

	"github.com/bwmarrin/discordgo"
)

type Builder struct {
	criticalThreshold float64
	warningThreshold  float64
}

func NewBuilder(critical, warning float64) *Builder {
	return &Builder{
		criticalThreshold: critical,
		warningThreshold:  warning,
	}
}

func (b *Builder) BuildTemperature(sensors []monitor.TemperatureSensor) *discordgo.MessageEmbed {
	// Find maximum temperature and categorize
	maxTemp := 0.0
	hardwareTemps := make(map[string]float64)
	hardwareStatus := make(map[string]monitor.TempStatus)

	for _, sensor := range sensors {
		if sensor.Temperature > maxTemp {
			maxTemp = sensor.Temperature
		}

		// Track highest temperature per category
		if existing, exists := hardwareTemps[sensor.Category]; !exists || sensor.Temperature > existing {
			hardwareTemps[sensor.Category] = sensor.Temperature
			hardwareStatus[sensor.Category] = sensor.Status
		}
	}

	// Determine overall status
	overallStatus := b.getTemperatureStatus(maxTemp)
	embed := &discordgo.MessageEmbed{
		Title:     "🖥️ System Hardware Temperatures",
		Color:     b.getStatusColor(overallStatus),
		Timestamp: time.Now().Format(time.RFC3339),
		Footer: &discordgo.MessageEmbedFooter{
			Text: "System Hardware Monitor",
		},
	}

	// Build hardware overview
	hardwareSummary := ""
	categories := []string{
		monitor.CategoryCPU, monitor.CategoryGPU, monitor.CategoryMotherboard,
		monitor.CategoryChipset, monitor.CategoryWiFi, monitor.CategoryStorage,
		monitor.CategorySystem, monitor.CategoryOther,
	}

	for _, category := range categories {
		if temp, exists := hardwareTemps[category]; exists {
			status := hardwareStatus[category]
			icon := b.getStatusIcon(status)
			hardwareSummary += fmt.Sprintf("%s **%s**: %.1f°C  ", icon, category, temp)
		}
	}
	hardwareSummary += fmt.Sprintf("**Max**: %.1f°C", maxTemp)

	// Add hardware overview field
	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
		Name:   fmt.Sprintf("%s Hardware Overview", b.getStatusIcon(overallStatus)),
		Value:  hardwareSummary,
		Inline: false,
	})

	// Add individual sensor readings
	for _, sensor := range sensors {
		if len(embed.Fields) >= 25 { // Discord limit
			embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
				Name:   "...",
				Value:  fmt.Sprintf("And %d more sensors", len(sensors)-(len(embed.Fields)-1)),
				Inline: true,
			})
			break
		}

		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   fmt.Sprintf("%s %s", b.getStatusIcon(sensor.Status), sensor.Name),
			Value:  fmt.Sprintf("%.1f°C", sensor.Temperature),
			Inline: true,
		})
	}

	return embed
}

func (b *Builder) BuildPorts(ports []monitor.NetworkPort, showAll bool) *discordgo.MessageEmbed {
	title := "🔌 Network Ports"
	description := "Showing listening ports"
	if showAll {
		title = "🌐 All Network Connections"
		description = "Showing all active connections and listening ports"
	}

	embed := &discordgo.MessageEmbed{
		Title:       title,
		Description: description,
		Color:       0x3498db,
		Timestamp:   time.Now().Format(time.RFC3339),
		Footer: &discordgo.MessageEmbedFooter{
			Text: "System Network Monitor",
		},
	}

	// Group ports by protocol
	tcpPorts := []monitor.NetworkPort{}
	udpPorts := []monitor.NetworkPort{}

	for _, port := range ports {
		if port.Protocol == "TCP" {
			tcpPorts = append(tcpPorts, port)
		} else if port.Protocol == "UDP" {
			udpPorts = append(udpPorts, port)
		}
	}

	// Add TCP ports section
	if len(tcpPorts) > 0 {
		tcpValue := ""
		for _, port := range tcpPorts {
			processName := port.ProcessName
			if processName == "" {
				processName = "Unknown Process"
			}
			tcpValue += fmt.Sprintf("`%s` - %s\n", port.Address, processName)
		}

		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   fmt.Sprintf("🔵 TCP Ports (%d)", len(tcpPorts)),
			Value:  tcpValue,
			Inline: false,
		})
	}

	// Add UDP ports section
	if len(udpPorts) > 0 {
		udpValue := ""
		for _, port := range udpPorts {
			processName := port.ProcessName
			if processName == "" {
				processName = "Unknown Process"
			}
			udpValue += fmt.Sprintf("`%s` - %s\n", port.Address, processName)
		}

		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   fmt.Sprintf("🟡 UDP Ports (%d)", len(udpPorts)),
			Value:  udpValue,
			Inline: false,
		})
	}

	// Add summary
	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
		Name:   "📊 Summary",
		Value:  fmt.Sprintf("**Total Connections**: %d\n**TCP**: %d | **UDP**: %d", len(ports), len(tcpPorts), len(udpPorts)),
		Inline: false,
	})

	return embed
}

func (b *Builder) BuildAlert(level string, sensors []monitor.TemperatureSensor, message string) *discordgo.MessageEmbed {
	// Find max temperature for color
	maxTemp := 0.0
	for _, sensor := range sensors {
		if sensor.Temperature > maxTemp {
			maxTemp = sensor.Temperature
		}
	}

	embed := &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("%s Temperature Alert", level),
		Description: message,
		Color:       b.getStatusColor(b.getTemperatureStatus(maxTemp)),
		Timestamp:   time.Now().Format(time.RFC3339),
		Footer: &discordgo.MessageEmbedFooter{
			Text: "System Hardware Monitor - Alert",
		},
	}

	// Add critical and warning sensors
	alertSensors := ""
	normalSensors := ""
	sensorCount := 0

	for _, sensor := range sensors {
		if sensorCount >= 15 { // Limit for alert embeds
			break
		}

		icon := b.getStatusIcon(sensor.Status)
		sensorInfo := fmt.Sprintf("%s **%s**: %.1f°C\n", icon, sensor.Name, sensor.Temperature)

		if sensor.Status == monitor.TempCritical || sensor.Status == monitor.TempWarning {
			alertSensors += sensorInfo
		} else {
			normalSensors += sensorInfo
		}
		sensorCount++
	}

	// Add alert sensors first
	if alertSensors != "" {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "🔥 Critical/Warning Sensors",
			Value:  alertSensors,
			Inline: false,
		})
	}

	// Add normal sensors if space permits
	if normalSensors != "" && len(embed.Fields) < 3 {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "✅ Normal Sensors",
			Value:  normalSensors,
			Inline: false,
		})
	}

	// Add timestamp
	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
		Name:   "⏰ Alert Time",
		Value:  time.Now().Format("2006-01-02 15:04:05 MST"),
		Inline: true,
	})

	return embed
}

func (b *Builder) getTemperatureStatus(temp float64) monitor.TempStatus {
	if temp >= b.criticalThreshold {
		return monitor.TempCritical
	}
	if temp >= b.warningThreshold {
		return monitor.TempWarning
	}
	return monitor.TempNormal
}

func (b *Builder) getStatusIcon(status monitor.TempStatus) string {
	switch status {
	case monitor.TempCritical:
		return "🚨"
	case monitor.TempWarning:
		return "⚠️"
	default:
		return "✅"
	}
}

func (b *Builder) getStatusColor(status monitor.TempStatus) int {
	switch status {
	case monitor.TempCritical:
		return 0xff0000 // Red
	case monitor.TempWarning:
		return 0xff8800 // Orange
	default:
		return 0x00ff00 // Green
	}
}
