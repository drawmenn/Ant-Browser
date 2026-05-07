package usernamescan

type GeneratorOptions struct {
	SourceText   string `json:"sourceText"`
	TargetLength int    `json:"targetLength"`
	AllowDigits  bool   `json:"allowDigits"`
	MaxResults   int    `json:"maxResults"`
}

type StartRequest struct {
	Seeds      []string           `json:"seeds"`
	Candidates []string           `json:"candidates"`
	Provider   string             `json:"provider"`
	Browser    BrowserScanOptions `json:"browser"`
	IntervalMs int                `json:"intervalMs"`
	Limit      int                `json:"limit"`
}

type BrowserScanOptions struct {
	ProfileID         string `json:"profileId"`
	URL               string `json:"url"`
	InputSelector     string `json:"inputSelector"`
	SubmitSelector    string `json:"submitSelector"`
	ResultSelector    string `json:"resultSelector"`
	AvailableText     string `json:"availableText"`
	TakenText         string `json:"takenText"`
	HoldText          string `json:"holdText"`
	WaitAfterSubmitMs int    `json:"waitAfterSubmitMs"`
	TimeoutMs         int    `json:"timeoutMs"`
}

type Stats struct {
	Checked int     `json:"checked"`
	Hit     int     `json:"hit"`
	Taken   int     `json:"taken"`
	Hold    int     `json:"hold"`
	Error   int     `json:"error"`
	Rate    float64 `json:"rate"`
}

type History struct {
	Checked []int     `json:"checked"`
	Hit     []int     `json:"hit"`
	Rate    []float64 `json:"rate"`
}

type Result struct {
	Name      string `json:"name"`
	Status    string `json:"status"`
	Message   string `json:"message"`
	CheckedAt string `json:"checkedAt"`
}

type LogEntry struct {
	Level     string         `json:"level"`
	Message   string         `json:"message"`
	Params    map[string]any `json:"params"`
	Timestamp string         `json:"timestamp"`
}

type Snapshot struct {
	Running    bool       `json:"running"`
	Paused     bool       `json:"paused"`
	Provider   string     `json:"provider"`
	QueueSize  int        `json:"queueSize"`
	NextIndex  int        `json:"nextIndex"`
	ActiveName string     `json:"activeName"`
	Stats      Stats      `json:"stats"`
	History    History    `json:"history"`
	Results    []Result   `json:"results"`
	Logs       []LogEntry `json:"logs"`
}
