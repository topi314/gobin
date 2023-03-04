package cmd

type ErrorResponse struct {
	Message   string `json:"message"`
	Status    int    `json:"status"`
	Path      string `json:"path"`
	RequestID string `json:"request_id"`
}
