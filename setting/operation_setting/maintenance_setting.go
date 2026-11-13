package operation_setting

import "github.com/QuantumNous/new-api/setting/config"

// MaintenanceSetting stops new executions, not management or retained results.
type MaintenanceSetting struct {
	Enabled bool   `json:"enabled"`
	Message string `json:"message"`
}

var maintenanceSetting = MaintenanceSetting{Message: "The API is temporarily under maintenance. Please try again later."}

func init() {
	config.GlobalConfig.Register("maintenance_setting", &maintenanceSetting)
}

func GetMaintenanceSetting() *MaintenanceSetting {
	return &maintenanceSetting
}
