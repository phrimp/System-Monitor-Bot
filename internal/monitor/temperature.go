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

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

type TemperatureMonitor struct {
	criticalThreshold float64
	warningThreshold  float64
}

func NewTemperatureMonitor(critical, warning float64) *TemperatureMonitor {
	logger.Info("Creating new TemperatureMonitor with thresholds - Critical:", critical, "Warning:", warning)
	return &TemperatureMonitor{
		criticalThreshold: critical,
		warningThreshold:  warning,
	}
}

func (tm *TemperatureMonitor) GetSensors() ([]TemperatureSensor, error) {
	logger.Info("Starting temperature sensor reading...")

	// Check if sensors command exists
	logger.Info("Checking for lm-sensors availability...")
	if _, err := exec.LookPath("sensors"); err != nil {
		logger.Error("lm-sensors not found:", err)
		return nil, fmt.Errorf("lm-sensors not installed - run: sudo pacman -S lm_sensors")
	}
	logger.Info("lm-sensors found and available")

	// Execute sensors command
	logger.Info("Executing sensors command with flags: -A -u")
	startTime := time.Now()
	cmd := exec.Command("sensors", "-A", "-u")
	output, err := cmd.Output()
	duration := time.Since(startTime)

	if err != nil {
		logger.Error("sensors command failed after", duration, "error:", err)
		return nil, fmt.Errorf("sensors command failed: %v", err)
	}

	logger.Info("sensors command completed successfully in", duration)
	logger.Info("sensors output length:", len(output), "bytes")

	sensors, parseErr := tm.parseSensorsOutput(string(output))
	if parseErr != nil {
		logger.Error("Failed to parse sensors output:", parseErr)
		return nil, parseErr
	}

	logger.Info("Successfully parsed", len(sensors), "temperature sensors")
	return sensors, nil
}

func (tm *TemperatureMonitor) parseSensorsOutput(output string) ([]TemperatureSensor, error) {
	logger.Info("Starting sensors output parsing...")
	var sensors []TemperatureSensor
	lines := strings.Split(output, "\n")
	logger.Info("Processing", len(lines), "lines from sensors output")

	var currentChip string
	tempValues := make(map[string]float64)
	tempLabels := make(map[string]string)

	tempRegex := regexp.MustCompile(`^(\w+)_input:\s+([\d.]+)`)
	labelRegex := regexp.MustCompile(`^(\w+)_label:\s+(.+)`)

	processedLines := 0
	foundTemps := 0
	foundLabels := 0

	for lineNum, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		processedLines++

		// Detect chip name
		if !strings.Contains(line, ":") && line != "" {
			logger.Info("Found chip:", line, "at line", lineNum+1)
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
					foundTemps++
					logger.Info("Found temperature sensor:", key, "=", temp, "°C")
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
				foundLabels++
				logger.Info("Found temperature label:", key, "=", label)
			}
		}
	}

	logger.Info("Parsing statistics - Processed lines:", processedLines, "Temperature values:", foundTemps, "Labels:", foundLabels)

	// Create sensor objects
	logger.Info("Creating sensor objects...")
	for key, temperature := range tempValues {
		label := tempLabels[key]
		if label == "" {
			parts := strings.Split(key, "_")
			if len(parts) >= 2 {
				label = fmt.Sprintf("%s %s", parts[0], parts[1])
			} else {
				label = key
			}
			logger.Info("Generated label for", key, ":", label)
		}

		sensor := TemperatureSensor{
			ID:          key,
			Name:        tm.getReadableSensorName(label),
			Temperature: temperature,
			Category:    tm.categorizeSensor(label),
			Status:      tm.getTemperatureStatus(temperature),
		}
		sensors = append(sensors, sensor)
		logger.Info("Created sensor:", sensor.Name, "Category:", sensor.Category, "Temp:", sensor.Temperature, "Status:", sensor.Status)
	}

	// Fallback parsing if no structured data found
	if len(sensors) == 0 {
		logger.Warn("No structured sensor data found, attempting fallback parsing...")
		sensors = tm.parseSimpleSensorsOutput(output)
		logger.Info("Fallback parsing found", len(sensors), "sensors")
	}

	// Sort sensors
	logger.Info("Sorting sensors by category and temperature...")
	sort.Slice(sensors, func(i, j int) bool {
		if sensors[i].Category != sensors[j].Category {
			return sensors[i].Category < sensors[j].Category
		}
		return sensors[i].Temperature > sensors[j].Temperature
	})

	logger.Info("Temperature sensor parsing complete. Total sensors:", len(sensors))
	return sensors, nil
}

func (tm *TemperatureMonitor) parseSimpleSensorsOutput(output string) []TemperatureSensor {
	logger.Info("Starting simple sensors output parsing as fallback...")
	var sensors []TemperatureSensor
	lines := strings.Split(output, "\n")
	tempRegex := regexp.MustCompile(`(Core \d+|temp\d+|CPU|GPU).*?([+-]?\d+\.\d+)°C`)

	foundSensors := 0
	for lineNum, line := range lines {
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
				foundSensors++
				logger.Info("Fallback found sensor at line", lineNum+1, ":", sensor.Name, "=", temp, "°C")
			}
		}
	}
	logger.Info("Simple parsing complete. Found", foundSensors, "sensors")
	return sensors
}

func (tm *TemperatureMonitor) getTemperatureStatus(temp float64) TempStatus {
	if temp >= tm.criticalThreshold {
		logger.Info("Temperature", temp, "is CRITICAL (>= ", tm.criticalThreshold, ")")
		return TempCritical
	}
	if temp >= tm.warningThreshold {
		logger.Info("Temperature", temp, "is WARNING (>= ", tm.warningThreshold, ")")
		return TempWarning
	}
	return TempNormal
}

func (tm *TemperatureMonitor) getReadableSensorName(label string) string {
	logger.Info("Converting sensor label to readable name:", label)
	lower := strings.ToLower(label)
	caser := cases.Title(language.English)

	// CPU sensors
	if strings.Contains(lower, "package id 0") {
		logger.Info("Mapped to: CPU Package")
		return "CPU Package"
	}
	if strings.Contains(lower, "core 0") {
		logger.Info("Mapped to: CPU Core 0")
		return "CPU Core 0"
	}
	if strings.Contains(lower, "core 1") {
		logger.Info("Mapped to: CPU Core 1")
		return "CPU Core 1"
	}
	// ... continue with other sensor mappings

	cleaned := strings.ReplaceAll(label, "_", " ")
	result := caser.String(cleaned)
	logger.Info("Final readable name:", result)
	return result
}

func (tm *TemperatureMonitor) categorizeSensor(label string) string {
	logger.Info("Categorizing sensor:", label)
	lower := strings.ToLower(label)

	if strings.Contains(lower, "core") || strings.Contains(lower, "package") ||
		strings.Contains(lower, "cpu") || strings.Contains(lower, "peci") {
		logger.Info("Categorized as: CPU")
		return CategoryCPU
	}

	if strings.Contains(lower, "gpu") || strings.Contains(lower, "nouveau") ||
		strings.Contains(lower, "radeon") || strings.Contains(lower, "amdgpu") {
		logger.Info("Categorized as: GPU")
		return CategoryGPU
	}

	// ... continue with other categories

	logger.Info("Categorized as: Other")
	return CategoryOther
}
