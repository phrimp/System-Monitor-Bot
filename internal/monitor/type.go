package monitor

import "time"

// TempStatus represents temperature status
type TempStatus int

const (
	TempNormal TempStatus = iota
	TempWarning
	TempCritical
)

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

// NetworkPort represents a network port
type NetworkPort struct {
	Protocol    string
	Address     string
	Port        string
	State       string
	ProcessName string
	PID         string
}

// MonitorData contains system monitoring data
type MonitorData struct {
	Sensors   []TemperatureSensor
	Ports     []NetworkPort
	Timestamp time.Time
	MaxTemp   float64
}
