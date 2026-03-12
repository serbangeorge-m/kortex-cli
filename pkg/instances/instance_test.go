// Copyright 2026 Red Hat, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package instances

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewInstance(t *testing.T) {
	t.Parallel()

	t.Run("creates instance with valid paths", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		sourceDir := filepath.Join(tmpDir, "test-source")
		configDir := filepath.Join(tmpDir, "test-config")

		inst, err := NewInstance(NewInstanceParams{
			SourceDir: sourceDir,
			ConfigDir: configDir,
		})
		if err != nil {
			t.Fatalf("NewInstance() unexpected error = %v", err)
		}
		if inst == nil {
			t.Fatal("NewInstance() returned nil instance")
		}

		if inst.GetID() != "" {
			t.Errorf("GetID() = %v, want empty string (ID should be set by Manager)", inst.GetID())
		}
		if inst.GetName() != "" {
			t.Errorf("GetName() = %v, want empty string (Name should be set by Manager if not provided)", inst.GetName())
		}
		if inst.GetSourceDir() != sourceDir {
			t.Errorf("GetSourceDir() = %v, want %v", inst.GetSourceDir(), sourceDir)
		}
		if inst.GetConfigDir() != configDir {
			t.Errorf("GetConfigDir() = %v, want %v", inst.GetConfigDir(), configDir)
		}
	})

	t.Run("converts relative paths to absolute", func(t *testing.T) {
		t.Parallel()

		inst, err := NewInstance(NewInstanceParams{
			SourceDir: "source",
			ConfigDir: "config",
		})
		if err != nil {
			t.Fatalf("NewInstance() unexpected error = %v", err)
		}

		if !filepath.IsAbs(inst.GetSourceDir()) {
			t.Errorf("GetSourceDir() returned relative path: %v", inst.GetSourceDir())
		}
		if !filepath.IsAbs(inst.GetConfigDir()) {
			t.Errorf("GetConfigDir() returned relative path: %v", inst.GetConfigDir())
		}

		// Verify the paths are based on current working directory
		wd, err := os.Getwd()
		if err != nil {
			t.Fatalf("Failed to get working directory: %v", err)
		}

		expectedSourceDir := filepath.Join(wd, "source")
		expectedConfigDir := filepath.Join(wd, "config")

		if inst.GetSourceDir() != expectedSourceDir {
			t.Errorf("GetSourceDir() = %v, want %v", inst.GetSourceDir(), expectedSourceDir)
		}
		if inst.GetConfigDir() != expectedConfigDir {
			t.Errorf("GetConfigDir() = %v, want %v", inst.GetConfigDir(), expectedConfigDir)
		}
	})

	t.Run("creates instance with custom name", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		sourceDir := filepath.Join(tmpDir, "test-source")
		configDir := filepath.Join(tmpDir, "test-config")

		inst, err := NewInstance(NewInstanceParams{
			SourceDir: sourceDir,
			ConfigDir: configDir,
			Name:      "my-workspace",
		})
		if err != nil {
			t.Fatalf("NewInstance() unexpected error = %v", err)
		}

		if inst.GetName() != "my-workspace" {
			t.Errorf("GetName() = %v, want 'my-workspace'", inst.GetName())
		}
	})

	t.Run("returns error for empty source dir", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		_, err := NewInstance(NewInstanceParams{
			SourceDir: "",
			ConfigDir: filepath.Join(tmpDir, "config"),
		})
		if err != ErrInvalidPath {
			t.Errorf("NewInstance() error = %v, want %v", err, ErrInvalidPath)
		}
	})

	t.Run("returns error for empty config dir", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		_, err := NewInstance(NewInstanceParams{
			SourceDir: filepath.Join(tmpDir, "source"),
			ConfigDir: "",
		})
		if err != ErrInvalidPath {
			t.Errorf("NewInstance() error = %v, want %v", err, ErrInvalidPath)
		}
	})

	t.Run("returns error for both empty", func(t *testing.T) {
		t.Parallel()

		_, err := NewInstance(NewInstanceParams{
			SourceDir: "",
			ConfigDir: "",
		})
		if err != ErrInvalidPath {
			t.Errorf("NewInstance() error = %v, want %v", err, ErrInvalidPath)
		}
	})
}

