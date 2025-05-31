package embed

import (
	"fmt"
	"sort"
	"strings"
	"system-monitor-bot/internal/monitor"
	"system-monitor-bot/pkg/logger"
	"time"

	"github.com/bwmarrin/discordgo"
)

type Builder struct {
	criticalThreshold float64
	warningThreshold  float64
}

func NewBuilder(critical, warning float64) *Builder {
	logger.Info("Creating new embed Builder with thresholds - Critical:", critical, "Warning:", warning)
	return &Builder{
		criticalThreshold: critical,
		warningThreshold:  warning,
	}
}

func (b *Builder) BuildTemperature(sensors []monitor.TemperatureSensor) *discordgo.MessageEmbed {
	logger.Info("Building temperature embed for", len(sensors), "sensors")

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

	logger.Info("Maximum temperature found:", maxTemp, "°C")
	logger.Info("Hardware categories found:", len(hardwareTemps))

	// Determine overall status
	overallStatus := b.getTemperatureStatus(maxTemp)
	logger.Info("Overall temperature status:", overallStatus)

	embed := &discordgo.MessageEmbed{
		Title:     "🖥️ System Hardware Temperatures",
		Color:     b.getStatusColor(overallStatus),
		Timestamp: time.Now().Format(time.RFC3339),
		Footer: &discordgo.MessageEmbedFooter{
			Text: "System Hardware Monitor",
		},
	}

	// Build hardware overview
	logger.Info("Building hardware overview...")
	hardwareSummary := ""
	categories := []string{
		monitor.CategoryCPU, monitor.CategoryGPU, monitor.CategoryMotherboard,
		monitor.CategoryChipset, monitor.CategoryWiFi, monitor.CategoryStorage,
		monitor.CategorySystem, monitor.CategoryOther,
	}

	categoriesFound := 0
	for _, category := range categories {
		if temp, exists := hardwareTemps[category]; exists {
			status := hardwareStatus[category]
			icon := b.getStatusIcon(status)
			hardwareSummary += fmt.Sprintf("%s **%s**: %.1f°C  ", icon, category, temp)
			categoriesFound++
		}
	}
	hardwareSummary += fmt.Sprintf("**Max**: %.1f°C", maxTemp)

	logger.Info("Hardware overview includes", categoriesFound, "categories")

	// Add hardware overview field
	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
		Name:   fmt.Sprintf("%s Hardware Overview", b.getStatusIcon(overallStatus)),
		Value:  hardwareSummary,
		Inline: false,
	})

	// Add individual sensor readings
	logger.Info("Adding individual sensor fields...")
	sensorsAdded := 0
	for _, sensor := range sensors {
		if len(embed.Fields) >= 25 { // Discord limit
			logger.Info("Reached Discord field limit (25), adding truncation notice")
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
		sensorsAdded++
	}

	logger.Info("Temperature embed built successfully with", sensorsAdded, "sensor fields")
	return embed
}

