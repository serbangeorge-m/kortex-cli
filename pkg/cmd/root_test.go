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
)

func TestNewRootCmd(t *testing.T) {
	t.Parallel()

	t.Run("sets correct use and description", func(t *testing.T) {
		t.Parallel()

		rootCmd := NewRootCmd()
		if rootCmd.Use != "kortex-cli" {
			t.Errorf("Expected Use to be 'kortex-cli', got '%s'", rootCmd.Use)
		}

		if rootCmd.Short == "" {
			t.Error("Expected Short description to be set")
		}
	})

	t.Run("succeeds with help flag", func(t *testing.T) {
		t.Parallel()

		rootCmd := NewRootCmd()
		buf := new(bytes.Buffer)
		rootCmd.SetOut(buf)
		rootCmd.SetArgs([]string{"--help"})

		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("Execute() failed: %v", err)
		}
	})

	t.Run("succeeds with no arguments", func(t *testing.T) {
		t.Parallel()

		rootCmd := NewRootCmd()
		buf := new(bytes.Buffer)
		rootCmd.SetOut(buf)
		rootCmd.SetArgs([]string{})

		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("Execute() failed: %v", err)
		}
	})
}

func TestNewRootCmd_storageFlag(t *testing.T) {
	t.Parallel()

	t.Run("exists with default value ending in .kortex-cli", func(t *testing.T) {
		t.Parallel()

		rootCmd := NewRootCmd()

		flag := rootCmd.PersistentFlags().Lookup("storage")
		if flag == nil {
			t.Fatal("Expected --storage flag to exist")
		}

		if flag.DefValue == "" {
			t.Error("Expected --storage flag to have a default value")
		}

		if !strings.HasSuffix(flag.DefValue, ".kortex-cli") {
			t.Errorf("Expected default value to end with '.kortex-cli', got '%s'", flag.DefValue)
		}
	})

	t.Run("accepts custom value", func(t *testing.T) {
		t.Parallel()

		rootCmd := NewRootCmd()
		buf := new(bytes.Buffer)
		rootCmd.SetOut(buf)
		rootCmd.SetErr(buf)

		tmpDir := t.TempDir()
		customPath := filepath.Join(tmpDir, "custom", "path", "storage")
		rootCmd.SetArgs([]string{"--storage", customPath, "version"})

		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("Execute() failed: %v", err)
		}

		storagePath, err := rootCmd.PersistentFlags().GetString("storage")
		if err != nil {
			t.Fatalf("Failed to get storage flag: %v", err)
		}

		if storagePath != customPath {
			t.Errorf("Expected storage to be '%s', got '%s'", customPath, storagePath)
		}
	})

	t.Run("is inherited by subcommands", func(t *testing.T) {
		t.Parallel()

		rootCmd := NewRootCmd()

		versionCmd, _, err := rootCmd.Find([]string{"version"})
		if err != nil {
			t.Fatalf("Failed to find version command: %v", err)
		}

		flag := versionCmd.InheritedFlags().Lookup("storage")
		if flag == nil {
			t.Error("Expected --storage flag to be inherited by subcommands")
		}
	})

	t.Run("returns error when value missing", func(t *testing.T) {
		t.Parallel()

		rootCmd := NewRootCmd()
		buf := new(bytes.Buffer)
		rootCmd.SetOut(buf)
		rootCmd.SetErr(buf)

		rootCmd.SetArgs([]string{"--storage"})

		err := rootCmd.Execute()
		if err == nil {
			t.Fatal("Expected Execute() to fail when --storage flag is provided without a value")
		}

		errMsg := err.Error()
		if !strings.Contains(errMsg, "flag") && !strings.Contains(errMsg, "argument") {
			t.Errorf("Expected error message to contain 'flag' or 'argument', got: %s", errMsg)
		}
	})
}

func TestNewRootCmd_storageEnvVariable(t *testing.T) {
	t.Run("sets default from env variable", func(t *testing.T) {
		envPath := filepath.Join(t.TempDir(), "from-env")
		t.Setenv("KORTEX_CLI_STORAGE", envPath)

		rootCmd := NewRootCmd()
		flag := rootCmd.PersistentFlags().Lookup("storage")
		if flag == nil {
			t.Fatal("Expected --storage flag to exist")
		}

		if flag.DefValue != envPath {
			t.Errorf("Expected default value to be '%s' (from env var), got '%s'", envPath, flag.DefValue)
		}
	})

	t.Run("flag overrides env variable", func(t *testing.T) {
		envPath := filepath.Join(t.TempDir(), "from-env")
		t.Setenv("KORTEX_CLI_STORAGE", envPath)

		rootCmd := NewRootCmd()
		buf := new(bytes.Buffer)
		rootCmd.SetOut(buf)
		rootCmd.SetErr(buf)

		flagPath := filepath.Join(t.TempDir(), "from-flag")
		rootCmd.SetArgs([]string{"--storage", flagPath, "version"})

		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("Execute() failed: %v", err)
		}

		storagePath, err := rootCmd.PersistentFlags().GetString("storage")
		if err != nil {
			t.Fatalf("Failed to get storage flag: %v", err)
		}

		if storagePath != flagPath {
			t.Errorf("Expected storage to be '%s' (from flag), got '%s'", flagPath, storagePath)
		}
	})

	t.Run("uses computed default when env var empty", func(t *testing.T) {
		t.Setenv("KORTEX_CLI_STORAGE", "")

		rootCmd := NewRootCmd()
		flag := rootCmd.PersistentFlags().Lookup("storage")
		if flag == nil {
			t.Fatal("Expected --storage flag to exist")
		}

		if !strings.HasSuffix(flag.DefValue, ".kortex-cli") {
			t.Errorf("Expected default value to end with '.kortex-cli', got '%s'", flag.DefValue)
		}
	})
}
