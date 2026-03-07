// blast_radius_test.go — Tests for blast radius hook.

package hook

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunBlastRadius_EditExportedFunction(t *testing.T) {
	projectRoot := setupTestProject(t)
	input := Input{
		ToolName: "Edit",
		ToolInput: json.RawMessage(`{"file_path":"` + filepath.Join(projectRoot, "handlers", "handlers.go") +
			`","new_string":"func HandleUser(w http.ResponseWriter, r *http.Request) {\n\tw.WriteHeader(200)\n}"}`),
	}

	result := RunBlastRadius(input, projectRoot, "")
	if result == nil {
		t.Fatal("expected blast radius result for exported function edit")
	}
	ctx := result.FormatContext()
	if !strings.Contains(ctx, "Blast Radius") {
		t.Errorf("expected 'Blast Radius' in: %s", ctx)
	}
	if !strings.Contains(ctx, "imported by") {
		t.Errorf("expected 'imported by' in: %s", ctx)
	}
}

func TestRunBlastRadius_EditUnexported(t *testing.T) {
	projectRoot := setupTestProject(t)
	input := Input{
		ToolName: "Edit",
		ToolInput: json.RawMessage(`{"file_path":"` + filepath.Join(projectRoot, "db", "db.go") +
			`","new_string":"func getConnection() *sql.DB {\n\treturn nil\n}"}`),
	}

	result := RunBlastRadius(input, projectRoot, "")
	if result != nil {
		t.Errorf("expected nil result for unexported function, got: %s", result.FormatContext())
	}
}

func TestRunBlastRadius_ReadIgnored(t *testing.T) {
	projectRoot := setupTestProject(t)
	input := Input{
		ToolName:  "Read",
		ToolInput: json.RawMessage(`{"file_path":"` + filepath.Join(projectRoot, "handlers", "handlers.go") + `"}`),
	}

	result := RunBlastRadius(input, projectRoot, "")
	if result != nil {
		t.Errorf("expected nil result for Read tool, got: %s", result.FormatContext())
	}
}

func TestRunBlastRadius_SessionAware(t *testing.T) {
	projectRoot := setupTestProject(t)
	sessionDir := t.TempDir()

	// Pre-populate session with a read of routes.go.
	routesPath := filepath.Join(projectRoot, "routes", "routes.go")
	_ = AppendTouch(sessionDir, TouchEntry{
		Tool:   "Read",
		File:   routesPath,
		Action: "read",
	})

	input := Input{
		ToolName: "Edit",
		ToolInput: json.RawMessage(`{"file_path":"` + filepath.Join(projectRoot, "handlers", "handlers.go") +
			`","new_string":"func HandleUser(w http.ResponseWriter, r *http.Request) {}"}`),
	}

	result := RunBlastRadius(input, projectRoot, sessionDir)
	if result == nil {
		t.Fatal("expected blast radius result")
	}
	ctx := result.FormatContext()
	if !strings.Contains(ctx, "already read") {
		t.Errorf("expected 'already read' annotation for routes.go in: %s", ctx)
	}
}

func TestLooksExported_Go(t *testing.T) {
	tests := []struct {
		content  string
		exported bool
	}{
		{"func HandleUser() {}", true},
		{"type Config struct {}", true},
		{"var DefaultPort = 8080", true},
		{"func getConnection() {}", false},
		{"var count = 0", false},
	}
	for _, tt := range tests {
		got := looksExported(tt.content, "test.go")
		if got != tt.exported {
			t.Errorf("looksExported(%q, .go) = %v, want %v", tt.content, got, tt.exported)
		}
	}
}

func TestLooksExported_TS(t *testing.T) {
	tests := []struct {
		content  string
		exported bool
	}{
		{"export function getData() {}", true},
		{"export const API_URL = ''", true},
		{"export class UserService {}", true},
		{"function helper() {}", false},
		{"const x = 1", false},
	}
	for _, tt := range tests {
		got := looksExported(tt.content, "test.ts")
		if got != tt.exported {
			t.Errorf("looksExported(%q, .ts) = %v, want %v", tt.content, got, tt.exported)
		}
	}
}

