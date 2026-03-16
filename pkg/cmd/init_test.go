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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kortex-hub/kortex-cli/pkg/instances"
	"github.com/spf13/cobra"
)

func TestInitCmd_preRun(t *testing.T) {
	t.Parallel()

	t.Run("uses current directory as default", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()

		c := &initCmd{}
		cmd := &cobra.Command{}
		cmd.Flags().String("workspace-configuration", "", "test flag")
		cmd.Flags().String("storage", tempDir, "test storage flag")

		args := []string{}

		err := c.preRun(cmd, args)
		if err != nil {
			t.Fatalf("preRun() failed: %v", err)
		}

		if c.manager == nil {
			t.Error("Expected manager to be created")
		}

		if c.sourcesDir != "." {
			t.Errorf("Expected sourcesDir to be '.', got %s", c.sourcesDir)
		}

		expectedAbsSourcesDir, _ := filepath.Abs(".")
		if c.absSourcesDir != expectedAbsSourcesDir {
			t.Errorf("Expected absSourcesDir to be %s, got %s", expectedAbsSourcesDir, c.absSourcesDir)
		}

		expectedConfigDir := filepath.Join(".", ".kortex")
		if c.workspaceConfigDir != expectedConfigDir {
			t.Errorf("Expected workspaceConfigDir to be %s, got %s", expectedConfigDir, c.workspaceConfigDir)
		}

		expectedAbsConfigDir, _ := filepath.Abs(expectedConfigDir)
		if c.absConfigDir != expectedAbsConfigDir {
			t.Errorf("Expected absConfigDir to be %s, got %s", expectedAbsConfigDir, c.absConfigDir)
		}
	})

	t.Run("uses provided sources directory", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		sourcesDir := t.TempDir()

		c := &initCmd{}
		cmd := &cobra.Command{}
		cmd.Flags().String("workspace-configuration", "", "test flag")
		cmd.Flags().String("storage", tempDir, "test storage flag")

		args := []string{sourcesDir}

		err := c.preRun(cmd, args)
		if err != nil {
			t.Fatalf("preRun() failed: %v", err)
		}

		if c.manager == nil {
			t.Error("Expected manager to be created")
		}

		if c.sourcesDir != sourcesDir {
			t.Errorf("Expected sourcesDir to be %s, got %s", sourcesDir, c.sourcesDir)
		}

		expectedAbsSourcesDir, _ := filepath.Abs(sourcesDir)
		if c.absSourcesDir != expectedAbsSourcesDir {
			t.Errorf("Expected absSourcesDir to be %s, got %s", expectedAbsSourcesDir, c.absSourcesDir)
		}

		expectedConfigDir := filepath.Join(sourcesDir, ".kortex")
		if c.workspaceConfigDir != expectedConfigDir {
			t.Errorf("Expected workspaceConfigDir to be %s, got %s", expectedConfigDir, c.workspaceConfigDir)
		}

		expectedAbsConfigDir, _ := filepath.Abs(expectedConfigDir)
		if c.absConfigDir != expectedAbsConfigDir {
			t.Errorf("Expected absConfigDir to be %s, got %s", expectedAbsConfigDir, c.absConfigDir)
		}
	})

	t.Run("uses custom workspace configuration", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		configDir := t.TempDir()

		c := &initCmd{
			workspaceConfigDir: configDir,
		}
		cmd := &cobra.Command{}
		cmd.Flags().String("workspace-configuration", "", "test flag")
		cmd.Flags().Set("workspace-configuration", configDir)
		cmd.Flags().String("storage", tempDir, "test storage flag")

		args := []string{}

		err := c.preRun(cmd, args)
		if err != nil {
			t.Fatalf("preRun() failed: %v", err)
		}

		if c.manager == nil {
			t.Error("Expected manager to be created")
		}

		if c.sourcesDir != "." {
			t.Errorf("Expected sourcesDir to be '.', got %s", c.sourcesDir)
		}

		if c.workspaceConfigDir != configDir {
			t.Errorf("Expected workspaceConfigDir to be %s, got %s", configDir, c.workspaceConfigDir)
		}

		expectedAbsConfigDir, _ := filepath.Abs(configDir)
		if c.absConfigDir != expectedAbsConfigDir {
			t.Errorf("Expected absConfigDir to be %s, got %s", expectedAbsConfigDir, c.absConfigDir)
		}
	})

	t.Run("uses custom source and config directories", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		sourcesDir := t.TempDir()
		configDir := t.TempDir()

		c := &initCmd{
			workspaceConfigDir: configDir,
		}
		cmd := &cobra.Command{}
		cmd.Flags().String("workspace-configuration", "", "test flag")
		cmd.Flags().Set("workspace-configuration", configDir)
		cmd.Flags().String("storage", tempDir, "test storage flag")

		args := []string{sourcesDir}

		err := c.preRun(cmd, args)
		if err != nil {
			t.Fatalf("preRun() failed: %v", err)
		}

		if c.manager == nil {
			t.Error("Expected manager to be created")
		}

		if c.sourcesDir != sourcesDir {
			t.Errorf("Expected sourcesDir to be %s, got %s", sourcesDir, c.sourcesDir)
		}

		expectedAbsSourcesDir, _ := filepath.Abs(sourcesDir)
		if c.absSourcesDir != expectedAbsSourcesDir {
			t.Errorf("Expected absSourcesDir to be %s, got %s", expectedAbsSourcesDir, c.absSourcesDir)
		}

		if c.workspaceConfigDir != configDir {
			t.Errorf("Expected workspaceConfigDir to be %s, got %s", configDir, c.workspaceConfigDir)
		}

		expectedAbsConfigDir, _ := filepath.Abs(configDir)
		if c.absConfigDir != expectedAbsConfigDir {
			t.Errorf("Expected absConfigDir to be %s, got %s", expectedAbsConfigDir, c.absConfigDir)
		}
	})

	t.Run("fails when sources directory does not exist", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		nonExistentDir := filepath.Join(tempDir, "does-not-exist")

		c := &initCmd{}
		cmd := &cobra.Command{}
		cmd.Flags().String("workspace-configuration", "", "test flag")
		cmd.Flags().String("storage", tempDir, "test storage flag")

		args := []string{nonExistentDir}

		err := c.preRun(cmd, args)
		if err == nil {
			t.Fatal("Expected preRun() to fail with non-existent directory")
		}

		if !strings.Contains(err.Error(), "sources directory does not exist") {
			t.Errorf("Expected error to contain 'sources directory does not exist', got: %v", err)
		}
	})

	t.Run("fails when sources path is a file not a directory", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		regularFile := filepath.Join(tempDir, "regular-file.txt")

		// Create a regular file
		if err := os.WriteFile(regularFile, []byte("test content"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		c := &initCmd{}
		cmd := &cobra.Command{}
		cmd.Flags().String("workspace-configuration", "", "test flag")
		cmd.Flags().String("storage", tempDir, "test storage flag")

		args := []string{regularFile}

		err := c.preRun(cmd, args)
		if err == nil {
			t.Fatal("Expected preRun() to fail when sources path is a file")
		}

		if !strings.Contains(err.Error(), "sources path is not a directory") {
			t.Errorf("Expected error to contain 'sources path is not a directory', got: %v", err)
		}
	})
}

