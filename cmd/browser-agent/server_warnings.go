// Purpose: One-shot server warning queue for surfacing operator diagnostics via MCP responses.
// Why: Keeps warning state management independent from log buffering internals.

package main

// AddWarning stores a one-shot warning to be surfaced in the next tool response.
func (s *Server) AddWarning(msg string) {
	if msg == "" {
		return
	}
	s.warningsMu.Lock()
	defer s.warningsMu.Unlock()
	if s.warningSeen == nil {
		s.warningSeen = make(map[string]struct{})
	}
	if _, ok := s.warningSeen[msg]; ok {
		return
	}
	s.warningSeen[msg] = struct{}{}
	s.warnings = append(s.warnings, msg)
}

// TakeWarnings returns pending warnings and clears the pending list.
func (s *Server) TakeWarnings() []string {
	s.warningsMu.Lock()
	defer s.warningsMu.Unlock()
	if len(s.warnings) == 0 {
		return nil
	}
	out := make([]string, len(s.warnings))
	copy(out, s.warnings)
	s.warnings = nil
	return out
}
