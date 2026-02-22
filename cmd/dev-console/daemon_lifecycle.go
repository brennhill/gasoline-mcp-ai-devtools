package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dev-console/dev-console/internal/state"
)

type daemonLaunchOptions struct {
	Parallel bool
}

type daemonLockRecord struct {
	PID       int    `json:"pid"`
	Port      int    `json:"port"`
	StateDir  string `json:"state_dir"`
	Version   string `json:"version,omitempty"`
	UpdatedAt string `json:"updated_at"`
}

var (
	daemonIsProcessAlive     = isProcessAlive
	daemonTryShutdown        = tryShutdownViaHTTP
	daemonWaitForPortRelease = waitForPortRelease
	daemonTerminatePID       = terminatePIDQuiet
	daemonNow                = time.Now
)

func daemonLockFilePath() (string, error) {
	return state.InRoot("run", "daemon.lock.json")
}

func daemonLockFilePathForError() string {
	path, err := daemonLockFilePath()
	if err != nil {
		return "<unknown>"
	}
	return path
}

func readDaemonLockFile() (*daemonLockRecord, error) {
	path, err := daemonLockFilePath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var rec daemonLockRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		return nil, fmt.Errorf("invalid daemon lock metadata at %s: %w", path, err)
	}
	return &rec, nil
}

func writeDaemonLockFile(rec daemonLockRecord) error {
	path, err := daemonLockFilePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	if rec.UpdatedAt == "" {
		rec.UpdatedAt = daemonNow().UTC().Format(time.RFC3339)
	}
	data, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func removeDaemonLockFile() error {
	path, err := daemonLockFilePath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func removeDaemonLockIfOwned(pid int) {
	rec, err := readDaemonLockFile()
	if err != nil || rec == nil {
		return
	}
	if rec.PID == pid {
		_ = removeDaemonLockFile()
	}
}

func persistCurrentDaemonLock(port int) error {
	stateDir, err := state.RootDir()
	if err != nil {
		return err
	}
	return writeDaemonLockFile(daemonLockRecord{
		PID:       os.Getpid(),
		Port:      port,
		StateDir:  stateDir,
		Version:   version,
		UpdatedAt: daemonNow().UTC().Format(time.RFC3339),
	})
}

func enforceDaemonStartupPolicy(server *Server, port int, opts daemonLaunchOptions) error {
	stateDir, err := state.RootDir()
	if err != nil {
		return fmt.Errorf("cannot resolve state dir: %w", err)
	}
	rec, err := readDaemonLockFile()
	if err != nil {
		return err
	}
	if rec == nil {
		return nil
	}

	if opts.Parallel {
		return validateParallelIsolation(rec)
	}
	return performDefaultTakeover(server, stateDir, port, rec)
}

func validateParallelIsolation(rec *daemonLockRecord) error {
	if rec.PID <= 0 || rec.Port <= 0 {
		return fmt.Errorf(
			"parallel mode requires isolated --state-dir; invalid daemon lock metadata (pid=%d, port=%d) at %s",
			rec.PID,
			rec.Port,
			daemonLockFilePathForError(),
		)
	}
	if rec.PID == os.Getpid() {
		return nil
	}
	if !daemonIsProcessAlive(rec.PID) {
		return removeDaemonLockFile()
	}
	return fmt.Errorf(
		"parallel mode requires isolated --state-dir; existing daemon is active (existing_pid=%d existing_port=%d state_dir=%s)",
		rec.PID,
		rec.Port,
		rec.StateDir,
	)
}

func performDefaultTakeover(server *Server, stateDir string, port int, rec *daemonLockRecord) error {
	if rec.PID <= 0 || rec.Port <= 0 {
		return fmt.Errorf(
			"invalid daemon lock metadata for state_dir=%s (pid=%d, port=%d). remove %s and retry",
			stateDir,
			rec.PID,
			rec.Port,
			daemonLockFilePathForError(),
		)
	}
	if rec.PID == os.Getpid() {
		return nil
	}
	if !daemonIsProcessAlive(rec.PID) {
		return removeDaemonLockFile()
	}

	pidFromPortFile := readPIDFile(rec.Port)
	if pidFromPortFile != rec.PID {
		return fmt.Errorf(
			"daemon ownership mismatch for state_dir=%s: lock pid=%d port=%d, pid_file=%d; refusing takeover",
			stateDir,
			rec.PID,
			rec.Port,
			pidFromPortFile,
		)
	}

	server.logLifecycle("daemon_takeover", port, map[string]any{
		"existing_pid":  rec.PID,
		"existing_port": rec.Port,
		"takeover":      true,
		"state_dir":     stateDir,
		"new_pid":       os.Getpid(),
	})

	_ = daemonTryShutdown(rec.Port)
	if !daemonWaitForPortRelease(rec.Port, 2*time.Second) {
		daemonTerminatePID(rec.PID, false)
		if !daemonWaitForPortRelease(rec.Port, 2*time.Second) {
			daemonTerminatePID(rec.PID, true)
			if !daemonWaitForPortRelease(rec.Port, 2*time.Second) {
				return fmt.Errorf(
					"failed to takeover existing daemon for state_dir=%s (existing_pid=%d existing_port=%d)",
					stateDir,
					rec.PID,
					rec.Port,
				)
			}
		}
	}

	removePIDFile(rec.Port)
	return removeDaemonLockFile()
}
