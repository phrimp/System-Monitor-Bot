package monitor

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"system-monitor-bot/pkg/logger"
	"time"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

type NetworkMonitor struct{}

func NewNetworkMonitor() *NetworkMonitor {
	logger.Info("Creating new NetworkMonitor instance")
	return &NetworkMonitor{}
}

func (nm *NetworkMonitor) GetPorts(showAll bool) ([]NetworkPort, error) {
	logger.Info("Starting network ports reading with showAll:", showAll)

	// Check if ss command exists
	logger.Info("Checking for ss command availability...")
	if _, err := exec.LookPath("ss"); err != nil {
		logger.Error("ss command not found:", err)
		return nil, fmt.Errorf("ss command not found")
	}
	logger.Info("ss command found and available")

	// Execute ss command
	logger.Info("Executing ss command with flags: -tulnp")
	startTime := time.Now()
	cmd := exec.Command("ss", "-tulnp")
	output, err := cmd.Output()
	duration := time.Since(startTime)

	if err != nil {
		logger.Error("ss command failed after", duration, "error:", err)
		return nil, fmt.Errorf("ss command failed: %v", err)
	}

	logger.Info("ss command completed successfully in", duration)
	logger.Info("ss output length:", len(output), "bytes")

	ports, parseErr := nm.parseNetworkOutput(string(output), showAll)
	if parseErr != nil {
		logger.Error("Failed to parse network output:", parseErr)
		return nil, parseErr
	}

	logger.Info("Successfully parsed", len(ports), "network ports")
	return ports, nil
}

func (nm *NetworkMonitor) parseNetworkOutput(output string, showAll bool) ([]NetworkPort, error) {
	logger.Info("Starting network output parsing...")
	var ports []NetworkPort
	lines := strings.Split(output, "\n")
	logger.Info("Processing", len(lines), "lines from ss output")

	processedLines := 0
	skippedLines := 0
	foundPorts := 0

	for i, line := range lines {
		// Skip header and empty lines
		if i == 0 || strings.TrimSpace(line) == "" {
			continue
		}
		processedLines++

		fields := strings.Fields(line)
		if len(fields) < 4 {
			logger.Info("Skipping line", i+1, "- insufficient fields:", len(fields))
			skippedLines++
			continue
		}

		protocol := strings.ToLower(fields[0])
		state := ""
		address := ""
		processInfo := ""

		logger.Info("Processing line", i+1, "- Protocol:", protocol, "Fields:", len(fields))

		// Parse fields based on ss output format
		if len(fields) >= 5 {
			if strings.Contains(fields[1], "LISTEN") || strings.Contains(fields[1], "UNCONN") {
				state = fields[1]
				address = fields[4]
			} else {
				state = fields[1]
				address = fields[3]
			}
		} else {
			address = fields[3]
		}

		// Extract process information
		if len(fields) > 5 {
			processField := fields[len(fields)-1]
			if strings.Contains(processField, "users:") {
				processInfo = nm.parseProcessInfo(processField)
				logger.Info("Found process info:", processInfo)
			}
		}

		// Filter for listening ports if not showing all
		if !showAll && !strings.Contains(state, "LISTEN") && !strings.Contains(state, "UNCONN") {
			logger.Info("Skipping non-listening port:", address, "state:", state)
			continue
		}

		// Extract port number
		addressParts := strings.Split(address, ":")
		port := ""
		if len(addressParts) > 0 {
			port = addressParts[len(addressParts)-1]
		}

		networkPort := NetworkPort{
			Protocol:    strings.ToUpper(protocol),
			Address:     address,
			Port:        port,
			State:       state,
			ProcessName: processInfo,
		}

		ports = append(ports, networkPort)
		foundPorts++
		logger.Info("Added port:", protocol, address, "port:", port, "state:", state)
	}

	logger.Info("Network parsing statistics:")
	logger.Info("- Processed lines:", processedLines)
	logger.Info("- Skipped lines:", skippedLines)
	logger.Info("- Found ports:", foundPorts)

	return ports, nil
}

func (nm *NetworkMonitor) parseProcessInfo(processField string) string {
	logger.Info("Parsing process info from field:", processField)

	// Extract process name and PID
	re := regexp.MustCompile(`\(\("([^"]+)",pid=(\d+)`)
	matches := re.FindStringSubmatch(processField)

	if len(matches) >= 3 {
		processName := matches[1]
		pid := matches[2]
		enhancedName := nm.enhanceProcessName(processName)
		result := fmt.Sprintf("%s (PID: %s)", enhancedName, pid)
		logger.Info("Extracted process with PID:", result)
		return result
	}

	// Fallback: extract just process name
	re2 := regexp.MustCompile(`"([^"]+)"`)
	matches2 := re2.FindStringSubmatch(processField)
	if len(matches2) >= 2 {
		processName := matches2[1]
		result := nm.enhanceProcessName(processName)
		logger.Info("Extracted process name only:", result)
		return result
	}

	logger.Info("Could not parse process info, using default")
	return "Unknown Process"
}

func (nm *NetworkMonitor) enhanceProcessName(processName string) string {
	logger.Info("Enhancing process name:", processName)
	lower := strings.ToLower(processName)
	caser := cases.Title(language.English)

	processMap := map[string]string{
		"docker-proxy": "Docker Container Port",
		"docker":       "Docker Engine",
		"containerd":   "Container Runtime",
		"nginx":        "Nginx Web Server",
		"apache":       "Apache Web Server",
		"httpd":        "Apache Web Server",
		"node":         "Node.js Application",
		"mysql":        "MySQL Database",
		"mariadb":      "MySQL Database",
		"postgres":     "PostgreSQL Database",
		"redis":        "Redis Cache",
		"mongo":        "MongoDB Database",
		"sshd":         "SSH Server",
		"systemd":      "System Service",
		"resolve":      "DNS Resolver",
		"dhcp":         "DHCP Client",
		"python":       "Python Application",
		"java":         "Java Application",
	}

	for key, value := range processMap {
		if strings.Contains(lower, key) {
			logger.Info("Mapped process name:", processName, "->", value)
			return value
		}
	}

	result := caser.String(processName)
	logger.Info("Using title case for process name:", processName, "->", result)
	return result
}
