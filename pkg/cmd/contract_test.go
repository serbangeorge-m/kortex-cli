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
// helpers — execute a CLI command and return stdout + stderr
// ---------------------------------------------------------------------------

func execCmd(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	rootCmd := NewRootCmd()
	outBuf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(outBuf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs(args)
	err = rootCmd.Execute()
	return outBuf.String(), errBuf.String(), err
}

func mustExecCmd(t *testing.T, args ...string) string {
	t.Helper()
	out, _, err := execCmd(t, args...)
	if err != nil {
		t.Fatalf("command %v failed: %v", args, err)
	}
	return out
}

func mustParseWorkspacesList(t *testing.T, jsonOutput string) api.WorkspacesList {
	t.Helper()
	var result api.WorkspacesList
	if err := json.Unmarshal([]byte(jsonOutput), &result); err != nil {
		t.Fatalf("failed to parse workspace list JSON: %v\nOutput: %s", err, jsonOutput)
	}
	return result
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

		listed := mustParseWorkspacesList(t, listOut)
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

		listed2 := mustParseWorkspacesList(t, listOut2)
		if len(listed2.Items) != 0 {
			t.Errorf("expected 0 workspaces after remove, got %d", len(listed2.Items))
		}
	})

	// FAILS IF: init on the same source directory stops creating separate workspaces.
	// This is intentional: the same source directory can have multiple workspaces
	// with different configurations (e.g., different agent setups, MCP servers,
	// or skills). Workspace identity is the (source, config) pair, not source alone.
	t.Run("init same directory twice creates separate workspaces", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		sourcesDir := t.TempDir()

		// First init
		id1 := strings.TrimSpace(mustExecCmd(t,
			"--storage", storageDir, "init", sourcesDir, "--name", "dup-test"))

		// Second init with same source directory
		id2 := strings.TrimSpace(mustExecCmd(t,
			"--storage", storageDir, "init", sourcesDir, "--name", "dup-test"))

		// Currently creates two distinct workspaces
		if id1 == id2 {
			t.Errorf("expected different IDs for duplicate init, got same: %s", id1)
		}

		listOut := mustExecCmd(t,
			"--storage", storageDir, "workspace", "list", "-o", "json")
		listed := mustParseWorkspacesList(t, listOut)
		if len(listed.Items) != 2 {
			t.Errorf("expected 2 workspaces after duplicate init, got %d", len(listed.Items))
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
		allWs := mustParseWorkspacesList(t, listOut)
		if len(allWs.Items) != 3 {
			t.Fatalf("expected 3 workspaces, got %d", len(allWs.Items))
		}

		// Remove the middle one
		mustExecCmd(t, "--storage", storageDir, "workspace", "remove", id2)

		// Verify remaining two
		listOut2 := mustExecCmd(t,
			"--storage", storageDir, "workspace", "list", "-o", "json")
		remaining := mustParseWorkspacesList(t, listOut2)
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
		typed := mustParseWorkspacesList(t, listOut)

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

	// FAILS IF: version command stops producing output or fails.
	// CI scripts and container tests use 'kortex-cli version' to verify the binary works.
	t.Run("version outputs non-empty string", func(t *testing.T) {
		t.Parallel()

		out, stderr, err := execCmd(t, "version")
		if err != nil {
			t.Fatalf("version command failed: %v", err)
		}
		if stderr != "" {
			t.Errorf("version produced stderr: %q", stderr)
		}

		trimmed := strings.TrimSpace(out)
		if trimmed == "" {
			t.Error("version output is empty")
		}
		if !strings.Contains(trimmed, "kortex-cli") {
			t.Errorf("version output missing 'kortex-cli': %q", trimmed)
		}
	})

	// FAILS IF: error details are not returned via Execute() error return.
	t.Run("errors returned via Execute error", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()

		_, _, err := execCmd(t,
			"--storage", storageDir, "workspace", "remove", "nonexistent-id")
		if err == nil {
			t.Fatal("expected error for nonexistent workspace, got nil")
		}

		if !strings.Contains(err.Error(), "workspace not found") {
			t.Errorf("error = %v, want it to contain 'workspace not found'", err)
		}

		// Verify init also returns errors through Execute
		_, _, err = execCmd(t,
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
		storageFile := filepath.Join(storageDir, instances.DefaultStorageFileName)
		if err := os.WriteFile(storageFile, []byte("{invalid json!!!"), 0644); err != nil {
			t.Fatalf("failed to write corrupted file: %v", err)
		}

		_, _, err := execCmd(t,
			"--storage", storageDir, "workspace", "list", "-o", "json")
		if err == nil {
			t.Error("expected error with corrupted storage, got nil")
		}
	})

	// FAILS IF: corrupted storage causes a panic in text output mode.
	t.Run("corrupted storage returns error in text mode", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		storageFile := filepath.Join(storageDir, instances.DefaultStorageFileName)
		if err := os.WriteFile(storageFile, []byte("{invalid json!!!"), 0644); err != nil {
			t.Fatalf("failed to write corrupted file: %v", err)
		}

		_, _, err := execCmd(t,
			"--storage", storageDir, "workspace", "list")
		if err == nil {
			t.Error("expected error with corrupted storage in text mode, got nil")
		}
	})

	// FAILS IF: an empty storage file causes a crash instead of returning empty list.
	t.Run("empty storage file returns empty list", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		storageFile := filepath.Join(storageDir, instances.DefaultStorageFileName)
		if err := os.WriteFile(storageFile, []byte{}, 0644); err != nil {
			t.Fatalf("failed to write empty file: %v", err)
		}

		listOut := mustExecCmd(t,
			"--storage", storageDir, "workspace", "list", "-o", "json")

		listed := mustParseWorkspacesList(t, listOut)
		if len(listed.Items) != 0 {
			t.Errorf("expected 0 items from empty storage, got %d", len(listed.Items))
		}
	})

	// FAILS IF: init into empty storage file fails instead of creating the workspace.
	t.Run("init works with empty storage file", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		storageFile := filepath.Join(storageDir, instances.DefaultStorageFileName)
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
		listed := mustParseWorkspacesList(t, listOut)
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
		listed := mustParseWorkspacesList(t, listOut)
		if len(listed.Items) != 0 {
			t.Errorf("storage B has %d workspaces, expected 0", len(listed.Items))
		}

		// Storage A should have one
		listOutA := mustExecCmd(t,
			"--storage", storageA, "workspace", "list", "-o", "json")
		listedA := mustParseWorkspacesList(t, listOutA)
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

		listed := mustParseWorkspacesList(t, listOut)
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
// 5. Help Text Stability Tests
// ---------------------------------------------------------------------------

func TestContract_HelpText(t *testing.T) {
	t.Parallel()

	// FAILS IF: a top-level command is renamed or removed.
	t.Run("root help lists all commands", func(t *testing.T) {
		t.Parallel()

		out := mustExecCmd(t, "--help")

		expectedCommands := []string{"init", "workspace", "version", "list", "remove"}
		for _, cmd := range expectedCommands {
			if !strings.Contains(out, cmd) {
				t.Errorf("root help missing command %q, got:\n%s", cmd, out)
			}
		}
	})

	// FAILS IF: an init flag is renamed or removed.
	t.Run("init help lists all flags", func(t *testing.T) {
		t.Parallel()

		out := mustExecCmd(t, "init", "--help")

		expectedFlags := []string{"--name", "--verbose", "--workspace-configuration", "--storage"}
		for _, flag := range expectedFlags {
			if !strings.Contains(out, flag) {
				t.Errorf("init help missing flag %q, got:\n%s", flag, out)
			}
		}
	})

	// FAILS IF: a workspace subcommand is renamed or removed.
	t.Run("workspace help lists subcommands", func(t *testing.T) {
		t.Parallel()

		out := mustExecCmd(t, "workspace", "--help")

		expectedSubs := []string{"list", "remove"}
		for _, sub := range expectedSubs {
			if !strings.Contains(out, sub) {
				t.Errorf("workspace help missing subcommand %q, got:\n%s", sub, out)
			}
		}
	})

	// FAILS IF: the --output flag is renamed or removed from workspace list.
	t.Run("workspace list help lists output flag", func(t *testing.T) {
		t.Parallel()

		out := mustExecCmd(t, "workspace", "list", "--help")

		if !strings.Contains(out, "--output") {
			t.Errorf("workspace list help missing --output flag, got:\n%s", out)
		}
	})

	// FAILS IF: workspace remove stops showing the ID argument in usage.
	t.Run("workspace remove help shows ID argument", func(t *testing.T) {
		t.Parallel()

		out := mustExecCmd(t, "workspace", "remove", "--help")

		if !strings.Contains(out, "ID") {
			t.Errorf("workspace remove help missing ID argument, got:\n%s", out)
		}
	})
}

// ---------------------------------------------------------------------------
// 6. Stderr Content Tests
// ---------------------------------------------------------------------------

func TestContract_Stderr(t *testing.T) {
	t.Parallel()

	// FAILS IF: error for nonexistent workspace is not written to stderr.
	t.Run("remove nonexistent writes to stderr", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()

		_, stderr, _ := execCmd(t,
			"--storage", storageDir, "workspace", "remove", "nonexistent-id")

		if !strings.Contains(stderr, "workspace not found") {
			t.Errorf("stderr = %q, want it to contain 'workspace not found'", stderr)
		}
	})

	// FAILS IF: error for nonexistent sources path is not written to stderr.
	t.Run("init nonexistent path writes to stderr", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()

		_, stderr, _ := execCmd(t,
			"--storage", storageDir, "init", filepath.Join(storageDir, "does-not-exist"))

		if !strings.Contains(stderr, "sources directory does not exist") {
			t.Errorf("stderr = %q, want it to contain 'sources directory does not exist'", stderr)
		}
	})

	// FAILS IF: error for invalid output format is not written to stderr.
	t.Run("invalid output format writes to stderr", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()

		_, stderr, _ := execCmd(t,
			"--storage", storageDir, "workspace", "list", "-o", "xml")

		if !strings.Contains(stderr, "unsupported output format") {
			t.Errorf("stderr = %q, want it to contain 'unsupported output format'", stderr)
		}
	})

	// FAILS IF: successful commands produce unexpected output on stderr.
	t.Run("successful commands produce empty stderr", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		sourcesDir := t.TempDir()

		_, stderr, err := execCmd(t,
			"--storage", storageDir, "init", sourcesDir, "--name", "stderr-test")
		if err != nil {
			t.Fatalf("init failed: %v", err)
		}
		if stderr != "" {
			t.Errorf("init produced stderr output: %q", stderr)
		}

		_, stderr, err = execCmd(t,
			"--storage", storageDir, "workspace", "list", "-o", "json")
		if err != nil {
			t.Fatalf("list failed: %v", err)
		}
		if stderr != "" {
			t.Errorf("list produced stderr output: %q", stderr)
		}
	})
}

