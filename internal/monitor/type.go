// internal/monitor/type.go
package monitor

import (
	"system-monitor-bot/pkg/logger"
	"time"
)

// TempStatus represents temperature status
type TempStatus int

const (
	TempNormal TempStatus = iota
	TempWarning
	TempCritical
)

// String method for TempStatus to improve logging
func (ts TempStatus) String() string {
	switch ts {
	case TempNormal:
		return "Normal"
	case TempWarning:
		return "Warning"
	case TempCritical:
		return "Critical"
	default:
		return "Unknown"
	}
}

// Hardware categories for temperature sensors
const (
	CategoryCPU         = "CPU"
	CategoryGPU         = "GPU"
	CategoryMotherboard = "Motherboard"
	CategoryChipset     = "Chipset"
	CategoryWiFi        = "WiFi"
	CategoryStorage     = "Storage"
	CategorySystem      = "System"
	CategoryOther       = "Other"
)

// TemperatureSensor represents a temperature reading
type TemperatureSensor struct {
	ID          string
	Name        string
	Temperature float64
	Category    string
	Status      TempStatus
}

// LogDetails logs detailed information about the temperature sensor
func (ts *TemperatureSensor) LogDetails() {
	logger.Info("TemperatureSensor Details:")
	logger.Info("- ID:", ts.ID)
	logger.Info("- Name:", ts.Name)
	logger.Info("- Temperature:", ts.Temperature, "°C")
	logger.Info("- Category:", ts.Category)
	logger.Info("- Status:", ts.Status.String())
}

// NetworkPort represents a network port
type NetworkPort struct {
	Protocol    string
	Address     string
	Port        string
	State       string
	ProcessName string
	PID         string
}

// LogDetails logs detailed information about the network port
func (np *NetworkPort) LogDetails() {
	logger.Info("NetworkPort Details:")
	logger.Info("- Protocol:", np.Protocol)
	logger.Info("- Address:", np.Address)
	logger.Info("- Port:", np.Port)
	logger.Info("- State:", np.State)
	logger.Info("- ProcessName:", np.ProcessName)
	logger.Info("- PID:", np.PID)
}

// ProcessMemory represents a process's memory usage
type ProcessMemory struct {
	PID           string
	User          string
	Command       string
	MemoryPercent float64
	CPUPercent    float64
}

// LogDetails logs detailed information about the process memory usage
func (pm *ProcessMemory) LogDetails() {
	logger.Info("ProcessMemory Details:")
	logger.Info("- PID:", pm.PID)
	logger.Info("- User:", pm.User)
	logger.Info("- Command:", pm.Command)
	logger.Info("- Memory:", pm.MemoryPercent, "%")
	logger.Info("- CPU:", pm.CPUPercent, "%")
}

// MonitorData contains system monitoring data
type MonitorData struct {
	Sensors     []TemperatureSensor
	Ports       []NetworkPort
	Processes   []ProcessMemory
	Timestamp   time.Time
	MaxTemp     float64
	TotalMemory float64
}

// LogSummary logs a summary of the monitoring data
func (md *MonitorData) LogSummary() {
	logger.Info("MonitorData Summary:")
	logger.Info("- Timestamp:", md.Timestamp.Format("2006-01-02 15:04:05"))
	logger.Info("- Total Sensors:", len(md.Sensors))
	logger.Info("- Total Ports:", len(md.Ports))
	logger.Info("- Total Processes:", len(md.Processes))
	logger.Info("- Max Temperature:", md.MaxTemp, "°C")

	if len(md.Sensors) > 0 {
		criticalCount := 0
		warningCount := 0
		normalCount := 0

		for _, sensor := range md.Sensors {
			switch sensor.Status {
			case TempCritical:
				criticalCount++
			case TempWarning:
				warningCount++
			case TempNormal:
				normalCount++
			}
		}

		logger.Info("- Temperature Status Distribution:")
		logger.Info("  - Critical:", criticalCount)
		logger.Info("  - Warning:", warningCount)
		logger.Info("  - Normal:", normalCount)
	}

	if len(md.Ports) > 0 {
		tcpCount := 0
		udpCount := 0

		for _, port := range md.Ports {
			switch port.Protocol {
			case "TCP":
				tcpCount++
			case "UDP":
				udpCount++
			}
		}

		logger.Info("- Port Protocol Distribution:")
		logger.Info("  - TCP:", tcpCount)
		logger.Info("  - UDP:", udpCount)
	}

	if len(md.Processes) > 0 {
		totalMemUsage := 0.0
		for _, process := range md.Processes {
			totalMemUsage += process.MemoryPercent
		}

		logger.Info("- Memory Usage Summary:")
		logger.Info("  - Top 5 Total Memory:", totalMemUsage, "%")
		if len(md.Processes) > 0 {
			logger.Info("  - Highest Process:", md.Processes[0].Command, md.Processes[0].MemoryPercent, "%")
		}
	}
}
