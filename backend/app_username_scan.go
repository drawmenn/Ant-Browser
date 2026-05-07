package backend

import (
	"ant-chrome/backend/internal/usernamescan"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	usernameScanSnapshotEvent = "username-scan:snapshot"
	usernameScanLogEvent      = "username-scan:log"
)

type UsernameScanGeneratorOptions = usernamescan.GeneratorOptions
type UsernameScanStartRequest = usernamescan.StartRequest
type UsernameScanSnapshot = usernamescan.Snapshot
type UsernameScanLogEntry = usernamescan.LogEntry

func (a *App) ensureUsernameScanner() *usernamescan.Service {
	if a.usernameScanner != nil {
		return a.usernameScanner
	}

	a.usernameScanner = usernamescan.NewService(
		func(snapshot usernamescan.Snapshot) {
			if a.ctx != nil {
				runtime.EventsEmit(a.ctx, usernameScanSnapshotEvent, snapshot)
			}
		},
		func(entry usernamescan.LogEntry) {
			if a.ctx != nil {
				runtime.EventsEmit(a.ctx, usernameScanLogEvent, entry)
			}
		},
	)
	a.usernameScanner.SetScanFunc(a.scanUsernameInBrowser)
	return a.usernameScanner
}

func (a *App) UsernameScanGenerate(options UsernameScanGeneratorOptions) []string {
	return a.ensureUsernameScanner().Generate(options)
}

func (a *App) UsernameScanStart(request UsernameScanStartRequest) (UsernameScanSnapshot, error) {
	if err := a.validateUsernameScanStart(request); err != nil {
		return a.ensureUsernameScanner().Snapshot(), err
	}
	return a.ensureUsernameScanner().Start(request), nil
}

func (a *App) UsernameScanPause() UsernameScanSnapshot {
	return a.ensureUsernameScanner().Pause()
}

func (a *App) UsernameScanStop() UsernameScanSnapshot {
	return a.ensureUsernameScanner().Stop()
}

func (a *App) UsernameScanSnapshot() UsernameScanSnapshot {
	return a.ensureUsernameScanner().Snapshot()
}