func (b *Builder) BuildPorts(ports []monitor.NetworkPort, showAll bool) *discordgo.MessageEmbed {
	logger.Info("Building ports embed for", len(ports), "ports, showAll:", showAll)

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

	// Debug: Show original count
	originalCount := len(ports)
	logger.Info("Original port count:", originalCount)

	// Deduplicate and clean ports
	logger.Info("Deduplicating ports...")
	uniquePorts := b.deduplicatePorts(ports)
	logger.Info("After deduplication:", len(uniquePorts), "ports")

	// Debug info in description if we removed duplicates
	if len(uniquePorts) != originalCount {
		embed.Description += fmt.Sprintf(" (removed %d duplicates)", originalCount-len(uniquePorts))
	}

	// Group ports by protocol
	logger.Info("Grouping ports by protocol...")
	tcpPorts := []monitor.NetworkPort{}
	udpPorts := []monitor.NetworkPort{}

	for _, port := range uniquePorts {
		switch strings.ToUpper(port.Protocol) {
		case "TCP":
			tcpPorts = append(tcpPorts, port)
		case "UDP":
			udpPorts = append(udpPorts, port)
		}
	}

	logger.Info("Protocol distribution - TCP:", len(tcpPorts), "UDP:", len(udpPorts))

	// Constants for Discord limits - adjusted for full addresses
	const maxPortsPerField = 6       // Reduced since addresses will be longer
	const maxFieldValueLength = 1000 // Slightly increased for full addresses
	const maxTotalFields = 12        // Reduced to prevent hitting overall embed limits

	fieldCount := 0

	// Add TCP ports section with pagination
	if len(tcpPorts) > 0 && fieldCount < maxTotalFields {
		logger.Info("Processing TCP ports...")
		tcpChunks := b.chunkPorts(tcpPorts, maxPortsPerField, maxFieldValueLength)
		logger.Info("TCP ports split into", len(tcpChunks), "chunks")

		for i, chunk := range tcpChunks {
			if fieldCount >= maxTotalFields {
				logger.Info("Reached field limit, adding TCP truncation notice")
				embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
					Name:   "⚠️ Truncated",
					Value:  fmt.Sprintf("Showing %d/%d TCP ports (Discord limit)", i, len(tcpChunks)),
					Inline: false,
				})
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
		logger.Info("Processing UDP ports...")
		udpChunks := b.chunkPorts(udpPorts, maxPortsPerField, maxFieldValueLength)
		logger.Info("UDP ports split into", len(udpChunks), "chunks")

		for i, chunk := range udpChunks {
			if fieldCount >= maxTotalFields {
				logger.Info("Reached field limit, adding UDP truncation notice")
				embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
					Name:   "⚠️ Truncated",
					Value:  fmt.Sprintf("Showing %d/%d UDP ports (Discord limit)", i, len(udpChunks)),
					Inline: false,
				})
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
	logger.Info("Building summary section...")
	summaryValue := fmt.Sprintf("**Original**: %d | **Unique**: %d | **TCP**: %d | **UDP**: %d",
		originalCount, len(uniquePorts), len(tcpPorts), len(udpPorts))

	// Add notable services
	notableServices := b.getNotableServices(uniquePorts)
	if notableServices != "" {
		summaryValue += fmt.Sprintf("\n\n**Services**: %s", notableServices)
		logger.Info("Notable services found:", notableServices)
	}

	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
		Name:   "📊 Summary",
		Value:  summaryValue,
		Inline: false,
	})

	logger.Info("Ports embed built successfully with", fieldCount+1, "total fields")
	return embed
}

func (b *Builder) BuildAlert(level string, sensors []monitor.TemperatureSensor, message string) *discordgo.MessageEmbed {
	logger.Info("Building alert embed - Level:", level, "Sensors:", len(sensors))

	// Find max temperature for color
	maxTemp := 0.0
	for _, sensor := range sensors {
		if sensor.Temperature > maxTemp {
			maxTemp = sensor.Temperature
		}
	}
	logger.Info("Alert max temperature:", maxTemp, "°C")

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
	logger.Info("Processing sensors for alert...")
	alertSensors := ""
	normalSensors := ""
	sensorCount := 0
	alertSensorCount := 0
	normalSensorCount := 0

	for _, sensor := range sensors {
		if sensorCount >= 15 { // Limit for alert embeds
			logger.Info("Reached sensor limit for alert embed")
			break
		}

		icon := b.getStatusIcon(sensor.Status)
		sensorInfo := fmt.Sprintf("%s **%s**: %.1f°C\n", icon, sensor.Name, sensor.Temperature)

		if sensor.Status == monitor.TempCritical || sensor.Status == monitor.TempWarning {
			alertSensors += sensorInfo
			alertSensorCount++
		} else {
			normalSensors += sensorInfo
			normalSensorCount++
		}
		sensorCount++
	}

	logger.Info("Alert sensors breakdown - Alert/Warning:", alertSensorCount, "Normal:", normalSensorCount)

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

	logger.Info("Alert embed built successfully with", len(embed.Fields), "fields")
	return embed
}