func TestLooksExported_Python(t *testing.T) {
	tests := []struct {
		content  string
		exported bool
	}{
		{"def get_users():", true},
		{"class UserService:", true},
		{"def _internal():", false},
		{"class _Helper:", false},
	}
	for _, tt := range tests {
		got := looksExported(tt.content, "test.py")
		if got != tt.exported {
			t.Errorf("looksExported(%q, .py) = %v, want %v", tt.content, got, tt.exported)
		}
	}
}

func TestLooksExported_Rust(t *testing.T) {
	tests := []struct {
		content  string
		exported bool
	}{
		{"pub fn process() {}", true},
		{"pub struct Config {}", true},
		{"fn internal() {}", false},
	}
	for _, tt := range tests {
		got := looksExported(tt.content, "test.rs")
		if got != tt.exported {
			t.Errorf("looksExported(%q, .rs) = %v, want %v", tt.content, got, tt.exported)
		}
	}
}

func TestIsEditTool(t *testing.T) {
	edits := []string{"Edit", "Write", "write_file", "replace_in_file", "edit_file"}
	nonEdits := []string{"Read", "read_file", "Bash", "run_shell_command", ""}

	for _, name := range edits {
		if !isEditTool(name) {
			t.Errorf("isEditTool(%q) should be true", name)
		}
	}
	for _, name := range nonEdits {
		if isEditTool(name) {
			t.Errorf("isEditTool(%q) should be false", name)
		}
	}
}

func TestBuildImportGraph(t *testing.T) {
	projectRoot := setupTestProject(t)
	graph := buildImportGraph(projectRoot)
	if graph == nil {
		t.Fatal("expected non-nil graph")
	}

	// handlers/handlers.go imports db/db.go, so db.go should have handlers.go as an importer.
	// routes/routes.go imports handlers, so handlers/*.go should have routes.go as an importer.
	// main.go imports both handlers and routes.

	// Check that at least some importers were found.
	totalImporters := 0
	for _, importers := range graph.Importers {
		totalImporters += len(importers)
	}
	if totalImporters == 0 {
		t.Error("expected at least some import relationships in graph")
	}
}

// setupTestProject creates a temporary Go project with known import relationships.
func setupTestProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	dirs := []string{"handlers", "routes", "db"}
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	// go.mod
	writeFile(t, root, "go.mod", "module github.com/test/web\n\ngo 1.21\n")

	// .gasoline.json
	writeFile(t, root, ".gasoline.json", `{"code_standards":"standards.md","file_size_limit":800}`)

	// handlers/handlers.go — imports db package.
	writeFile(t, root, "handlers/handlers.go", `package handlers

import (
	"net/http"
	"github.com/test/web/db"
)

func HandleUser(w http.ResponseWriter, r *http.Request) {
	db.GetUsers()
}
`)

	// routes/routes.go — imports handlers package.
	writeFile(t, root, "routes/routes.go", `package routes

import (
	"net/http"
	"github.com/test/web/handlers"
)

func Setup() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/users", handlers.HandleUser)
	return mux
}
`)

	// db/db.go — no local imports.
	writeFile(t, root, "db/db.go", `package db

type User struct {
	ID   int
	Name string
}

func GetUsers() []User {
	return nil
}
`)

	// main.go — imports routes and handlers.
	writeFile(t, root, "main.go", `package main

import (
	"github.com/test/web/handlers"
	"github.com/test/web/routes"
)

func main() {
	r := routes.Setup()
	_ = r
	_ = handlers.HandleUser
}
`)

	return root
}

func writeFile(t *testing.T, root, relPath, content string) {
	t.Helper()
	full := filepath.Join(root, relPath)
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
