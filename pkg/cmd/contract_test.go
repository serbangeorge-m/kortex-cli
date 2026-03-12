/**********************************************************************
 * Copyright (C) 2026 Red Hat, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * SPDX-License-Identifier: Apache-2.0
 **********************************************************************/

package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	api "github.com/kortex-hub/kortex-cli-api/cli/go"
	"github.com/kortex-hub/kortex-cli/pkg/instances"
)

// ---------------------------------------------------------------------------
// helpers — execute a CLI command and return stdout
// ---------------------------------------------------------------------------

func execCmd(t *testing.T, args ...string) (stdout string, err error) {
	t.Helper()
	rootCmd := NewRootCmd()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs(args)
	err = rootCmd.Execute()
	return buf.String(), err
}

func mustExecCmd(t *testing.T, args ...string) string {
	t.Helper()
	out, err := execCmd(t, args...)
	if err != nil {
		t.Fatalf("command %v failed: %v", args, err)
	}
	return out
}

// ---------------------------------------------------------------------------
// 1. Lifecycle Tests
// ---------------------------------------------------------------------------

func TestContract_Lifecycle(t *testing.T) {
	t.Parallel()

	// FAILS IF: the ID returned by init cannot be used to remove the workspace,
	// or if list -o json does not reflect the current state after each operation.
	t.Run("full CRUD flow", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		sourcesDir := t.TempDir()

		// Step 1: init — capture workspace ID from stdout
		initOut := mustExecCmd(t,
			"--storage", storageDir, "init", sourcesDir, "--name", "test-ws")
		wsID := strings.TrimSpace(initOut)
		if wsID == "" {
			t.Fatal("init returned empty ID")
		}

		// Step 2: list -o json — verify the workspace appears
		listOut := mustExecCmd(t,
			"--storage", storageDir, "workspace", "list", "-o", "json")

		var listed api.WorkspacesList
		if err := json.Unmarshal([]byte(listOut), &listed); err != nil {
			t.Fatalf("failed to parse list JSON: %v", err)
		}
		if len(listed.Items) != 1 {
			t.Fatalf("expected 1 workspace, got %d", len(listed.Items))
		}
		if listed.Items[0].Id != wsID {
			t.Errorf("listed ID = %s, want %s", listed.Items[0].Id, wsID)
		}
		if listed.Items[0].Name != "test-ws" {
			t.Errorf("listed Name = %s, want test-ws", listed.Items[0].Name)
		}
		if listed.Items[0].Paths.Source != sourcesDir {
			t.Errorf("listed Source = %s, want %s", listed.Items[0].Paths.Source, sourcesDir)
		}

		// Step 3: remove — using the ID from init
		removeOut := mustExecCmd(t,
			"--storage", storageDir, "workspace", "remove", wsID)
		removedID := strings.TrimSpace(removeOut)
		if removedID != wsID {
			t.Errorf("remove echoed %s, want %s", removedID, wsID)
		}

		// Step 4: list -o json — verify empty
		listOut2 := mustExecCmd(t,
			"--storage", storageDir, "workspace", "list", "-o", "json")

		var listed2 api.WorkspacesList
		if err := json.Unmarshal([]byte(listOut2), &listed2); err != nil {
			t.Fatalf("failed to parse list JSON after remove: %v", err)
		}
		if len(listed2.Items) != 0 {
			t.Errorf("expected 0 workspaces after remove, got %d", len(listed2.Items))
		}
	})

	// FAILS IF: removing one workspace affects other workspaces in the same storage.
	t.Run("multi-workspace management", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		sources1 := t.TempDir()
		sources2 := t.TempDir()
		sources3 := t.TempDir()

		// Init three workspaces
		id1 := strings.TrimSpace(mustExecCmd(t,
			"--storage", storageDir, "init", sources1, "--name", "ws-one"))
		id2 := strings.TrimSpace(mustExecCmd(t,
			"--storage", storageDir, "init", sources2, "--name", "ws-two"))
		id3 := strings.TrimSpace(mustExecCmd(t,
			"--storage", storageDir, "init", sources3, "--name", "ws-three"))

		// Verify all three appear in list
		listOut := mustExecCmd(t,
			"--storage", storageDir, "workspace", "list", "-o", "json")
		var allWs api.WorkspacesList
		if err := json.Unmarshal([]byte(listOut), &allWs); err != nil {
			t.Fatalf("failed to parse list JSON: %v", err)
		}
		if len(allWs.Items) != 3 {
			t.Fatalf("expected 3 workspaces, got %d", len(allWs.Items))
		}

		// Remove the middle one
		mustExecCmd(t, "--storage", storageDir, "workspace", "remove", id2)

		// Verify remaining two
		listOut2 := mustExecCmd(t,
			"--storage", storageDir, "workspace", "list", "-o", "json")
		var remaining api.WorkspacesList
		if err := json.Unmarshal([]byte(listOut2), &remaining); err != nil {
			t.Fatalf("failed to parse list JSON: %v", err)
		}
		if len(remaining.Items) != 2 {
			t.Fatalf("expected 2 workspaces after removal, got %d", len(remaining.Items))
		}

		remainingIDs := map[string]bool{}
		for _, ws := range remaining.Items {
			remainingIDs[ws.Id] = true
		}

		if !remainingIDs[id1] {
			t.Errorf("workspace %s (ws-one) missing after removing ws-two", id1)
		}
		if remainingIDs[id2] {
			t.Errorf("workspace %s (ws-two) still present after removal", id2)
		}
		if !remainingIDs[id3] {
			t.Errorf("workspace %s (ws-three) missing after removing ws-two", id3)
		}
	})

	// FAILS IF: the list alias stops working as a proxy for workspace list.
	t.Run("aliases produce same results as full commands", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		sourcesDir := t.TempDir()

		wsID := strings.TrimSpace(mustExecCmd(t,
			"--storage", storageDir, "init", sourcesDir, "--name", "alias-test"))

		// list via alias
		aliasOut := mustExecCmd(t,
			"--storage", storageDir, "list", "-o", "json")
		// list via full command
		fullOut := mustExecCmd(t,
			"--storage", storageDir, "workspace", "list", "-o", "json")

		if aliasOut != fullOut {
			t.Errorf("alias and full command produced different output:\nalias: %s\nfull:  %s", aliasOut, fullOut)
		}

		// remove via alias
		removeAliasOut := mustExecCmd(t,
			"--storage", storageDir, "remove", wsID)
		if strings.TrimSpace(removeAliasOut) != wsID {
			t.Errorf("remove alias echoed %s, want %s", strings.TrimSpace(removeAliasOut), wsID)
		}
	})
}