// ---------------------------------------------------------------------------
// 7. Special Character Tests
// ---------------------------------------------------------------------------

func TestContract_SpecialCharacters(t *testing.T) {
	t.Parallel()

	// FAILS IF: workspace names with spaces are truncated or mangled.
	t.Run("workspace name with spaces", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		sourcesDir := t.TempDir()
		name := "My Project Name"

		mustExecCmd(t,
			"--storage", storageDir, "init", sourcesDir, "--name", name)

		listOut := mustExecCmd(t,
			"--storage", storageDir, "workspace", "list", "-o", "json")

		listed := mustParseWorkspacesList(t, listOut)
		if len(listed.Items) != 1 {
			t.Fatalf("expected 1 workspace, got %d", len(listed.Items))
		}
		if listed.Items[0].Name != name {
			t.Errorf("name = %q, want %q", listed.Items[0].Name, name)
		}
	})

	// FAILS IF: workspace names with unicode are corrupted or rejected.
	t.Run("workspace name with unicode", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		sourcesDir := t.TempDir()
		name := "projekt-übersicht"

		mustExecCmd(t,
			"--storage", storageDir, "init", sourcesDir, "--name", name)

		listOut := mustExecCmd(t,
			"--storage", storageDir, "workspace", "list", "-o", "json")

		listed := mustParseWorkspacesList(t, listOut)
		if len(listed.Items) != 1 {
			t.Fatalf("expected 1 workspace, got %d", len(listed.Items))
		}
		if listed.Items[0].Name != name {
			t.Errorf("name = %q, want %q", listed.Items[0].Name, name)
		}
	})

	// FAILS IF: source directories with spaces cause init to fail or paths to be mangled.
	t.Run("source directory with spaces", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		sourcesDir := filepath.Join(t.TempDir(), "my project")
		if err := os.MkdirAll(sourcesDir, 0755); err != nil {
			t.Fatalf("failed to create dir with spaces: %v", err)
		}

		mustExecCmd(t,
			"--storage", storageDir, "init", sourcesDir, "--name", "space-path")

		listOut := mustExecCmd(t,
			"--storage", storageDir, "workspace", "list", "-o", "json")

		listed := mustParseWorkspacesList(t, listOut)
		if len(listed.Items) != 1 {
			t.Fatalf("expected 1 workspace, got %d", len(listed.Items))
		}
		if listed.Items[0].Paths.Source != sourcesDir {
			t.Errorf("source = %q, want %q", listed.Items[0].Paths.Source, sourcesDir)
		}
	})

	// FAILS IF: source directories with unicode cause init to fail or paths to be corrupted.
	t.Run("source directory with unicode", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		sourcesDir := filepath.Join(t.TempDir(), "проект")
		if err := os.MkdirAll(sourcesDir, 0755); err != nil {
			t.Fatalf("failed to create dir with unicode: %v", err)
		}

		mustExecCmd(t,
			"--storage", storageDir, "init", sourcesDir, "--name", "unicode-path")

		listOut := mustExecCmd(t,
			"--storage", storageDir, "workspace", "list", "-o", "json")

		listed := mustParseWorkspacesList(t, listOut)
		if len(listed.Items) != 1 {
			t.Fatalf("expected 1 workspace, got %d", len(listed.Items))
		}
		if listed.Items[0].Paths.Source != sourcesDir {
			t.Errorf("source = %q, want %q", listed.Items[0].Paths.Source, sourcesDir)
		}
	})
}

