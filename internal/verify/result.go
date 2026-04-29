package verify

type Result struct {
	Valid        bool     `json:"valid"`
	Level        string   `json:"level,omitempty"`
	FailureCodes []string `json:"failureCodes"`
}