// TestInitCmd_preRun_relativePath tests relative path handling separately
// because it changes the working directory, which affects the entire process.
func TestInitCmd_preRun_relativePath(t *testing.T) {
	storageDir := t.TempDir()
	workDir := t.TempDir()
	relativePath := filepath.Join(".", "relative", "path")

	// Save current working directory
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current working directory: %v", err)
	}

	// Change to temp directory
	if err := os.Chdir(workDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}
	defer func() {
		// Restore original working directory
		if err := os.Chdir(origWd); err != nil {
			t.Errorf("Failed to restore working directory: %v", err)
		}
	}()

	// Create the relative directory in the temp working directory
	if err := os.MkdirAll(relativePath, 0755); err != nil {
		t.Fatalf("Failed to create relative directory: %v", err)
	}

	c := &initCmd{}
	cmd := &cobra.Command{}
	cmd.Flags().String("workspace-configuration", "", "test flag")
	cmd.Flags().String("storage", storageDir, "test storage flag")

	args := []string{relativePath}

	err = c.preRun(cmd, args)
	if err != nil {
		t.Fatalf("preRun() failed: %v", err)
	}

	if c.manager == nil {
		t.Error("Expected manager to be created")
	}

	if c.sourcesDir != relativePath {
		t.Errorf("Expected sourcesDir to be %s, got %s", relativePath, c.sourcesDir)
	}

	expectedAbsSourcesDir, _ := filepath.Abs(relativePath)
	if c.absSourcesDir != expectedAbsSourcesDir {
		t.Errorf("Expected absSourcesDir to be %s, got %s", expectedAbsSourcesDir, c.absSourcesDir)
	}

	expectedConfigDir := filepath.Join(relativePath, ".kortex")
	if c.workspaceConfigDir != expectedConfigDir {
		t.Errorf("Expected workspaceConfigDir to be %s, got %s", expectedConfigDir, c.workspaceConfigDir)
	}
}

func TestInitCmd_rejectsFileAsSourcesPath(t *testing.T) {
	t.Parallel()

	storageDir := t.TempDir()
	regularFile := filepath.Join(storageDir, "regular-file.txt")

	// Create a regular file
	if err := os.WriteFile(regularFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	rootCmd := NewRootCmd()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"--storage", storageDir, "init", regularFile})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("Expected Execute() to fail when sources path is a file")
	}

	if !strings.Contains(err.Error(), "sources path is not a directory") {
		t.Errorf("Expected error to contain 'sources path is not a directory', got: %v", err)
	}

	// Verify no instance was created
	manager, err := instances.NewManager(storageDir)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	instancesList, err := manager.List()
	if err != nil {
		t.Fatalf("Failed to list instances: %v", err)
	}

	if len(instancesList) != 0 {
		t.Errorf("Expected 0 instances, got %d", len(instancesList))
	}
}