// ---------------------------------------------------------------------------
// 8. Unknown Command and Flag Tests
// ---------------------------------------------------------------------------

func TestContract_UnknownInputs(t *testing.T) {
	t.Parallel()

	// FAILS IF: an unknown command causes a panic or produces no output.
	// Cobra's default behavior shows help text for unknown commands (exit 0).
	t.Run("unknown command shows help text", func(t *testing.T) {
		t.Parallel()

		out, _, err := execCmd(t, "foobar")
		if err != nil {
			t.Fatalf("unexpected error for unknown command: %v", err)
		}
		// Cobra shows usage/help when an unknown command is given
		if !strings.Contains(out, "Usage:") {
			t.Errorf("expected help text with 'Usage:', got: %q", out)
		}
	})

	// FAILS IF: an unknown flag causes a panic or silent exit instead of a helpful error.
	t.Run("unknown flag returns error", func(t *testing.T) {
		t.Parallel()

		_, stderr, err := execCmd(t, "init", "--nonexistent-flag")
		if err == nil {
			t.Fatal("expected error for unknown flag, got nil")
		}
		if !strings.Contains(stderr, "unknown flag") {
			t.Errorf("stderr = %q, want it to contain 'unknown flag'", stderr)
		}
	})

	// FAILS IF: extra arguments to a no-args command cause a panic instead of an error.
	t.Run("extra arguments to version returns error", func(t *testing.T) {
		t.Parallel()

		_, _, err := execCmd(t, "version", "extra-arg")
		if err == nil {
			t.Fatal("expected error for extra arguments to version, got nil")
		}
	})
}

