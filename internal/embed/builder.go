package embed

import (
	"fmt"
	"strings"
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
		title = "All Network Connections"
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
		switch port.Protocol {
		case "TCP":
			tcpPorts = append(tcpPorts, port)
		case "UDP":
			udpPorts = append(udpPorts, port)
		}
	}

	const maxPortsPerField = 15
	const maxFieldValueLength = 900 // Leave buffer under 1024 limit

	if len(tcpPorts) > 0 {
		tcpChunks := b.chunkPorts(tcpPorts, maxPortsPerField, maxFieldValueLength)
		for i, chunk := range tcpChunks {
			fieldName := fmt.Sprintf("TCP Ports (%d)", len(tcpPorts))
			if len(tcpChunks) > 1 {
				fieldName = fmt.Sprintf("TCP Ports (%d/%d)", i+1, len(tcpChunks))
			}

			embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
				Name:   fieldName,
				Value:  chunk,
				Inline: false,
			})
		}
	}

	// Add UDP ports section with chunking
	if len(udpPorts) > 0 {
		udpChunks := b.chunkPorts(udpPorts, maxPortsPerField, maxFieldValueLength)
		for i, chunk := range udpChunks {
			fieldName := fmt.Sprintf("UDP Ports (%d)", len(udpPorts))
			if len(udpChunks) > 1 {
				fieldName = fmt.Sprintf("UDP Ports (%d/%d)", i+1, len(udpChunks))
			}

			embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
				Name:   fieldName,
				Value:  chunk,
				Inline: false,
			})
		}
	}

	// Add summary (always fits in one field)
	summaryValue := fmt.Sprintf("**Total**: %d | **TCP**: %d | **UDP**: %d",
		len(ports), len(tcpPorts), len(udpPorts))

	// Add top ports by common services if space allows
	if len(embed.Fields) < 8 { // Leave room for summary + top ports
		topPorts := b.getTopPorts(append(tcpPorts, udpPorts...))
		if topPorts != "" {
			summaryValue += fmt.Sprintf("\n\n**Notable Services**:\n%s", topPorts)
		}
	}

	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
		Name:   "📊 Summary",
		Value:  summaryValue,
		Inline: false,
	})

	return embed
}

func (b *Builder) chunkPorts(ports []monitor.NetworkPort, maxPorts int, maxLength int) []string {
	var chunks []string
	var currentChunk strings.Builder
	currentCount := 0

	for _, port := range ports {
		// Format port entry more compactly
		processName := b.shortenProcessName(port.ProcessName)
		portEntry := fmt.Sprintf("`%s` %s\n", b.shortenAddress(port.Address), processName)

		// Check if adding this entry would exceed limits
		if currentCount >= maxPorts || currentChunk.Len()+len(portEntry) > maxLength {
			if currentChunk.Len() > 0 {
				chunks = append(chunks, strings.TrimSpace(currentChunk.String()))
				currentChunk.Reset()
				currentCount = 0
			}
		}

		currentChunk.WriteString(portEntry)
		currentCount++
	}

	// Add final chunk if not empty
	if currentChunk.Len() > 0 {
		chunks = append(chunks, strings.TrimSpace(currentChunk.String()))
	}

	// If no chunks created, create a truncated version
	if len(chunks) == 0 && len(ports) > 0 {
		chunks = append(chunks, "Too many ports to display")
	}

	return chunks
}

func (b *Builder) shortenAddress(address string) string {
	// Replace common localhost representations
	address = strings.ReplaceAll(address, "127.0.0.1:", "localhost:")
	address = strings.ReplaceAll(address, "[::1]:", "localhost:")
	address = strings.ReplaceAll(address, "0.0.0.0:", "*:")
	address = strings.ReplaceAll(address, "[::]:", "*:")

	// Limit length for very long addresses
	if len(address) > 25 {
		parts := strings.Split(address, ":")
		if len(parts) >= 2 {
			port := parts[len(parts)-1]
			return fmt.Sprintf("...:%s", port)
		}
	}

	return address
}

// shortenProcessName creates compact process names
func (b *Builder) shortenProcessName(processName string) string {
	if processName == "" {
		return "?"
	}

	// Remove PID for space savings, keep just the main name
	if strings.Contains(processName, "(PID:") {
		parts := strings.Split(processName, " (PID:")
		if len(parts) > 0 {
			processName = parts[0]
		}
	}

	// Truncate very long process names
	if len(processName) > 20 {
		return processName[:17] + "..."
	}

	return processName
}

// getTopPorts identifies notable/common services for summary
func (b *Builder) getTopPorts(ports []monitor.NetworkPort) string {
	wellKnownPorts := map[string]string{
		"22":    "SSH",
		"80":    "HTTP",
		"443":   "HTTPS",
		"3306":  "MySQL",
		"5432":  "PostgreSQL",
		"6379":  "Redis",
		"27017": "MongoDB",
		"8080":  "HTTP Alt",
		"9000":  "SonarQube",
	}

	var notable []string
	seen := make(map[string]bool)

	for _, port := range ports {
		if service, exists := wellKnownPorts[port.Port]; exists && !seen[port.Port] {
			notable = append(notable, fmt.Sprintf("%s:%s", service, port.Port))
			seen[port.Port] = true
		}

		// Limit to prevent summary from getting too long
		if len(notable) >= 5 {
			break
		}
	}

	if len(notable) > 0 {
		return strings.Join(notable, " • ")
	}

	return ""
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