// ---------------------------------------------------------------------------
// 2. JSON Schema Contract Tests
// ---------------------------------------------------------------------------

func TestContract_JSONSchema(t *testing.T) {
	t.Parallel()

	// FAILS IF: the top-level JSON key is renamed from "items" to something else.
	t.Run("top-level structure has items key", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		sourcesDir := t.TempDir()

		mustExecCmd(t, "--storage", storageDir, "init", sourcesDir, "--name", "schema-test")

		listOut := mustExecCmd(t,
			"--storage", storageDir, "workspace", "list", "-o", "json")

		var raw map[string]json.RawMessage
		if err := json.Unmarshal([]byte(listOut), &raw); err != nil {
			t.Fatalf("failed to parse JSON as map: %v", err)
		}

		if _, ok := raw["items"]; !ok {
			t.Errorf("top-level JSON missing 'items' key, got keys: %v", mapKeys(raw))
		}

		if len(raw) != 1 {
			t.Errorf("expected exactly 1 top-level key (items), got %d: %v", len(raw), mapKeys(raw))
		}
	})

	// FAILS IF: empty list returns null items instead of empty array.
	t.Run("empty list produces non-null items array", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()

		listOut := mustExecCmd(t,
			"--storage", storageDir, "workspace", "list", "-o", "json")

		var raw map[string]json.RawMessage
		if err := json.Unmarshal([]byte(listOut), &raw); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}

		itemsRaw := raw["items"]
		trimmed := strings.TrimSpace(string(itemsRaw))
		if trimmed == "null" {
			t.Error("items is null, expected empty array []")
		}

		var items []json.RawMessage
		if err := json.Unmarshal(itemsRaw, &items); err != nil {
			t.Fatalf("items is not an array: %v", err)
		}
		if len(items) != 0 {
			t.Errorf("expected 0 items, got %d", len(items))
		}
	})

	// FAILS IF: workspace JSON fields are renamed, added, or removed.
	t.Run("workspace object has exact field set", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		sourcesDir := t.TempDir()

		mustExecCmd(t, "--storage", storageDir, "init", sourcesDir, "--name", "fields-test")

		listOut := mustExecCmd(t,
			"--storage", storageDir, "workspace", "list", "-o", "json")

		var rawTop map[string]json.RawMessage
		if err := json.Unmarshal([]byte(listOut), &rawTop); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}

		var rawItems []map[string]json.RawMessage
		if err := json.Unmarshal(rawTop["items"], &rawItems); err != nil {
			t.Fatalf("failed to parse items: %v", err)
		}

		if len(rawItems) != 1 {
			t.Fatalf("expected 1 item, got %d", len(rawItems))
		}

		ws := rawItems[0]
		expectedKeys := map[string]bool{"id": true, "name": true, "paths": true}
		for key := range ws {
			if !expectedKeys[key] {
				t.Errorf("unexpected workspace field: %q", key)
			}
		}
		for key := range expectedKeys {
			if _, ok := ws[key]; !ok {
				t.Errorf("missing required workspace field: %q", key)
			}
		}

		// Verify paths sub-object fields
		var paths map[string]json.RawMessage
		if err := json.Unmarshal(ws["paths"], &paths); err != nil {
			t.Fatalf("failed to parse paths: %v", err)
		}

		expectedPathKeys := map[string]bool{"source": true, "configuration": true}
		for key := range paths {
			if !expectedPathKeys[key] {
				t.Errorf("unexpected paths field: %q", key)
			}
		}
		for key := range expectedPathKeys {
			if _, ok := paths[key]; !ok {
				t.Errorf("missing required paths field: %q", key)
			}
		}
	})

	// FAILS IF: JSON output changes between identical invocations.
	t.Run("JSON output is deterministic", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		sourcesDir := t.TempDir()

		mustExecCmd(t, "--storage", storageDir, "init", sourcesDir, "--name", "determ-test")

		out1 := mustExecCmd(t,
			"--storage", storageDir, "workspace", "list", "-o", "json")
		out2 := mustExecCmd(t,
			"--storage", storageDir, "workspace", "list", "-o", "json")

		if out1 != out2 {
			t.Errorf("JSON output is not deterministic:\nfirst:  %s\nsecond: %s", out1, out2)
		}
	})

	// FAILS IF: typed and untyped deserialization produce inconsistent results.
	t.Run("typed and untyped parsing agree", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		sourcesDir := t.TempDir()

		mustExecCmd(t, "--storage", storageDir, "init", sourcesDir, "--name", "parse-test")

		listOut := mustExecCmd(t,
			"--storage", storageDir, "workspace", "list", "-o", "json")

		// Typed parse
		var typed api.WorkspacesList
		if err := json.Unmarshal([]byte(listOut), &typed); err != nil {
			t.Fatalf("typed parse failed: %v", err)
		}

		// Untyped parse
		var untyped map[string][]map[string]interface{}
		if err := json.Unmarshal([]byte(listOut), &untyped); err != nil {
			t.Fatalf("untyped parse failed: %v", err)
		}

		items := untyped["items"]
		if len(items) != len(typed.Items) {
			t.Fatalf("item count mismatch: typed=%d, untyped=%d",
				len(typed.Items), len(items))
		}

		for i, ws := range typed.Items {
			raw := items[i]
			if raw["id"] != ws.Id {
				t.Errorf("item[%d] id mismatch: typed=%s, untyped=%v", i, ws.Id, raw["id"])
			}
			if raw["name"] != ws.Name {
				t.Errorf("item[%d] name mismatch: typed=%s, untyped=%v", i, ws.Name, raw["name"])
			}
			pathsRaw, ok := raw["paths"].(map[string]interface{})
			if !ok {
				t.Fatalf("item[%d] paths is not a map", i)
			}
			if pathsRaw["source"] != ws.Paths.Source {
				t.Errorf("item[%d] paths.source mismatch", i)
			}
			if pathsRaw["configuration"] != ws.Paths.Configuration {
				t.Errorf("item[%d] paths.configuration mismatch", i)
			}
		}
	})
}