// ---------------------------------------------------------------------------
// 9. CLI Standards Tests
// ---------------------------------------------------------------------------

func TestContract_CLIStandards(t *testing.T) {
	t.Parallel()

	// -----------------------------------------------------------------------
	// 9a. Exit codes: err == nil means exit 0, err != nil means exit 1.
	// main.go maps these via os.Exit(1) on error.
	// -----------------------------------------------------------------------

	// FAILS IF: a successful command returns a non-nil error (would cause exit 1).
	t.Run("successful commands return nil error", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		sourcesDir := t.TempDir()

		// init
		_, _, err := execCmd(t,
			"--storage", storageDir, "init", sourcesDir, "--name", "exit-test")
		if err != nil {
			t.Errorf("init returned error (exit 1): %v", err)
		}

		// list
		_, _, err = execCmd(t,
			"--storage", storageDir, "workspace", "list")
		if err != nil {
			t.Errorf("list returned error (exit 1): %v", err)
		}

		// list -o json
		_, _, err = execCmd(t,
			"--storage", storageDir, "workspace", "list", "-o", "json")
		if err != nil {
			t.Errorf("list -o json returned error (exit 1): %v", err)
		}

		// version
		_, _, err = execCmd(t, "version")
		if err != nil {
			t.Errorf("version returned error (exit 1): %v", err)
		}

		// --help
		_, _, err = execCmd(t, "--help")
		if err != nil {
			t.Errorf("--help returned error (exit 1): %v", err)
		}
	})

	// FAILS IF: a failed command returns nil error (would cause exit 0).
	t.Run("failed commands return non-nil error", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()

		// remove nonexistent
		_, _, err := execCmd(t,
			"--storage", storageDir, "workspace", "remove", "nonexistent-id")
		if err == nil {
			t.Error("remove nonexistent returned nil error (exit 0), want error (exit 1)")
		}

		// init nonexistent path
		_, _, err = execCmd(t,
			"--storage", storageDir, "init", filepath.Join(storageDir, "no-such-dir"))
		if err == nil {
			t.Error("init nonexistent path returned nil error (exit 0), want error (exit 1)")
		}

		// invalid output format
		_, _, err = execCmd(t,
			"--storage", storageDir, "workspace", "list", "-o", "xml")
		if err == nil {
			t.Error("list -o xml returned nil error (exit 0), want error (exit 1)")
		}
	})

	// -----------------------------------------------------------------------
	// 9b. --help works on every subcommand without error.
	// -----------------------------------------------------------------------

	// FAILS IF: --help on any subcommand returns an error or produces empty output.
	t.Run("help flag works on all subcommands", func(t *testing.T) {
		t.Parallel()

		subcommands := [][]string{
			{"--help"},
			{"version", "--help"},
			{"init", "--help"},
			{"workspace", "--help"},
			{"workspace", "list", "--help"},
			{"workspace", "remove", "--help"},
			{"list", "--help"},
			{"remove", "--help"},
		}

		for _, args := range subcommands {
			cmdName := strings.Join(args, " ")
			out, stderr, err := execCmd(t, args...)
			if err != nil {
				t.Errorf("%s returned error: %v", cmdName, err)
			}
			if strings.TrimSpace(out) == "" {
				t.Errorf("%s produced empty stdout", cmdName)
			}
			if stderr != "" {
				t.Errorf("%s produced stderr: %q", cmdName, stderr)
			}
		}
	})

	// -----------------------------------------------------------------------
	// 9c. Stderr for errors, stdout for data — never mixed.
	// -----------------------------------------------------------------------

	// FAILS IF: successful data commands leak anything to stderr.
	t.Run("data commands write only to stdout", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		sourcesDir := t.TempDir()

		// init — stdout is the ID, stderr must be empty
		_, stderr, err := execCmd(t,
			"--storage", storageDir, "init", sourcesDir, "--name", "stdout-test")
		if err != nil {
			t.Fatalf("init failed: %v", err)
		}
		if stderr != "" {
			t.Errorf("init leaked to stderr: %q", stderr)
		}

		// init --verbose — stdout is the verbose output, stderr must be empty
		sourcesDir2 := t.TempDir()
		_, stderr, err = execCmd(t,
			"--storage", storageDir, "init", sourcesDir2, "--name", "stdout-verbose", "--verbose")
		if err != nil {
			t.Fatalf("init --verbose failed: %v", err)
		}
		if stderr != "" {
			t.Errorf("init --verbose leaked to stderr: %q", stderr)
		}

		// list -o json — stdout is JSON, stderr must be empty
		_, stderr, err = execCmd(t,
			"--storage", storageDir, "workspace", "list", "-o", "json")
		if err != nil {
			t.Fatalf("list -o json failed: %v", err)
		}
		if stderr != "" {
			t.Errorf("list -o json leaked to stderr: %q", stderr)
		}

		// list (text) — stdout is text, stderr must be empty
		_, stderr, err = execCmd(t,
			"--storage", storageDir, "workspace", "list")
		if err != nil {
			t.Fatalf("list failed: %v", err)
		}
		if stderr != "" {
			t.Errorf("list leaked to stderr: %q", stderr)
		}

		// version — stdout is version string, stderr must be empty
		_, stderr, err = execCmd(t, "version")
		if err != nil {
			t.Fatalf("version failed: %v", err)
		}
		if stderr != "" {
			t.Errorf("version leaked to stderr: %q", stderr)
		}
	})

	// FAILS IF: error commands produce empty stderr (error details must reach the user).
	// Note: Cobra writes usage text to stdout on error by default — this is standard
	// Cobra behavior and not a violation. We only assert that stderr is non-empty.
	t.Run("error commands always write to stderr", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()

		// remove nonexistent
		_, stderr, _ := execCmd(t,
			"--storage", storageDir, "workspace", "remove", "nonexistent-id")
		if stderr == "" {
			t.Error("remove nonexistent produced empty stderr, want error message")
		}

		// init nonexistent path
		_, stderr, _ = execCmd(t,
			"--storage", storageDir, "init", filepath.Join(storageDir, "no-such-dir"))
		if stderr == "" {
			t.Error("init nonexistent produced empty stderr, want error message")
		}
	})

	// -----------------------------------------------------------------------
	// 9d. Non-verbose mode never mixes human text into stdout.
	// -----------------------------------------------------------------------

	// FAILS IF: init default (non-verbose) output contains anything other than
	// machine-parseable data (the workspace ID).
	t.Run("non-verbose init is machine parseable", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		sourcesDir := t.TempDir()

		out, _, err := execCmd(t,
			"--storage", storageDir, "init", sourcesDir, "--name", "machine-test")
		if err != nil {
			t.Fatalf("init failed: %v", err)
		}

		lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
		if len(lines) != 1 {
			t.Errorf("non-verbose init produced %d lines, want 1: %q", len(lines), out)
		}

		// The single line should be a hex ID — no colons, no labels, no prose
		id := strings.TrimSpace(lines[0])
		if strings.ContainsAny(id, " \t:") {
			t.Errorf("non-verbose init output contains human text: %q", id)
		}
	})

	// FAILS IF: list -o json output contains anything outside the JSON object
	// (e.g., log lines, warnings, progress messages).
	t.Run("json output is pure JSON with no surrounding text", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		sourcesDir := t.TempDir()

		mustExecCmd(t, "--storage", storageDir, "init", sourcesDir, "--name", "pure-json")

		out := mustExecCmd(t,
			"--storage", storageDir, "workspace", "list", "-o", "json")

		trimmed := strings.TrimSpace(out)
		if !strings.HasPrefix(trimmed, "{") {
			t.Errorf("JSON output does not start with '{': %q", trimmed[:min(50, len(trimmed))])
		}
		if !strings.HasSuffix(trimmed, "}") {
			t.Errorf("JSON output does not end with '}': %q", trimmed[max(0, len(trimmed)-50):])
		}

		// Verify it's valid JSON (no trailing garbage)
		var raw json.RawMessage
		if err := json.Unmarshal([]byte(trimmed), &raw); err != nil {
			t.Errorf("output is not valid JSON: %v\nOutput: %s", err, trimmed)
		}
	})
}

