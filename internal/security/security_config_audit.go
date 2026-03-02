package security

import (
	"sync"
	"time"
)

var (
	securityAuditLog []SecurityAuditEvent
	securityAuditMu  sync.Mutex
)

func LogSecurityEvent(event SecurityAuditEvent) {
	securityAuditMu.Lock()
	defer securityAuditMu.Unlock()

	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	securityAuditLog = append(securityAuditLog, event)
}

func GetSecurityAuditEvents() []SecurityAuditEvent {
	securityAuditMu.Lock()
	defer securityAuditMu.Unlock()

	events := make([]SecurityAuditEvent, len(securityAuditLog))
	copy(events, securityAuditLog)
	return events
}

func ClearSecurityAuditEvents() {
	securityAuditMu.Lock()
	defer securityAuditMu.Unlock()
	securityAuditLog = nil
}
