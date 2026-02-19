// annotation_store.go — Type aliases bridging internal/annotation into package main.
// The annotation store implementation lives in internal/annotation.
package main

import (
	"time"

	"github.com/dev-console/dev-console/internal/annotation"
)

// Type aliases — keep existing code compiling without changes.
type AnnotationRect = annotation.Rect
type Annotation = annotation.Annotation
type AnnotationDetail = annotation.Detail
type AnnotationSession = annotation.Session
type NamedAnnotationSession = annotation.NamedSession
type AnnotationStore = annotation.Store

// Constant aliases for tests.
const maxSessions = annotation.MaxSessions
const maxNamedSessions = annotation.MaxNamedSessions
const maxDetails = annotation.MaxDetails

// globalAnnotationStore is the shared annotation store used by both HTTP routes and tool handlers.
var globalAnnotationStore = annotation.NewStore(10 * time.Minute)

// NewAnnotationStore creates a new store (wrapper for tests).
func NewAnnotationStore(detailTTL time.Duration) *annotation.Store {
	return annotation.NewStore(detailTTL)
}
