package monitor

import (
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"system-monitor-bot/pkg/logger"
	"time"
)

type MemoryMonitor struct{}

func NewMemoryMonitor() *MemoryMonitor {
	logger.Info("Creating new MemoryMonitor instance")
	return &MemoryMonitor{}
}

func (mm *MemoryMonitor) GetTopProcesses() ([]ProcessMemory, error) {
	logger.Info("Starting memory usage reading...")

	// Check if top command exists
	logger.Info("Checking for top command availability...")
	if _, err := exec.LookPath("top"); err != nil {
		logger.Error("top command not found:", err)
		return nil, fmt.Errorf("top command not found")
	}
	logger.Info("top command found and available")

	// Execute top command with batch mode, 1 iteration, sorted by memory
	logger.Info("Executing top command with flags: -b -n1 -o %MEM")
	startTime := time.Now()
	cmd := exec.Command("top", "-b", "-n1", "-o", "%MEM")
	output, err := cmd.Output()
	duration := time.Since(startTime)

	if err != nil {
		logger.Error("top command failed after", duration, "error:", err)
		return nil, fmt.Errorf("top command failed: %v", err)
	}

	logger.Info("top command completed successfully in", duration)
	logger.Info("top output length:", len(output), "bytes")

	processes, parseErr := mm.parseTopOutput(string(output))
	if parseErr != nil {
		logger.Error("Failed to parse top output:", parseErr)
		return nil, parseErr
	}

	logger.Info("Successfully parsed", len(processes), "memory processes")
	return processes, nil
}

func (mm *MemoryMonitor) parseTopOutput(output string) ([]ProcessMemory, error) {
	logger.Info("Starting top output parsing...")
	var processes []ProcessMemory
	lines := strings.Split(output, "\n")
	logger.Info("Processing", len(lines), "lines from top output")

	// Find the header line to understand column positions
	headerFound := false
	headerLine := ""
	dataStartIndex := 0

	for i, line := range lines {
		if strings.Contains(line, "PID") && strings.Contains(line, "%MEM") && strings.Contains(line, "COMMAND") {
			headerFound = true
			headerLine = line
			dataStartIndex = i + 1
			logger.Info("Found header line at index", i, ":", headerLine)
			break
		}
	}

	if !headerFound {
		logger.Error("Could not find header line in top output")
		return nil, fmt.Errorf("invalid top output format - no header found")
	}

	// Parse column positions
	pidCol := strings.Index(headerLine, "PID")
	userCol := strings.Index(headerLine, "USER")
	memCol := strings.Index(headerLine, "%MEM")
	cpuCol := strings.Index(headerLine, "%CPU")
	commandCol := strings.Index(headerLine, "COMMAND")

	logger.Info("Column positions - PID:", pidCol, "USER:", userCol, "MEM:", memCol, "CPU:", cpuCol, "COMMAND:", commandCol)

	processedLines := 0
	foundProcesses := 0

	// Regex for parsing process lines - more flexible approach
	processRegex := regexp.MustCompile(`^\s*(\d+)\s+(\S+)\s+\S+\s+\S+\s+\S+\s+\S+\s+\S+\s+\S+\s+([\d.]+)\s+([\d.]+)\s+\S+\s+(.+)$`)

	for i := dataStartIndex; i < len(lines) && foundProcesses < 15; i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		processedLines++

		matches := processRegex.FindStringSubmatch(line)
		if len(matches) >= 6 {
			pid := matches[1]
			user := matches[2]
			memPercent := matches[3]
			cpuPercent := matches[4]
			command := strings.TrimSpace(matches[5])

			// Parse memory percentage
			memPct, err := strconv.ParseFloat(memPercent, 64)
			if err != nil {
				logger.Info("Could not parse memory percentage:", memPercent, "for PID:", pid)
				continue
			}

			// Parse CPU percentage
			cpuPct, err := strconv.ParseFloat(cpuPercent, 64)
			if err != nil {
				logger.Info("Could not parse CPU percentage:", cpuPercent, "for PID:", pid)
				cpuPct = 0.0
			}

			// Skip processes with 0% memory
			if memPct == 0.0 {
				continue
			}

			process := ProcessMemory{
				PID:           pid,
				User:          user,
				Command:       mm.cleanCommandName(command),
				MemoryPercent: memPct,
				CPUPercent:    cpuPct,
			}

			processes = append(processes, process)
			foundProcesses++
			logger.Info("Found process:", pid, command, "Memory:", memPct, "% CPU:", cpuPct, "%")
		} else {
			logger.Info("Skipping line", i+1, "- regex didn't match:", line)
		}
	}

	logger.Info("Top parsing statistics:")
	logger.Info("- Processed lines:", processedLines)
	logger.Info("- Found processes:", foundProcesses)

	sort.Slice(processes, func(i, j int) bool {
		return processes[i].MemoryPercent > processes[j].MemoryPercent
	})

	if len(processes) > 10 {
		processes = processes[:10]
		logger.Info("Trimmed to top 10 processes by memory usage")
	}

	logger.Info("Memory usage parsing complete. Final process count:", len(processes))
	return processes, nil
}

func (mm *MemoryMonitor) cleanCommandName(command string) string {
	logger.Info("Cleaning command name:", command)

	// Remove command line arguments for cleaner display
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return command
	}

	// Get the base command
	baseCommand := parts[0]

	// Remove path and get just the binary name
	if strings.Contains(baseCommand, "/") {
		pathParts := strings.Split(baseCommand, "/")
		baseCommand = pathParts[len(pathParts)-1]
	}

	// Handle bracketed processes (kernel threads)
	if strings.HasPrefix(baseCommand, "[") && strings.HasSuffix(baseCommand, "]") {
		result := strings.Trim(baseCommand, "[]")
		logger.Info("Cleaned kernel thread name:", command, "->", result)
		return result
	}

	// Map common process names to friendlier versions
	processMap := map[string]string{
		"dockerd":        "Docker Daemon",
		"containerd":     "Container Runtime",
		"docker-proxy":   "Docker Proxy",
		"nginx":          "Nginx",
		"apache2":        "Apache",
		"httpd":          "Apache",
		"node":           "Node.js",
		"mysql":          "MySQL",
		"mysqld":         "MySQL",
		"postgres":       "PostgreSQL",
		"redis-server":   "Redis",
		"mongod":         "MongoDB",
		"systemd":        "SystemD",
		"chrome":         "Chrome",
		"firefox":        "Firefox",
		"code":           "VS Code",
		"gnome-shell":    "GNOME Shell",
		"Xorg":           "X Server",
		"pulseaudio":     "PulseAudio",
		"NetworkManager": "Network Manager",
	}

	if friendlyName, exists := processMap[baseCommand]; exists {
		logger.Info("Mapped process name:", command, "->", friendlyName)
		return friendlyName
	}

	logger.Info("Using cleaned base command:", command, "->", baseCommand)
	return baseCommand
}
