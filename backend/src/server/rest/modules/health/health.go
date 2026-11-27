package health

import (
	"encoding/json"
	"log"
	"net/http"
)

// Check reports the state of one component as named boolean conditions. The
// component is ready only when every condition is true.
type Check struct {
	Name   string
	Status func() map[string]bool
}

type report struct {
	Status     string                     `json:"status"`
	Components map[string]map[string]bool `json:"components"`
}

// Handler reports readiness of all checks: 200 when every component is ready,
// 503 otherwise. An open HTTP listener alone does not make the app healthy.
func Handler(checks ...Check) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rep := report{Status: "ok", Components: make(map[string]map[string]bool, len(checks))}
		for _, c := range checks {
			state := c.Status()
			rep.Components[c.Name] = state
			for _, ok := range state {
				if !ok {
					rep.Status = "unavailable"
				}
			}
		}

		code := http.StatusOK
		if rep.Status != "ok" {
			code = http.StatusServiceUnavailable
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		if err := json.NewEncoder(w).Encode(rep); err != nil {
			log.Printf("health: write response: %v", err)
		}
	}
}
