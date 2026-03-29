// Purpose: Re-exports annotation store aliases into cmd package for draw-mode integration compatibility.
// Why: Avoids widespread refactors while annotation internals live in a dedicated package.
// Docs: docs/features/feature/annotated-screenshots/index.md

package main

import (
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/annotation"
)

// Type aliases — keep existing code compiling without changes.
type AnnotationRect = annotation.Rect
type Annotation = annotation.Annotation
type AnnotationDetail = annotation.Detail
type AnnotationSession = annotation.Session
type NamedAnnotationSession = annotation.NamedSession
type AnnotationStore = annotation.Store

// globalAnnotationStore is a legacy fallback store used by direct helper tests.
// Runtime HTTP/tool paths use a server-scoped store via Server.getAnnotationStore().
var globalAnnotationStore = annotation.NewStore(10 * time.Minute)

// NewAnnotationStore creates a new store (wrapper for tests).
func NewAnnotationStore(detailTTL time.Duration) *annotation.Store {
	return annotation.NewStore(detailTTL)
}