// ---------------------------------------------------------------------------
// 3. Output Format Contract Tests
// ---------------------------------------------------------------------------

func TestContract_OutputFormat(t *testing.T) {
	t.Parallel()

	// FAILS IF: init default output contains anything other than the workspace ID.
	t.Run("init outputs exactly one line with ID", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		sourcesDir := t.TempDir()

		out := mustExecCmd(t,
			"--storage", storageDir, "init", sourcesDir, "--name", "format-test")

		lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
		if len(lines) != 1 {
			t.Errorf("expected exactly 1 line, got %d: %q", len(lines), out)
		}

		id := lines[0]
		if id == "" {
			t.Error("init output line is empty")
		}
		if strings.ContainsAny(id, " \t\r") {
			t.Errorf("init output contains whitespace: %q", id)
		}

		// Verify this is actually the stored workspace ID
		manager, err := instances.NewManager(storageDir)
		if err != nil {
			t.Fatalf("failed to create manager: %v", err)
		}
		inst, err := manager.Get(id)
		if err != nil {
			t.Fatalf("could not retrieve workspace by init output ID %q: %v", id, err)
		}
		if inst.GetName() != "format-test" {
			t.Errorf("retrieved workspace name = %s, want format-test", inst.GetName())
		}
	})

	// FAILS IF: init --verbose stops printing the structured labels.
	t.Run("init verbose outputs structured labels", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		sourcesDir := t.TempDir()

		out := mustExecCmd(t,
			"--storage", storageDir, "init", sourcesDir, "--name", "verbose-test", "--verbose")

		requiredLabels := []string{
			"ID:",
			"Name:",
			"Sources directory:",
			"Configuration directory:",
		}
		for _, label := range requiredLabels {
			if !strings.Contains(out, label) {
				t.Errorf("verbose output missing label %q, got: %s", label, out)
			}
		}
	})

	// FAILS IF: remove stops echoing the removed workspace ID.
	t.Run("remove outputs exactly one line with ID", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		sourcesDir := t.TempDir()

		wsID := strings.TrimSpace(mustExecCmd(t,
			"--storage", storageDir, "init", sourcesDir))

		out := mustExecCmd(t,
			"--storage", storageDir, "workspace", "remove", wsID)

		lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
		if len(lines) != 1 {
			t.Errorf("expected exactly 1 line, got %d: %q", len(lines), out)
		}
		if lines[0] != wsID {
			t.Errorf("remove output = %q, want %q", lines[0], wsID)
		}
	})

	// FAILS IF: error details are not returned via Execute() error return.
	t.Run("errors returned via Execute error", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()

		_, err := execCmd(t,
			"--storage", storageDir, "workspace", "remove", "nonexistent-id")
		if err == nil {
			t.Fatal("expected error for nonexistent workspace, got nil")
		}

		if !strings.Contains(err.Error(), "workspace not found") {
			t.Errorf("error = %v, want it to contain 'workspace not found'", err)
		}

		// Verify init also returns errors through Execute
		_, err = execCmd(t,
			"--storage", storageDir, "init", filepath.Join(storageDir, "does-not-exist"))
		if err == nil {
			t.Fatal("expected error for nonexistent sources dir, got nil")
		}
	})
}

