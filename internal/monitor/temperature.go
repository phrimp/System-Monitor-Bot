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

type TemperatureMonitor struct {
	criticalThreshold float64
	warningThreshold  float64
}

func NewTemperatureMonitor(critical, warning float64) *TemperatureMonitor {
	return &TemperatureMonitor{
		criticalThreshold: critical,
		warningThreshold:  warning,
	}
}

func (tm *TemperatureMonitor) GetSensors() ([]TemperatureSensor, error) {
	logger.Info("🌡️ Starting temperature sensor reading...")

	// Check if sensors command exists
	if _, err := exec.LookPath("sensors"); err != nil {
		logger.Error("❌ lm-sensors not found:", err)
		return nil, fmt.Errorf("lm-sensors not installed - run: sudo pacman -S lm_sensors")
	}

	// Execute sensors command
	startTime := time.Now()
	cmd := exec.Command("sensors", "-A", "-u")
	output, err := cmd.Output()
	duration := time.Since(startTime)

	if err != nil {
		logger.Error("❌ sensors command failed after", duration, err)
		return nil, fmt.Errorf("sensors command failed: %v", err)
	}

	logger.Info("✅ sensors command completed in", duration)
	return tm.parseSensorsOutput(string(output))
}

func (tm *TemperatureMonitor) parseSensorsOutput(output string) ([]TemperatureSensor, error) {
	var sensors []TemperatureSensor
	lines := strings.Split(output, "\n")

	var currentChip string
	tempValues := make(map[string]float64)
	tempLabels := make(map[string]string)

	tempRegex := regexp.MustCompile(`^(\w+)_input:\s+([\d.]+)`)
	labelRegex := regexp.MustCompile(`^(\w+)_label:\s+(.+)`)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Detect chip name
		if !strings.Contains(line, ":") && line != "" {
			currentChip = line
			continue
		}

		// Parse temperature values
		if matches := tempRegex.FindStringSubmatch(line); matches != nil {
			sensorName := matches[1]
			if temp, err := strconv.ParseFloat(matches[2], 64); err == nil {
				if strings.Contains(sensorName, "temp") || strings.Contains(sensorName, "Core") {
					key := fmt.Sprintf("%s_%s", currentChip, sensorName)
					tempValues[key] = temp
				}
			}
		}

		// Parse temperature labels
		if matches := labelRegex.FindStringSubmatch(line); matches != nil {
			sensorName := matches[1]
			label := matches[2]
			if strings.Contains(sensorName, "temp") || strings.Contains(sensorName, "Core") {
				key := fmt.Sprintf("%s_%s", currentChip, sensorName)
				tempLabels[key] = label
			}
		}
	}

	// Create sensor objects
	for key, temperature := range tempValues {
		label := tempLabels[key]
		if label == "" {
			parts := strings.Split(key, "_")
			if len(parts) >= 2 {
				label = fmt.Sprintf("%s %s", parts[0], parts[1])
			} else {
				label = key
			}
		}

		sensor := TemperatureSensor{
			ID:          key,
			Name:        tm.getReadableSensorName(label),
			Temperature: temperature,
			Category:    tm.categorizeSensor(label),
			Status:      tm.getTemperatureStatus(temperature),
		}
		sensors = append(sensors, sensor)
	}

	// Fallback parsing if no structured data found
	if len(sensors) == 0 {
		sensors = tm.parseSimpleSensorsOutput(output)
	}

	// Sort sensors
	sort.Slice(sensors, func(i, j int) bool {
		if sensors[i].Category != sensors[j].Category {
			return sensors[i].Category < sensors[j].Category
		}
		return sensors[i].Temperature > sensors[j].Temperature
	})

	return sensors, nil
}

func (tm *TemperatureMonitor) parseSimpleSensorsOutput(output string) []TemperatureSensor {
	var sensors []TemperatureSensor
	lines := strings.Split(output, "\n")
	tempRegex := regexp.MustCompile(`(Core \d+|temp\d+|CPU|GPU).*?([+-]?\d+\.\d+)°C`)

	for _, line := range lines {
		if matches := tempRegex.FindStringSubmatch(line); matches != nil {
			if temp, err := strconv.ParseFloat(matches[2], 64); err == nil {
				sensor := TemperatureSensor{
					ID:          strings.ToLower(strings.ReplaceAll(matches[1], " ", "_")),
					Name:        matches[1],
					Temperature: temp,
					Category:    tm.categorizeSensor(matches[1]),
					Status:      tm.getTemperatureStatus(temp),
				}
				sensors = append(sensors, sensor)
			}
		}
	}
	return sensors
}

func (tm *TemperatureMonitor) getTemperatureStatus(temp float64) TempStatus {
	if temp >= tm.criticalThreshold {
		return TempCritical
	}
	if temp >= tm.warningThreshold {
		return TempWarning
	}
	return TempNormal
}

func (tm *TemperatureMonitor) getReadableSensorName(label string) string {
	lower := strings.ToLower(label)

	// CPU sensors
	if strings.Contains(lower, "package id 0") {
		return "CPU Package"
	}
	if strings.Contains(lower, "core 0") {
		return "CPU Core 0"
	}
	if strings.Contains(lower, "core 1") {
		return "CPU Core 1"
	}
	// ... continue with other sensor mappings

	cleaned := strings.ReplaceAll(label, "_", " ")
	return strings.Title(cleaned)
}

func (tm *TemperatureMonitor) categorizeSensor(label string) string {
	lower := strings.ToLower(label)

	if strings.Contains(lower, "core") || strings.Contains(lower, "package") ||
		strings.Contains(lower, "cpu") || strings.Contains(lower, "peci") {
		return CategoryCPU
	}

	if strings.Contains(lower, "gpu") || strings.Contains(lower, "nouveau") ||
		strings.Contains(lower, "radeon") || strings.Contains(lower, "amdgpu") {
		return CategoryGPU
	}

	// ... continue with other categories

	return CategoryOther
}
