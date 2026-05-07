package usernamescan

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

const (
	defaultScanIntervalMs = 500
	maxHistoryPoints      = 100
	maxStoredResults      = 200
	maxStoredLogs         = 200
)

type ScanFunc func(ctx context.Context, request StartRequest, name string) Result

type Service struct {
	mu         sync.Mutex
	running    bool
	paused     bool
	cancel     context.CancelFunc
	generation int

	provider   string
	scanFunc   ScanFunc
	candidates []string
	nextIndex  int
	activeName string
	stats      Stats
	history    History
	results    []Result
	logs       []LogEntry

	onSnapshot func(Snapshot)
	onLog      func(LogEntry)
}

func NewService(onSnapshot func(Snapshot), onLog func(LogEntry)) *Service {
	return &Service{
		onSnapshot: onSnapshot,
		onLog:      onLog,
	}
}

func (s *Service) SetScanFunc(scanFunc ScanFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.scanFunc = scanFunc
}

func (s *Service) Generate(options GeneratorOptions) []string {
	return GenerateCandidates(options)
}

func (s *Service) Start(request StartRequest) Snapshot {
	candidates := buildStartCandidates(request)
	intervalMs := request.IntervalMs
	if intervalMs <= 0 {
		intervalMs = defaultScanIntervalMs
	}
	intervalMs = clampInt(intervalMs, 100, 10_000)

	if request.Limit > 0 && request.Limit < len(candidates) {
		candidates = candidates[:request.Limit]
	}

	provider := normalizeProvider(request.Provider)
	ctx, cancel := context.WithCancel(context.Background())

	s.mu.Lock()
	if s.cancel != nil {
		s.cancel()
	}
	s.generation++
	generation := s.generation
	s.cancel = cancel
	s.running = len(candidates) > 0
	s.paused = false
	s.provider = provider
	s.candidates = candidates
	s.nextIndex = 0
	s.activeName = ""
	s.stats = Stats{}
	s.history = History{}
	s.results = nil
	s.logs = nil

	if len(candidates) == 0 {
		entry := newLogEntry("warning", "No candidates to scan.", nil)
		s.appendLogLocked(entry)
		snap := s.snapshotLocked()
		s.mu.Unlock()
		s.emitLog(entry)
		s.emitSnapshot(snap)
		return snap
	}

	entry := newLogEntry("info", "Username scan started.", map[string]any{
		"count":      len(candidates),
		"provider":   provider,
		"intervalMs": intervalMs,
	})
	s.appendLogLocked(entry)
	snap := s.snapshotLocked()
	s.mu.Unlock()

	s.emitLog(entry)
	s.emitSnapshot(snap)
	go s.run(ctx, generation, request, time.Duration(intervalMs)*time.Millisecond)
	return snap
}

func (s *Service) Pause() Snapshot {
	s.mu.Lock()
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
	if s.running {
		s.paused = true
		s.activeName = ""
	}
	entry := newLogEntry("info", "Username scan paused.", nil)
	s.appendLogLocked(entry)
	snap := s.snapshotLocked()
	s.mu.Unlock()

	s.emitLog(entry)
	s.emitSnapshot(snap)
	return snap
}

func (s *Service) Stop() Snapshot {
	s.mu.Lock()
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
	s.running = false
	s.paused = false
	s.activeName = ""
	entry := newLogEntry("info", "Username scan stopped.", nil)
	s.appendLogLocked(entry)
	snap := s.snapshotLocked()
	s.mu.Unlock()

	s.emitLog(entry)
	s.emitSnapshot(snap)
	return snap
}

func (s *Service) Snapshot() Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.snapshotLocked()
}

func (s *Service) run(ctx context.Context, generation int, request StartRequest, interval time.Duration) {
	for {
		name, ok := s.takeNext(ctx, generation)
		if !ok {
			return
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(120 * time.Millisecond):
		}

		result := s.scan(ctx, request, name)
		if !s.complete(ctx, generation, result) {
			return
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
		}
	}
}

func (s *Service) scan(ctx context.Context, request StartRequest, name string) Result {
	if normalizeProvider(request.Provider) == "mock" || s.scanFunc == nil {
		return mockScan(name)
	}
	return s.scanFunc(ctx, request, name)
}

func (s *Service) takeNext(ctx context.Context, generation int) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if ctx.Err() != nil || generation != s.generation || !s.running || s.paused {
		return "", false
	}

	if s.nextIndex >= len(s.candidates) {
		s.running = false
		s.paused = false
		s.activeName = ""
		entry := newLogEntry("info", "Username scan finished.", map[string]any{
			"checked": s.stats.Checked,
			"hit":     s.stats.Hit,
			"taken":   s.stats.Taken,
			"error":   s.stats.Error,
		})
		s.appendLogLocked(entry)
		snap := s.snapshotLocked()
		go s.emitLog(entry)
		go s.emitSnapshot(snap)
		return "", false
	}

	name := s.candidates[s.nextIndex]
	s.nextIndex++
	s.activeName = name
	snap := s.snapshotLocked()
	go s.emitSnapshot(snap)
	return name, true
}

