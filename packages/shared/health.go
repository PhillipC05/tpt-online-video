package shared

type HealthStatus struct {
	Status string            `json:"status"`
	Checks map[string]string `json:"checks"`
}

func Healthy(checks map[string]string) HealthStatus {
	return HealthStatus{Status: "ok", Checks: checks}
}

func Unhealthy(checks map[string]string) HealthStatus {
	return HealthStatus{Status: "error", Checks: checks}
}