// ---------------------------------------------------------------------------
// 4. Storage Resilience Tests
// ---------------------------------------------------------------------------

func TestContract_StorageResilience(t *testing.T) {
	t.Parallel()

	// FAILS IF: corrupted storage causes a panic instead of a clean error.
	t.Run("corrupted storage returns error", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		storageFile := filepath.Join(storageDir, "instances.json")
		if err := os.WriteFile(storageFile, []byte("{invalid json!!!"), 0644); err != nil {
			t.Fatalf("failed to write corrupted file: %v", err)
		}

		_, err := execCmd(t,
			"--storage", storageDir, "workspace", "list", "-o", "json")
		if err == nil {
			t.Error("expected error with corrupted storage, got nil")
		}
	})

	// FAILS IF: an empty storage file causes a crash instead of returning empty list.
	t.Run("empty storage file returns empty list", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		storageFile := filepath.Join(storageDir, "instances.json")
		if err := os.WriteFile(storageFile, []byte{}, 0644); err != nil {
			t.Fatalf("failed to write empty file: %v", err)
		}

		listOut := mustExecCmd(t,
			"--storage", storageDir, "workspace", "list", "-o", "json")

		var listed api.WorkspacesList
		if err := json.Unmarshal([]byte(listOut), &listed); err != nil {
			t.Fatalf("failed to parse JSON from empty storage: %v", err)
		}
		if len(listed.Items) != 0 {
			t.Errorf("expected 0 items from empty storage, got %d", len(listed.Items))
		}
	})

	// FAILS IF: init into empty storage file fails instead of creating the workspace.
	t.Run("init works with empty storage file", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		storageFile := filepath.Join(storageDir, "instances.json")
		if err := os.WriteFile(storageFile, []byte{}, 0644); err != nil {
			t.Fatalf("failed to write empty file: %v", err)
		}

		sourcesDir := t.TempDir()
		out := mustExecCmd(t,
			"--storage", storageDir, "init", sourcesDir, "--name", "empty-storage-test")

		wsID := strings.TrimSpace(out)
		if wsID == "" {
			t.Fatal("init returned empty ID with empty storage file")
		}

		// Verify it was persisted
		listOut := mustExecCmd(t,
			"--storage", storageDir, "workspace", "list", "-o", "json")
		var listed api.WorkspacesList
		if err := json.Unmarshal([]byte(listOut), &listed); err != nil {
			t.Fatalf("failed to parse list JSON: %v", err)
		}
		if len(listed.Items) != 1 {
			t.Fatalf("expected 1 workspace, got %d", len(listed.Items))
		}
		if listed.Items[0].Id != wsID {
			t.Errorf("listed ID = %s, want %s", listed.Items[0].Id, wsID)
		}
	})

	// FAILS IF: two storage directories share state.
	t.Run("isolated storage paths do not interfere", func(t *testing.T) {
		t.Parallel()

		storageA := t.TempDir()
		storageB := t.TempDir()
		sourcesDir := t.TempDir()

		// Init in storage A
		mustExecCmd(t, "--storage", storageA, "init", sourcesDir, "--name", "ws-in-a")

		// Storage B should still be empty
		listOut := mustExecCmd(t,
			"--storage", storageB, "workspace", "list", "-o", "json")
		var listed api.WorkspacesList
		if err := json.Unmarshal([]byte(listOut), &listed); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}
		if len(listed.Items) != 0 {
			t.Errorf("storage B has %d workspaces, expected 0", len(listed.Items))
		}

		// Storage A should have one
		listOutA := mustExecCmd(t,
			"--storage", storageA, "workspace", "list", "-o", "json")
		var listedA api.WorkspacesList
		if err := json.Unmarshal([]byte(listOutA), &listedA); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}
		if len(listedA.Items) != 1 {
			t.Errorf("storage A has %d workspaces, expected 1", len(listedA.Items))
		}
	})

	// FAILS IF: workspace data is not persisted between separate command invocations.
	t.Run("storage persists across command invocations", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		sourcesDir := t.TempDir()

		// Init with one root command instance
		wsID := strings.TrimSpace(mustExecCmd(t,
			"--storage", storageDir, "init", sourcesDir, "--name", "persist-test"))

		// List with a completely fresh root command instance
		listOut := mustExecCmd(t,
			"--storage", storageDir, "workspace", "list", "-o", "json")

		var listed api.WorkspacesList
		if err := json.Unmarshal([]byte(listOut), &listed); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}
		if len(listed.Items) != 1 {
			t.Fatalf("expected 1 workspace, got %d", len(listed.Items))
		}
		if listed.Items[0].Id != wsID {
			t.Errorf("persisted ID = %s, want %s", listed.Items[0].Id, wsID)
		}
		if listed.Items[0].Name != "persist-test" {
			t.Errorf("persisted Name = %s, want persist-test", listed.Items[0].Name)
		}
	})
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func mapKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