func (s *Service) complete(ctx context.Context, generation int, result Result) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if ctx.Err() != nil || generation != s.generation || !s.running || s.paused {
		return false
	}

	s.activeName = ""
	s.applyResultLocked(result)
	entry := resultLogEntry(result)
	s.appendLogLocked(entry)
	snap := s.snapshotLocked()
	go s.emitLog(entry)
	go s.emitSnapshot(snap)
	return true
}

func (s *Service) applyResultLocked(result Result) {
	s.stats.Checked++
	switch result.Status {
	case "available":
		s.stats.Hit++
	case "taken":
		s.stats.Taken++
	case "hold":
		s.stats.Hold++
	default:
		s.stats.Error++
	}
	if s.stats.Checked > 0 {
		s.stats.Rate = float64(s.stats.Hit) / float64(s.stats.Checked) * 100
	}

	s.history.Checked = appendBoundedInt(s.history.Checked, s.stats.Checked, maxHistoryPoints)
	s.history.Hit = appendBoundedInt(s.history.Hit, s.stats.Hit, maxHistoryPoints)
	s.history.Rate = appendBoundedFloat(s.history.Rate, s.stats.Rate, maxHistoryPoints)
	s.results = appendBoundedResult(s.results, result, maxStoredResults)
}

func (s *Service) snapshotLocked() Snapshot {
	return Snapshot{
		Running:    s.running,
		Paused:     s.paused,
		Provider:   normalizeProvider(s.provider),
		QueueSize:  len(s.candidates),
		NextIndex:  s.nextIndex,
		ActiveName: s.activeName,
		Stats:      s.stats,
		History: History{
			Checked: append([]int{}, s.history.Checked...),
			Hit:     append([]int{}, s.history.Hit...),
			Rate:    append([]float64{}, s.history.Rate...),
		},
		Results: append([]Result{}, s.results...),
		Logs:    append([]LogEntry{}, s.logs...),
	}
}

func normalizeProvider(provider string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "browser":
		return "browser"
	default:
		return "mock"
	}
}

func (s *Service) appendLogLocked(entry LogEntry) {
	s.logs = appendBoundedLog(s.logs, entry, maxStoredLogs)
}

func (s *Service) emitSnapshot(snapshot Snapshot) {
	if s.onSnapshot != nil {
		s.onSnapshot(snapshot)
	}
}

func (s *Service) emitLog(entry LogEntry) {
	if s.onLog != nil {
		s.onLog(entry)
	}
}

func buildStartCandidates(request StartRequest) []string {
	seen := make(map[string]struct{})
	var candidates []string
	add := func(value string) {
		candidate := normalizeCandidate(value)
		if len(candidate) < 6 || len(candidate) > 30 {
			return
		}
		if _, ok := seen[candidate]; ok {
			return
		}
		seen[candidate] = struct{}{}
		candidates = append(candidates, candidate)
	}

	for _, candidate := range request.Candidates {
		add(candidate)
	}
	if len(candidates) > 0 {
		return candidates
	}

	for _, seed := range request.Seeds {
		add(seed)
	}
	if len(candidates) > 0 {
		return candidates
	}

	generated := GenerateCandidates(GeneratorOptions{
		SourceText:   strings.Join(request.Seeds, " "),
		TargetLength: 6,
		AllowDigits:  false,
		MaxResults:   max(request.Limit, 80),
	})
	for _, candidate := range generated {
		add(candidate)
	}
	return candidates
}

func mockScan(name string) Result {
	score := 0
	for _, ch := range name {
		score += int(ch)
	}

	status := "taken"
	message := fmt.Sprintf("%s is marked as taken by the mock provider.", name)
	if score%12 == 0 {
		status = "error"
		message = fmt.Sprintf("%s produced a mock provider error.", name)
	} else if score%12 == 2 || score%12 == 5 || score%12 == 9 {
		status = "available"
		message = fmt.Sprintf("%s is marked as available by the mock provider.", name)
	}

	return Result{
		Name:      name,
		Status:    status,
		Message:   message,
		CheckedAt: time.Now().Format(time.RFC3339),
	}
}

func resultLogEntry(result Result) LogEntry {
	level := "info"
	switch result.Status {
	case "available":
		level = "success"
	case "error":
		level = "error"
	}
	return newLogEntry(level, result.Message, map[string]any{
		"name":   result.Name,
		"status": result.Status,
	})
}

func newLogEntry(level string, message string, params map[string]any) LogEntry {
	if params == nil {
		params = map[string]any{}
	}
	return LogEntry{
		Level:     level,
		Message:   message,
		Params:    params,
		Timestamp: time.Now().Format(time.RFC3339),
	}
}

func appendBoundedInt(items []int, value int, limit int) []int {
	items = append(items, value)
	if len(items) > limit {
		return append([]int{}, items[len(items)-limit:]...)
	}
	return items
}

func appendBoundedFloat(items []float64, value float64, limit int) []float64 {
	items = append(items, value)
	if len(items) > limit {
		return append([]float64{}, items[len(items)-limit:]...)
	}
	return items
}

func appendBoundedResult(items []Result, value Result, limit int) []Result {
	items = append([]Result{value}, items...)
	if len(items) > limit {
		return append([]Result{}, items[:limit]...)
	}
	return items
}

func appendBoundedLog(items []LogEntry, value LogEntry, limit int) []LogEntry {
	items = append([]LogEntry{value}, items...)
	if len(items) > limit {
		return append([]LogEntry{}, items[:limit]...)
	}
	return items
}