// deduplicatePorts removes duplicate entries based on protocol+address combination
func (b *Builder) deduplicatePorts(ports []monitor.NetworkPort) []monitor.NetworkPort {
	logger.Info("Starting port deduplication for", len(ports), "ports")

	if len(ports) == 0 {
		return ports
	}

	// Use a more robust key that includes port number specifically
	seen := make(map[string]monitor.NetworkPort)
	duplicatesFound := 0

	for _, port := range ports {
		// Create a normalized key using protocol, address, and port
		normalizedAddr := strings.TrimSpace(port.Address)
		normalizedProto := strings.ToUpper(strings.TrimSpace(port.Protocol))
		normalizedPort := strings.TrimSpace(port.Port)

		key := fmt.Sprintf("%s|%s|%s", normalizedProto, normalizedAddr, normalizedPort)

		// Only keep the first occurrence - simpler logic
		if _, exists := seen[key]; !exists {
			seen[key] = port
		} else {
			duplicatesFound++
		}
	}

	logger.Info("Deduplication found", duplicatesFound, "duplicates")

	// Convert back to slice and sort for consistent output
	var unique []monitor.NetworkPort
	for _, port := range seen {
		unique = append(unique, port)
	}

	// Sort by protocol first, then by port number
	logger.Info("Sorting", len(unique), "unique ports")
	sort.Slice(unique, func(i, j int) bool {
		if unique[i].Protocol != unique[j].Protocol {
			return unique[i].Protocol == "TCP" // TCP before UDP
		}

		// Convert port strings to integers for proper numeric sorting
		portI := b.parsePortNumber(unique[i].Port)
		portJ := b.parsePortNumber(unique[j].Port)
		return portI < portJ
	})

	logger.Info("Port deduplication complete. Unique ports:", len(unique))
	return unique
}

// parsePortNumber safely converts port string to int for sorting
func (b *Builder) parsePortNumber(portStr string) int {
	// Handle cases where port might have extra characters
	portStr = strings.TrimSpace(portStr)

	// Try to parse the port number
	var portNum int
	if _, err := fmt.Sscanf(portStr, "%d", &portNum); err != nil {
		logger.Info("Could not parse port number:", portStr, "- placing at end")
		return 99999 // Put unparseable ports at the end
	}

	return portNum
}

// chunkPorts splits ports into chunks that fit Discord field limits
func (b *Builder) chunkPorts(ports []monitor.NetworkPort, maxPorts int, maxLength int) []string {
	logger.Info("Chunking", len(ports), "ports with maxPorts:", maxPorts, "maxLength:", maxLength)

	if len(ports) == 0 {
		return []string{"No ports found"}
	}

	var chunks []string
	var currentChunk strings.Builder
	currentCount := 0
	chunkNumber := 0

	for i, port := range ports {
		// Format port entry with full address and process name
		processName := b.shortenProcessName(port.ProcessName)
		address := b.formatAddress(port.Address)

		// Use a more compact format to fit full addresses
		portEntry := fmt.Sprintf("`%s` %s\n", address, processName)

		// Check if adding this entry would exceed limits
		// Be more flexible with length to accommodate full addresses
		if currentCount >= maxPorts || currentChunk.Len()+len(portEntry) > (maxLength+200) {
			if currentChunk.Len() > 0 {
				chunks = append(chunks, strings.TrimSpace(currentChunk.String()))
				chunkNumber++
				logger.Info("Created chunk", chunkNumber, "with", currentCount, "ports, length:", currentChunk.Len())
				currentChunk.Reset()
				currentCount = 0
			}
		}

		currentChunk.WriteString(portEntry)
		currentCount++

		if i == len(ports)-1 { // Last port
			logger.Info("Processing final port in chunk")
		}
	}

	// Add final chunk if not empty
	if currentChunk.Len() > 0 {
		chunks = append(chunks, strings.TrimSpace(currentChunk.String()))
		chunkNumber++
		logger.Info("Created final chunk", chunkNumber, "with", currentCount, "ports")
	}

	logger.Info("Port chunking complete. Created", len(chunks), "chunks")
	return chunks
}

