package monitor

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"system-monitor-bot/pkg/logger"
	"time"
)

type NetworkMonitor struct{}

func NewNetworkMonitor() *NetworkMonitor {
	return &NetworkMonitor{}
}

func (nm *NetworkMonitor) GetPorts(showAll bool) ([]NetworkPort, error) {
	logger.Info("🔌 Starting network ports reading...")

	// Check if ss command exists
	if _, err := exec.LookPath("ss"); err != nil {
		logger.Error("❌ ss command not found:", err)
		return nil, fmt.Errorf("ss command not found")
	}

	// Execute ss command
	startTime := time.Now()
	cmd := exec.Command("ss", "-tulnp")
	output, err := cmd.Output()
	duration := time.Since(startTime)

	if err != nil {
		logger.Error("❌ ss command failed after", duration, err)
		return nil, fmt.Errorf("ss command failed: %v", err)
	}

	logger.Info("✅ ss command completed in", duration)
	return nm.parseNetworkOutput(string(output), showAll)
}

func (nm *NetworkMonitor) parseNetworkOutput(output string, showAll bool) ([]NetworkPort, error) {
	var ports []NetworkPort
	lines := strings.Split(output, "\n")

	for i, line := range lines {
		// Skip header and empty lines
		if i == 0 || strings.TrimSpace(line) == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}

		protocol := strings.ToLower(fields[0])
		state := ""
		address := ""
		processInfo := ""

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
			}
		}

		// Filter for listening ports if not showing all
		if !showAll && !strings.Contains(state, "LISTEN") && !strings.Contains(state, "UNCONN") {
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
	}

	return ports, nil
}

func (nm *NetworkMonitor) parseProcessInfo(processField string) string {
	// Extract process name and PID
	re := regexp.MustCompile(`\(\("([^"]+)",pid=(\d+)`)
	matches := re.FindStringSubmatch(processField)

	if len(matches) >= 3 {
		processName := matches[1]
		pid := matches[2]
		enhancedName := nm.enhanceProcessName(processName)
		return fmt.Sprintf("%s (PID: %s)", enhancedName, pid)
	}

	// Fallback: extract just process name
	re2 := regexp.MustCompile(`"([^"]+)"`)
	matches2 := re2.FindStringSubmatch(processField)
	if len(matches2) >= 2 {
		processName := matches2[1]
		return nm.enhanceProcessName(processName)
	}

	return "Unknown Process"
}

func (nm *NetworkMonitor) enhanceProcessName(processName string) string {
	lower := strings.ToLower(processName)

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
			return value
		}
	}

	return strings.Title(processName)
}