func TestInstance_IsAccessible(t *testing.T) {
	t.Parallel()

	t.Run("returns true for accessible directories", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		sourceDir := filepath.Join(tmpDir, "source")
		configDir := filepath.Join(tmpDir, "config")

		if err := os.MkdirAll(sourceDir, 0755); err != nil {
			t.Fatalf("Failed to create source dir: %v", err)
		}
		if err := os.MkdirAll(configDir, 0755); err != nil {
			t.Fatalf("Failed to create config dir: %v", err)
		}

		inst, err := NewInstance(NewInstanceParams{
			SourceDir: sourceDir,
			ConfigDir: configDir,
		})
		if err != nil {
			t.Fatalf("NewInstance() unexpected error = %v", err)
		}

		if !inst.IsAccessible() {
			t.Error("IsAccessible() = false, want true for accessible directories")
		}
	})

	t.Run("returns false when source dir not accessible", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		configDir := filepath.Join(tmpDir, "config")

		if err := os.MkdirAll(configDir, 0755); err != nil {
			t.Fatalf("Failed to create config dir: %v", err)
		}

		inst, err := NewInstance(NewInstanceParams{
			SourceDir: filepath.Join(tmpDir, "nonexistent"),
			ConfigDir: configDir,
		})
		if err != nil {
			t.Fatalf("NewInstance() unexpected error = %v", err)
		}

		if inst.IsAccessible() {
			t.Error("IsAccessible() = true, want false when source dir not accessible")
		}
	})

	t.Run("returns false when config dir not accessible", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		sourceDir := filepath.Join(tmpDir, "source")

		if err := os.MkdirAll(sourceDir, 0755); err != nil {
			t.Fatalf("Failed to create source dir: %v", err)
		}

		inst, err := NewInstance(NewInstanceParams{
			SourceDir: sourceDir,
			ConfigDir: filepath.Join(tmpDir, "nonexistent"),
		})
		if err != nil {
			t.Fatalf("NewInstance() unexpected error = %v", err)
		}

		if inst.IsAccessible() {
			t.Error("IsAccessible() = true, want false when config dir not accessible")
		}
	})

	t.Run("returns false when both dirs not accessible", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()

		inst, err := NewInstance(NewInstanceParams{
			SourceDir: filepath.Join(tmpDir, "nonexistent1"),
			ConfigDir: filepath.Join(tmpDir, "nonexistent2"),
		})
		if err != nil {
			t.Fatalf("NewInstance() unexpected error = %v", err)
		}

		if inst.IsAccessible() {
			t.Error("IsAccessible() = true, want false when both dirs not accessible")
		}
	})
}