// formatAddress shows the complete, unmodified address
func (b *Builder) formatAddress(address string) string {
	// Return the full address exactly as it appears in the system
	// No replacements or modifications - show everything
	return strings.TrimSpace(address)
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
	logger.Info("Identifying notable services from", len(ports), "ports")

	wellKnownPorts := map[string]string{
		"22":    "SSH",
		"80":    "HTTP",
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
	foundServices := 0

	for _, port := range ports {
		if service, exists := wellKnownPorts[port.Port]; exists && !seen[service] {
			services = append(services, fmt.Sprintf("%s:%s", service, port.Port))
			seen[service] = true
			foundServices++
			logger.Info("Found notable service:", service, "on port", port.Port)

			// Limit to prevent summary from getting too long
			if len(services) >= 6 {
				logger.Info("Reached notable services limit (6)")
				break
			}
		}
	}

	logger.Info("Notable services identification complete. Found:", foundServices)

	if len(services) > 0 {
		result := strings.Join(services, " • ")
		logger.Info("Notable services string:", result)
		return result
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

func (b *Builder) BuildMemory(processes []monitor.ProcessMemory) *discordgo.MessageEmbed {
	logger.Info("Building memory embed for", len(processes), "processes")

	embed := &discordgo.MessageEmbed{
		Title:     "💾 Top Memory Usage",
		Color:     0x9b59b6, // Purple color for memory
		Timestamp: time.Now().Format(time.RFC3339),
		Footer: &discordgo.MessageEmbedFooter{
			Text: "System Memory Monitor",
		},
	}

	if len(processes) == 0 {
		embed.Description = "No processes found"
		logger.Info("No processes to display in memory embed")
		return embed
	}

	totalMemory := 0.0
	for _, process := range processes {
		totalMemory += process.MemoryPercent
	}

	embed.Description = fmt.Sprintf("Top %d processes consuming **%.1f%%** total memory", len(processes), totalMemory)
	logger.Info("Memory embed description set with total:", totalMemory, "%")

	// Add individual process fields
	logger.Info("Adding individual process fields...")
	for i, process := range processes {
		if i >= 10 {
			break
		}

		var emoji string
		if process.MemoryPercent >= 10.0 {
			emoji = "🔴" // High usage
		} else if process.MemoryPercent >= 5.0 {
			emoji = "🟠" // Medium usage
		} else if process.MemoryPercent >= 1.0 {
			emoji = "🟡" // Low-medium usage
		} else {
			emoji = "🟢" // Low usage
		}

		fieldName := fmt.Sprintf("%s #%d - %s", emoji, i+1, process.Command)
		fieldValue := fmt.Sprintf("**Memory**: %.1f%%\n**CPU**: %.1f%%\n**User**: %s\n**PID**: %s",
			process.MemoryPercent, process.CPUPercent, process.User, process.PID)

		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   fieldName,
			Value:  fieldValue,
			Inline: true,
		})

		logger.Info("Added process field:", process.Command, "Memory:", process.MemoryPercent, "%")
	}

	// Add summary field
	if len(processes) > 0 {
		summaryValue := fmt.Sprintf("**Highest**: %s (%.1f%%)\n**Average**: %.1f%%\n**Last Updated**: <t:%d:R>",
			processes[0].Command, processes[0].MemoryPercent, totalMemory/float64(len(processes)), time.Now().Unix())

		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "📊 Summary",
			Value:  summaryValue,
			Inline: false,
		})
		logger.Info("Added summary field to memory embed")
	}

	logger.Info("Memory embed built successfully with", len(embed.Fields), "fields")
	return embed
}
