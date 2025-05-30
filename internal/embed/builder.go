package embed

import (
	"fmt"
	"sort"
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

	// Deduplicate and clean ports
	uniquePorts := b.deduplicatePorts(ports)

	// Sort ports by port number for better organization
	sort.Slice(uniquePorts, func(i, j int) bool {
		// Sort by protocol first (TCP before UDP), then by port number
		if uniquePorts[i].Protocol != uniquePorts[j].Protocol {
			return uniquePorts[i].Protocol == "TCP"
		}
		return uniquePorts[i].Port < uniquePorts[j].Port
	})

	// Group ports by protocol
	tcpPorts := []monitor.NetworkPort{}
	udpPorts := []monitor.NetworkPort{}

	for _, port := range uniquePorts {
		if port.Protocol == "TCP" {
			tcpPorts = append(tcpPorts, port)
		} else if port.Protocol == "UDP" {
			udpPorts = append(udpPorts, port)
		}
	}

	// Constants for Discord limits
	const maxPortsPerField = 10
	const maxFieldValueLength = 900
	const maxTotalFields = 20 // Leave room for summary

	fieldCount := 0

	// Add TCP ports section with pagination
	if len(tcpPorts) > 0 && fieldCount < maxTotalFields {
		tcpChunks := b.chunkPorts(tcpPorts, maxPortsPerField, maxFieldValueLength)
		for i, chunk := range tcpChunks {
			if fieldCount >= maxTotalFields {
				break
			}

			fieldName := fmt.Sprintf("🔵 TCP (%d total)", len(tcpPorts))
			if len(tcpChunks) > 1 {
				fieldName = fmt.Sprintf("🔵 TCP - Page %d/%d", i+1, len(tcpChunks))
			}

			embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
				Name:   fieldName,
				Value:  chunk,
				Inline: false,
			})
			fieldCount++
		}
	}

	// Add UDP ports section with pagination
	if len(udpPorts) > 0 && fieldCount < maxTotalFields {
		udpChunks := b.chunkPorts(udpPorts, maxPortsPerField, maxFieldValueLength)
		for i, chunk := range udpChunks {
			if fieldCount >= maxTotalFields {
				break
			}

			fieldName := fmt.Sprintf("🟡 UDP (%d total)", len(udpPorts))
			if len(udpChunks) > 1 {
				fieldName = fmt.Sprintf("🟡 UDP - Page %d/%d", i+1, len(udpChunks))
			}

			embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
				Name:   fieldName,
				Value:  chunk,
				Inline: false,
			})
			fieldCount++
		}
	}

	// Add summary with notable services
	summaryValue := fmt.Sprintf("**Total**: %d unique | **TCP**: %d | **UDP**: %d",
		len(uniquePorts), len(tcpPorts), len(udpPorts))

	// Add notable services
	notableServices := b.getNotableServices(uniquePorts)
	if notableServices != "" {
		summaryValue += fmt.Sprintf("\n\n**Services**: %s", notableServices)
	}

	// Show if truncated
	if fieldCount >= maxTotalFields && (len(tcpPorts) > maxPortsPerField*maxTotalFields/2 || len(udpPorts) > maxPortsPerField*maxTotalFields/2) {
		summaryValue += "\n\n⚠️ *Some ports truncated due to Discord limits*"
	}

	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
		Name:   "📊 Summary",
		Value:  summaryValue,
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

// deduplicatePorts removes duplicate entries based on protocol+address combination
func (b *Builder) deduplicatePorts(ports []monitor.NetworkPort) []monitor.NetworkPort {
	seen := make(map[string]monitor.NetworkPort)

	for _, port := range ports {
		key := fmt.Sprintf("%s:%s", port.Protocol, port.Address)

		// Keep the entry with the best process information
		if existing, exists := seen[key]; exists {
			// Prefer entries with actual process names over "Unknown Process"
			if (port.ProcessName != "" && port.ProcessName != "Unknown Process") &&
				(existing.ProcessName == "" || existing.ProcessName == "Unknown Process") {
				seen[key] = port
			}
		} else {
			seen[key] = port
		}
	}

	// Convert back to slice
	var unique []monitor.NetworkPort
	for _, port := range seen {
		unique = append(unique, port)
	}

	return unique
}