func TestNewInstanceFromData(t *testing.T) {
	t.Parallel()

	// FAILS IF: NewInstanceFromData stops creating instances from valid InstanceData
	t.Run("creates instance from valid data", func(t *testing.T) {
		t.Parallel()

		data := InstanceData{
			ID:   "test-id-abc123",
			Name: "my-workspace",
			Paths: InstancePaths{
				Source:        filepath.Join(string(filepath.Separator), "home", "user", "project"),
				Configuration: filepath.Join(string(filepath.Separator), "home", "user", "project", ".kortex"),
			},
		}

		inst, err := NewInstanceFromData(data)
		if err != nil {
			t.Fatalf("NewInstanceFromData() unexpected error = %v", err)
		}
		if inst == nil {
			t.Fatal("NewInstanceFromData() returned nil instance")
		}

		if inst.GetID() != data.ID {
			t.Errorf("GetID() = %v, want %v", inst.GetID(), data.ID)
		}
		if inst.GetName() != data.Name {
			t.Errorf("GetName() = %v, want %v", inst.GetName(), data.Name)
		}
		if inst.GetSourceDir() != data.Paths.Source {
			t.Errorf("GetSourceDir() = %v, want %v", inst.GetSourceDir(), data.Paths.Source)
		}
		if inst.GetConfigDir() != data.Paths.Configuration {
			t.Errorf("GetConfigDir() = %v, want %v", inst.GetConfigDir(), data.Paths.Configuration)
		}
	})

	// FAILS IF: NewInstanceFromData stops validating empty ID
	t.Run("returns error for empty ID", func(t *testing.T) {
		t.Parallel()

		data := InstanceData{
			ID:   "",
			Name: "my-workspace",
			Paths: InstancePaths{
				Source:        filepath.Join(string(filepath.Separator), "home", "user", "project"),
				Configuration: filepath.Join(string(filepath.Separator), "home", "user", "project", ".kortex"),
			},
		}

		_, err := NewInstanceFromData(data)
		if err == nil {
			t.Fatal("NewInstanceFromData() expected error for empty ID, got nil")
		}
		if err.Error() != "instance ID cannot be empty" {
			t.Errorf("error = %v, want 'instance ID cannot be empty'", err)
		}
	})

	// FAILS IF: NewInstanceFromData stops validating empty Name
	t.Run("returns error for empty name", func(t *testing.T) {
		t.Parallel()

		data := InstanceData{
			ID:   "test-id-abc123",
			Name: "",
			Paths: InstancePaths{
				Source:        filepath.Join(string(filepath.Separator), "home", "user", "project"),
				Configuration: filepath.Join(string(filepath.Separator), "home", "user", "project", ".kortex"),
			},
		}

		_, err := NewInstanceFromData(data)
		if err == nil {
			t.Fatal("NewInstanceFromData() expected error for empty name, got nil")
		}
		if err.Error() != "instance name cannot be empty" {
			t.Errorf("error = %v, want 'instance name cannot be empty'", err)
		}
	})

	// FAILS IF: NewInstanceFromData stops validating empty Source path
	t.Run("returns error for empty source path", func(t *testing.T) {
		t.Parallel()

		data := InstanceData{
			ID:   "test-id-abc123",
			Name: "my-workspace",
			Paths: InstancePaths{
				Source:        "",
				Configuration: filepath.Join(string(filepath.Separator), "home", "user", "project", ".kortex"),
			},
		}

		_, err := NewInstanceFromData(data)
		if err != ErrInvalidPath {
			t.Errorf("error = %v, want %v", err, ErrInvalidPath)
		}
	})

	// FAILS IF: NewInstanceFromData stops validating empty Configuration path
	t.Run("returns error for empty configuration path", func(t *testing.T) {
		t.Parallel()

		data := InstanceData{
			ID:   "test-id-abc123",
			Name: "my-workspace",
			Paths: InstancePaths{
				Source:        filepath.Join(string(filepath.Separator), "home", "user", "project"),
				Configuration: "",
			},
		}

		_, err := NewInstanceFromData(data)
		if err != ErrInvalidPath {
			t.Errorf("error = %v, want %v", err, ErrInvalidPath)
		}
	})

	// FAILS IF: NewInstanceFromData stops preserving paths as-is (without re-resolving)
	t.Run("preserves paths without modification", func(t *testing.T) {
		t.Parallel()

		sourcePath := filepath.Join(string(filepath.Separator), "specific", "absolute", "source")
		configPath := filepath.Join(string(filepath.Separator), "specific", "absolute", "config")
		data := InstanceData{
			ID:   "test-id-xyz789",
			Name: "preserved-paths",
			Paths: InstancePaths{
				Source:        sourcePath,
				Configuration: configPath,
			},
		}

		inst, err := NewInstanceFromData(data)
		if err != nil {
			t.Fatalf("NewInstanceFromData() unexpected error = %v", err)
		}

		if inst.GetSourceDir() != sourcePath {
			t.Errorf("GetSourceDir() = %v, want %v (path should be preserved as-is)", inst.GetSourceDir(), sourcePath)
		}
		if inst.GetConfigDir() != configPath {
			t.Errorf("GetConfigDir() = %v, want %v (path should be preserved as-is)", inst.GetConfigDir(), configPath)
		}
	})

	// FAILS IF: Dump() round-trip stops being lossless
	t.Run("round-trips through Dump", func(t *testing.T) {
		t.Parallel()

		originalData := InstanceData{
			ID:   "round-trip-id",
			Name: "round-trip-workspace",
			Paths: InstancePaths{
				Source:        filepath.Join(string(filepath.Separator), "round", "trip", "source"),
				Configuration: filepath.Join(string(filepath.Separator), "round", "trip", "config"),
			},
		}

		inst, err := NewInstanceFromData(originalData)
		if err != nil {
			t.Fatalf("NewInstanceFromData() unexpected error = %v", err)
		}

		dumped := inst.Dump()

		if dumped.ID != originalData.ID {
			t.Errorf("Dump().ID = %v, want %v", dumped.ID, originalData.ID)
		}
		if dumped.Name != originalData.Name {
			t.Errorf("Dump().Name = %v, want %v", dumped.Name, originalData.Name)
		}
		if dumped.Paths.Source != originalData.Paths.Source {
			t.Errorf("Dump().Paths.Source = %v, want %v", dumped.Paths.Source, originalData.Paths.Source)
		}
		if dumped.Paths.Configuration != originalData.Paths.Configuration {
			t.Errorf("Dump().Paths.Configuration = %v, want %v", dumped.Paths.Configuration, originalData.Paths.Configuration)
		}
	})
}

func TestIsDirAccessible(t *testing.T) {
	t.Parallel()

	t.Run("returns true for existing directory", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		existingDir := filepath.Join(tmpDir, "existing")
		if err := os.MkdirAll(existingDir, 0755); err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}

		if !isDirAccessible(existingDir) {
			t.Error("isDirAccessible() = false, want true for existing directory")
		}
	})

	t.Run("returns false for nonexistent directory", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		nonexistentDir := filepath.Join(tmpDir, "nonexistent")

		if isDirAccessible(nonexistentDir) {
			t.Error("isDirAccessible() = true, want false for nonexistent directory")
		}
	})

	t.Run("returns false for file instead of directory", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "file.txt")
		if err := os.WriteFile(filePath, []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}

		if isDirAccessible(filePath) {
			t.Error("isDirAccessible() = true, want false for file instead of directory")
		}
	})
}