// ---------------------------------------------------------------------------
// 10. Flag Behavior Tests
// ---------------------------------------------------------------------------

func TestContract_FlagBehavior(t *testing.T) {
	t.Parallel()

	// -----------------------------------------------------------------------
	// 10a. --storage flag controls where data is stored.
	// -----------------------------------------------------------------------

	// FAILS IF: --storage flag stops isolating data between different paths.
	t.Run("storage flag controls data location", func(t *testing.T) {
		t.Parallel()

		storageA := t.TempDir()
		storageB := t.TempDir()
		sourcesDir := t.TempDir()

		// Init workspace in storage A
		wsID := strings.TrimSpace(mustExecCmd(t,
			"--storage", storageA, "init", sourcesDir, "--name", "storage-flag-test"))

		// List from storage A — should find the workspace
		listA := mustExecCmd(t, "--storage", storageA, "workspace", "list", "-o", "json")
		listedA := mustParseWorkspacesList(t, listA)
		if len(listedA.Items) != 1 {
			t.Fatalf("storage A: expected 1 workspace, got %d", len(listedA.Items))
		}
		if listedA.Items[0].Id != wsID {
			t.Errorf("storage A: ID = %s, want %s", listedA.Items[0].Id, wsID)
		}

		// List from storage B — should be empty
		listB := mustExecCmd(t, "--storage", storageB, "workspace", "list", "-o", "json")
		listedB := mustParseWorkspacesList(t, listB)
		if len(listedB.Items) != 0 {
			t.Errorf("storage B: expected 0 workspaces, got %d", len(listedB.Items))
		}

		// Remove from storage B — should fail (workspace is in A, not B)
		_, _, err := execCmd(t, "--storage", storageB, "workspace", "remove", wsID)
		if err == nil {
			t.Error("remove from wrong storage succeeded, want error")
		}

		// Remove from storage A — should succeed
		mustExecCmd(t, "--storage", storageA, "workspace", "remove", wsID)

		// Verify storage A is now empty
		listA2 := mustExecCmd(t, "--storage", storageA, "workspace", "list", "-o", "json")
		listedA2 := mustParseWorkspacesList(t, listA2)
		if len(listedA2.Items) != 0 {
			t.Errorf("storage A after remove: expected 0 workspaces, got %d", len(listedA2.Items))
		}
	})

	// -----------------------------------------------------------------------
	// 10b. --workspace-configuration flag sets custom config directory.
	// -----------------------------------------------------------------------

	// FAILS IF: --workspace-configuration flag is ignored or the custom path is not stored.
	t.Run("workspace-configuration flag sets custom config path", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		sourcesDir := t.TempDir()
		customConfig := filepath.Join(t.TempDir(), "my-custom-config")

		mustExecCmd(t,
			"--storage", storageDir, "init", sourcesDir,
			"--name", "custom-config-test",
			"--workspace-configuration", customConfig)

		listOut := mustExecCmd(t,
			"--storage", storageDir, "workspace", "list", "-o", "json")
		listed := mustParseWorkspacesList(t, listOut)
		if len(listed.Items) != 1 {
			t.Fatalf("expected 1 workspace, got %d", len(listed.Items))
		}
		if listed.Items[0].Paths.Configuration != customConfig {
			t.Errorf("config path = %q, want %q", listed.Items[0].Paths.Configuration, customConfig)
		}
	})

	// FAILS IF: omitting --workspace-configuration stops defaulting to <source>/.kortex.
	t.Run("workspace-configuration defaults to source/.kortex", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		sourcesDir := t.TempDir()

		mustExecCmd(t,
			"--storage", storageDir, "init", sourcesDir, "--name", "default-config-test")

		listOut := mustExecCmd(t,
			"--storage", storageDir, "workspace", "list", "-o", "json")
		listed := mustParseWorkspacesList(t, listOut)
		if len(listed.Items) != 1 {
			t.Fatalf("expected 1 workspace, got %d", len(listed.Items))
		}

		expectedConfig := filepath.Join(sourcesDir, ".kortex")
		if listed.Items[0].Paths.Configuration != expectedConfig {
			t.Errorf("default config path = %q, want %q",
				listed.Items[0].Paths.Configuration, expectedConfig)
		}
	})

	// -----------------------------------------------------------------------
	// 10c. Name auto-generation from source directory basename.
	// -----------------------------------------------------------------------

	// FAILS IF: omitting --name stops generating a name from the source directory.
	t.Run("name auto-generated from source directory basename", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		sourcesDir := filepath.Join(t.TempDir(), "my-cool-project")
		if err := os.MkdirAll(sourcesDir, 0755); err != nil {
			t.Fatalf("failed to create source dir: %v", err)
		}

		mustExecCmd(t,
			"--storage", storageDir, "init", sourcesDir)

		listOut := mustExecCmd(t,
			"--storage", storageDir, "workspace", "list", "-o", "json")
		listed := mustParseWorkspacesList(t, listOut)
		if len(listed.Items) != 1 {
			t.Fatalf("expected 1 workspace, got %d", len(listed.Items))
		}
		if listed.Items[0].Name != "my-cool-project" {
			t.Errorf("auto-generated name = %q, want %q",
				listed.Items[0].Name, "my-cool-project")
		}
	})

	// FAILS IF: auto-generated names collide instead of being de-duplicated.
	t.Run("name auto-generation deduplicates on conflict", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()

		// Create two source dirs with the same basename
		parent1 := t.TempDir()
		parent2 := t.TempDir()
		sources1 := filepath.Join(parent1, "project")
		sources2 := filepath.Join(parent2, "project")
		if err := os.MkdirAll(sources1, 0755); err != nil {
			t.Fatalf("failed to create source dir 1: %v", err)
		}
		if err := os.MkdirAll(sources2, 0755); err != nil {
			t.Fatalf("failed to create source dir 2: %v", err)
		}

		mustExecCmd(t, "--storage", storageDir, "init", sources1)
		mustExecCmd(t, "--storage", storageDir, "init", sources2)

		listOut := mustExecCmd(t,
			"--storage", storageDir, "workspace", "list", "-o", "json")
		listed := mustParseWorkspacesList(t, listOut)
		if len(listed.Items) != 2 {
			t.Fatalf("expected 2 workspaces, got %d", len(listed.Items))
		}

		name1 := listed.Items[0].Name
		name2 := listed.Items[1].Name
		if name1 == name2 {
			t.Errorf("auto-generated names collided: both are %q", name1)
		}
		// First should be "project", second should be "project-2"
		if name1 != "project" {
			t.Errorf("first workspace name = %q, want %q", name1, "project")
		}
		if name2 != "project-2" {
			t.Errorf("second workspace name = %q, want %q", name2, "project-2")
		}
	})

	// FAILS IF: auto-generated name is empty or whitespace.
	t.Run("auto-generated name is never empty", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		sourcesDir := t.TempDir()

		mustExecCmd(t, "--storage", storageDir, "init", sourcesDir)

		listOut := mustExecCmd(t,
			"--storage", storageDir, "workspace", "list", "-o", "json")
		listed := mustParseWorkspacesList(t, listOut)
		if len(listed.Items) != 1 {
			t.Fatalf("expected 1 workspace, got %d", len(listed.Items))
		}
		name := listed.Items[0].Name
		if strings.TrimSpace(name) == "" {
			t.Error("auto-generated name is empty or whitespace")
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