// chunkPorts splits ports into chunks that fit Discord field limits
func (b *Builder) chunkPorts(ports []monitor.NetworkPort, maxPorts int, maxLength int) []string {
	if len(ports) == 0 {
		return []string{"No ports found"}
	}

	var chunks []string
	var currentChunk strings.Builder
	currentCount := 0

	for _, port := range ports {
		// Format port entry compactly but readably
		processName := b.shortenProcessName(port.ProcessName)
		address := b.formatAddress(port.Address)

		portEntry := fmt.Sprintf("`%s` %s\n", address, processName)

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

	return chunks
}

// formatAddress creates clean, readable addresses
func (b *Builder) formatAddress(address string) string {
	// Replace verbose localhost representations
	replacements := map[string]string{
		"127.0.0.1:": "localhost:",
		"[::1]:":     "localhost:",
		"0.0.0.0:":   "*:",
		"[::]:":      "*:",
	}

	formatted := address
	for old, new := range replacements {
		formatted = strings.ReplaceAll(formatted, old, new)
	}

	// Handle very long addresses by showing just the port
	if len(formatted) > 25 {
		parts := strings.Split(formatted, ":")
		if len(parts) >= 2 {
			port := parts[len(parts)-1]
			return fmt.Sprintf("...:%s", port)
		}
	}

	return formatted
}

// shortenProcessName creates readable, compact process names
func (b *Builder) shortenProcessName(processName string) string {
	if processName == "" || processName == "Unknown Process" {
		return "Unknown"
	}

	// Clean up the name
	cleaned := processName

	// Remove PID information to save space
	if strings.Contains(cleaned, " (PID:") {
		parts := strings.Split(cleaned, " (PID:")
		if len(parts) > 0 {
			cleaned = parts[0]
		}
	}

	// Map common service names to shorter versions
	serviceAliases := map[string]string{
		"Docker Container Port": "Docker",
		"Docker Engine":         "Docker",
		"Container Runtime":     "Containerd",
		"Nginx Web Server":      "Nginx",
		"Apache Web Server":     "Apache",
		"Node.js Application":   "Node.js",
		"MySQL Database":        "MySQL",
		"PostgreSQL Database":   "PostgreSQL",
		"Redis Cache":           "Redis",
		"MongoDB Database":      "MongoDB",
		"SSH Server":            "SSH",
		"System Service":        "Systemd",
		"DNS Resolver":          "Resolved",
		"DHCP Client":           "DHCP",
		"Python Application":    "Python",
		"Java Application":      "Java",
	}

	// Check for exact matches
	if alias, exists := serviceAliases[cleaned]; exists {
		return alias
	}

	// Check for partial matches
	cleanedLower := strings.ToLower(cleaned)
	for full, alias := range serviceAliases {
		if strings.Contains(cleanedLower, strings.ToLower(full)) {
			return alias
		}
	}

	// Handle common patterns
	if strings.Contains(cleanedLower, "docker") {
		return "Docker"
	}
	if strings.Contains(cleanedLower, "nginx") {
		return "Nginx"
	}
	if strings.Contains(cleanedLower, "apache") || strings.Contains(cleanedLower, "httpd") {
		return "Apache"
	}

	// Intelligent truncation - preserve meaningful parts
	if len(cleaned) > 15 {
		words := strings.Fields(cleaned)
		if len(words) > 1 {
			// Keep first word if it's descriptive and not too long
			if len(words[0]) <= 12 && len(words[0]) > 2 {
				return strings.Title(words[0])
			}
		}
		// Fallback to simple truncation
		return cleaned[:12] + "..."
	}

	return cleaned
}

// getNotableServices identifies well-known services for the summary
func (b *Builder) getNotableServices(ports []monitor.NetworkPort) string {
	wellKnownPorts := map[string]string{
		"22":    "SSH",
		"80":    "Nginx",
		"443":   "HTTPS",
		"3306":  "MySQL",
		"5432":  "PostgreSQL",
		"6379":  "Redis",
		"27017": "MongoDB",
		"8080":  "HTTP-Alt",
		"8443":  "HTTPS-Alt",
		"9000":  "SonarQube",
		"5672":  "RabbitMQ",
		"15672": "RabbitMQ-UI",
		"1433":  "SQL Server",
		"9200":  "Elasticsearch",
		"9300":  "Elasticsearch",
	}

	var services []string
	seen := make(map[string]bool)

	for _, port := range ports {
		if service, exists := wellKnownPorts[port.Port]; exists && !seen[service] {
			services = append(services, fmt.Sprintf("%s:%s", service, port.Port))
			seen[service] = true

			// Limit to prevent summary from getting too long
			if len(services) >= 6 {
				break
			}
		}
	}

	if len(services) > 0 {
		return strings.Join(services, " • ")
	}

	return ""
}

// Helper functions for temperature monitoring
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
