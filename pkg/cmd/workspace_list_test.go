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
	"path/filepath"
	"strings"
	"testing"

	"github.com/kortex-hub/kortex-cli/pkg/instances"
)

func TestWorkspaceListCmd(t *testing.T) {
	t.Parallel()

	t.Run("rejects invalid output format", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		rootCmd := NewRootCmd()
		rootCmd.SetArgs([]string{"workspace", "list", "--storage", storageDir, "--output", "xml"})

		err := rootCmd.Execute()
		if err == nil {
			t.Fatal("Expected error with invalid output format, got nil")
		}

		if !strings.Contains(err.Error(), "unsupported output format") {
			t.Errorf("Expected error to contain 'unsupported output format', got: %v", err)
		}
	})

	t.Run("rejects invalid output format with short flag", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		rootCmd := NewRootCmd()
		rootCmd.SetArgs([]string{"workspace", "list", "--storage", storageDir, "-o", "yaml"})

		err := rootCmd.Execute()
		if err == nil {
			t.Fatal("Expected error with invalid output format, got nil")
		}

		if !strings.Contains(err.Error(), "unsupported output format") {
			t.Errorf("Expected error to contain 'unsupported output format', got: %v", err)
		}
	})

	t.Run("shows no workspaces message when empty", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		rootCmd := NewRootCmd()
		rootCmd.SetArgs([]string{"workspace", "list", "--storage", storageDir})

		var output bytes.Buffer
		rootCmd.SetOut(&output)

		err := rootCmd.Execute()
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		result := output.String()
		if !strings.Contains(result, "No workspaces registered") {
			t.Errorf("Expected 'No workspaces registered' message, got: %s", result)
		}
	})

	t.Run("lists single workspace", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		sourcesDir := t.TempDir()

		// Create a workspace first
		manager, err := instances.NewManager(storageDir)
		if err != nil {
			t.Fatalf("Failed to create manager: %v", err)
		}

		instance, err := instances.NewInstance(instances.NewInstanceParams{
			SourceDir: sourcesDir,
			ConfigDir: filepath.Join(sourcesDir, ".kortex"),
		})
		if err != nil {
			t.Fatalf("Failed to create instance: %v", err)
		}

		addedInstance, err := manager.Add(instance)
		if err != nil {
			t.Fatalf("Failed to add instance: %v", err)
		}

		// Now list workspaces
		rootCmd := NewRootCmd()
		rootCmd.SetArgs([]string{"workspace", "list", "--storage", storageDir})

		var output bytes.Buffer
		rootCmd.SetOut(&output)

		err = rootCmd.Execute()
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		result := output.String()
		expectedID := "ID: " + addedInstance.GetID()
		if !strings.Contains(result, expectedID) {
			t.Errorf("Expected output to contain %q, got: %s", expectedID, result)
		}
		expectedName := "  Name: " + addedInstance.GetName()
		if !strings.Contains(result, expectedName) {
			t.Errorf("Expected output to contain %q, got: %s", expectedName, result)
		}
		expectedSources := "  Sources: " + sourcesDir
		if !strings.Contains(result, expectedSources) {
			t.Errorf("Expected output to contain %q, got: %s", expectedSources, result)
		}
		expectedConfig := "  Configuration: " + filepath.Join(sourcesDir, ".kortex")
		if !strings.Contains(result, expectedConfig) {
			t.Errorf("Expected output to contain %q, got: %s", expectedConfig, result)
		}
	})

	t.Run("lists multiple workspaces", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()
		sourcesDir1 := t.TempDir()
		sourcesDir2 := t.TempDir()

		// Create two workspaces
		manager, err := instances.NewManager(storageDir)
		if err != nil {
			t.Fatalf("Failed to create manager: %v", err)
		}

		instance1, err := instances.NewInstance(instances.NewInstanceParams{
			SourceDir: sourcesDir1,
			ConfigDir: filepath.Join(sourcesDir1, ".kortex"),
		})
		if err != nil {
			t.Fatalf("Failed to create instance 1: %v", err)
		}

		instance2, err := instances.NewInstance(instances.NewInstanceParams{
			SourceDir: sourcesDir2,
			ConfigDir: filepath.Join(sourcesDir2, ".kortex"),
		})
		if err != nil {
			t.Fatalf("Failed to create instance 2: %v", err)
		}

		addedInstance1, err := manager.Add(instance1)
		if err != nil {
			t.Fatalf("Failed to add instance 1: %v", err)
		}

		addedInstance2, err := manager.Add(instance2)
		if err != nil {
			t.Fatalf("Failed to add instance 2: %v", err)
		}

		// Now list workspaces
		rootCmd := NewRootCmd()
		rootCmd.SetArgs([]string{"workspace", "list", "--storage", storageDir})

		var output bytes.Buffer
		rootCmd.SetOut(&output)

		err = rootCmd.Execute()
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		result := output.String()
		expectedID1 := "ID: " + addedInstance1.GetID()
		if !strings.Contains(result, expectedID1) {
			t.Errorf("Expected output to contain %q, got: %s", expectedID1, result)
		}
		expectedName1 := "  Name: " + addedInstance1.GetName()
		if !strings.Contains(result, expectedName1) {
			t.Errorf("Expected output to contain %q, got: %s", expectedName1, result)
		}
		expectedSources1 := "  Sources: " + sourcesDir1
		if !strings.Contains(result, expectedSources1) {
			t.Errorf("Expected output to contain %q, got: %s", expectedSources1, result)
		}
		expectedConfig1 := "  Configuration: " + filepath.Join(sourcesDir1, ".kortex")
		if !strings.Contains(result, expectedConfig1) {
			t.Errorf("Expected output to contain %q, got: %s", expectedConfig1, result)
		}
		expectedID2 := "ID: " + addedInstance2.GetID()
		if !strings.Contains(result, expectedID2) {
			t.Errorf("Expected output to contain %q, got: %s", expectedID2, result)
		}
		expectedName2 := "  Name: " + addedInstance2.GetName()
		if !strings.Contains(result, expectedName2) {
			t.Errorf("Expected output to contain %q, got: %s", expectedName2, result)
		}
		expectedSources2 := "  Sources: " + sourcesDir2
		if !strings.Contains(result, expectedSources2) {
			t.Errorf("Expected output to contain %q, got: %s", expectedSources2, result)
		}
		expectedConfig2 := "  Configuration: " + filepath.Join(sourcesDir2, ".kortex")
		if !strings.Contains(result, expectedConfig2) {
			t.Errorf("Expected output to contain %q, got: %s", expectedConfig2, result)
		}
	})
}
