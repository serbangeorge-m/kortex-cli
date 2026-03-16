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
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// fakeInstance is a test double for the Instance interface
type fakeInstance struct {
	id         string
	name       string
	sourceDir  string
	configDir  string
	accessible bool
}

// Compile-time check to ensure fakeInstance implements Instance interface
var _ Instance = (*fakeInstance)(nil)

func (f *fakeInstance) GetID() string {
	return f.id
}

func (f *fakeInstance) GetName() string {
	return f.name
}

func (f *fakeInstance) GetSourceDir() string {
	return f.sourceDir
}

func (f *fakeInstance) GetConfigDir() string {
	return f.configDir
}

func (f *fakeInstance) IsAccessible() bool {
	return f.accessible
}

func (f *fakeInstance) Dump() InstanceData {
	return InstanceData{
		ID:   f.id,
		Name: f.name,
		Paths: InstancePaths{
			Source:        f.sourceDir,
			Configuration: f.configDir,
		},
	}
}

// newFakeInstanceParams contains the parameters for creating a fake instance
type newFakeInstanceParams struct {
	ID         string
	Name       string
	SourceDir  string
	ConfigDir  string
	Accessible bool
}

// newFakeInstance creates a new fake instance for testing
func newFakeInstance(params newFakeInstanceParams) Instance {
	return &fakeInstance{
		id:         params.ID,
		name:       params.Name,
		sourceDir:  params.SourceDir,
		configDir:  params.ConfigDir,
		accessible: params.Accessible,
	}
}

// fakeInstanceFactory creates fake instances from InstanceData for testing
func fakeInstanceFactory(data InstanceData) (Instance, error) {
	if data.ID == "" {
		return nil, errors.New("instance ID cannot be empty")
	}
	if data.Name == "" {
		return nil, errors.New("instance name cannot be empty")
	}
	if data.Paths.Source == "" {
		return nil, ErrInvalidPath
	}
	if data.Paths.Configuration == "" {
		return nil, ErrInvalidPath
	}
	// For testing, we assume instances are accessible by default
	// Tests can verify accessibility behavior separately
	return &fakeInstance{
		id:         data.ID,
		name:       data.Name,
		sourceDir:  data.Paths.Source,
		configDir:  data.Paths.Configuration,
		accessible: true,
	}, nil
}

// fakeGenerator is a test double for the generator.Generator interface
type fakeGenerator struct {
	counter int
	mu      sync.Mutex
}

func newFakeGenerator() *fakeGenerator {
	return &fakeGenerator{counter: 0}
}

func (g *fakeGenerator) Generate() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.counter++
	// Generate a deterministic ID with hex characters to avoid all-numeric
	return fmt.Sprintf("test-id-%064x", g.counter)
}

// fakeSequentialGenerator returns a predefined sequence of IDs
type fakeSequentialGenerator struct {
	ids       []string
	callCount int
	mu        sync.Mutex
}

func newFakeSequentialGenerator(ids []string) *fakeSequentialGenerator {
	return &fakeSequentialGenerator{ids: ids, callCount: 0}
}

func (g *fakeSequentialGenerator) Generate() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.callCount >= len(g.ids) {
		// If we've exhausted the predefined IDs, return the last one
		return g.ids[len(g.ids)-1]
	}
	id := g.ids[g.callCount]
	g.callCount++
	return id
}

// mustNewManager creates a test manager with the default fake factory and generator.
func mustNewManager(t *testing.T, storageDir string) Manager {
	t.Helper()
	m, err := newManagerWithFactory(storageDir, fakeInstanceFactory, newFakeGenerator())
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}
	return m
}

// mustAdd adds an instance to the manager and fails the test if it errors.
func mustAdd(t *testing.T, m Manager, inst Instance) Instance {
	t.Helper()
	added, err := m.Add(inst)
	if err != nil {
		t.Fatalf("failed to add instance: %v", err)
	}
	return added
}

func TestNewManager(t *testing.T) {
	t.Parallel()

	t.Run("creates manager with valid storage directory", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		manager, err := NewManager(tmpDir)
		if err != nil {
			t.Fatalf("NewManager() unexpected error = %v", err)
		}
		if manager == nil {
			t.Fatal("NewManager() returned nil manager")
		}

		// Verify storage directory exists
		if _, err := os.Stat(tmpDir); os.IsNotExist(err) {
			t.Error("Storage directory was not created")
		}
	})

	t.Run("creates storage directory if not exists", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		nestedDir := filepath.Join(tmpDir, "nested", "storage")

		manager, err := NewManager(nestedDir)
		if err != nil {
			t.Fatalf("NewManager() unexpected error = %v", err)
		}
		if manager == nil {
			t.Fatal("NewManager() returned nil manager")
		}

		// Verify nested directory was created
		info, err := os.Stat(nestedDir)
		if err != nil {
			t.Fatalf("Nested directory was not created: %v", err)
		}
		if !info.IsDir() {
			t.Error("Storage path is not a directory")
		}
	})

	t.Run("returns error for empty storage directory", func(t *testing.T) {
		t.Parallel()

		_, err := NewManager("")
		if err == nil {
			t.Error("NewManager() expected error for empty storage dir, got nil")
		}
	})

	t.Run("creates storage file at expected path", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		manager, err := NewManager(tmpDir)
		if err != nil {
			t.Fatalf("NewManager() unexpected error = %v", err)
		}

		// We can't directly access storageFile since it's on the unexported struct,
		// but we can verify behavior by adding an instance and checking file creation
		instanceTmpDir := t.TempDir()
		inst := newFakeInstance(newFakeInstanceParams{
			SourceDir:  filepath.Join(instanceTmpDir, "source"),
			ConfigDir:  filepath.Join(instanceTmpDir, "config"),
			Accessible: true,
		})
		mustAdd(t, manager, inst)

		expectedFile := filepath.Join(tmpDir, DefaultStorageFileName)
		if _, err := os.Stat(expectedFile); os.IsNotExist(err) {
			t.Errorf("Storage file was not created at expected path: %v", expectedFile)
		}
	})
}

func TestNewManagerWithFactory(t *testing.T) {
	t.Parallel()

	// FAILS IF: newManagerWithFactory stops validating nil factory
	t.Run("returns error for nil factory", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		_, err := newManagerWithFactory(tmpDir, nil, newFakeGenerator())
		if err == nil {
			t.Fatal("newManagerWithFactory() expected error for nil factory, got nil")
		}
		if err.Error() != "factory cannot be nil" {
			t.Errorf("error = %v, want 'factory cannot be nil'", err)
		}
	})

	// FAILS IF: newManagerWithFactory stops validating nil generator
	t.Run("returns error for nil generator", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		_, err := newManagerWithFactory(tmpDir, fakeInstanceFactory, nil)
		if err == nil {
			t.Fatal("newManagerWithFactory() expected error for nil generator, got nil")
		}
		if err.Error() != "generator cannot be nil" {
			t.Errorf("error = %v, want 'generator cannot be nil'", err)
		}
	})

	// FAILS IF: newManagerWithFactory stops validating empty storage directory
	t.Run("returns error for empty storage directory", func(t *testing.T) {
		t.Parallel()

		_, err := newManagerWithFactory("", fakeInstanceFactory, newFakeGenerator())
		if err == nil {
			t.Fatal("newManagerWithFactory() expected error for empty storage dir, got nil")
		}
		if err.Error() != "storage directory cannot be empty" {
			t.Errorf("error = %v, want 'storage directory cannot be empty'", err)
		}
	})
}

func TestManager_Add(t *testing.T) {
	t.Parallel()

	t.Run("adds valid instance successfully", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		manager := mustNewManager(t, tmpDir)

		instanceTmpDir := t.TempDir()
		inst := newFakeInstance(newFakeInstanceParams{
			SourceDir:  filepath.Join(instanceTmpDir, "source"),
			ConfigDir:  filepath.Join(instanceTmpDir, "config"),
			Accessible: true,
		})
		added, err := manager.Add(inst)
		if err != nil {
			t.Fatalf("Add() unexpected error = %v", err)
		}
		if added == nil {
			t.Fatal("Add() returned nil instance")
		}
		if added.GetID() == "" {
			t.Error("Add() returned instance with empty ID")
		}

		// Verify instance was added
		instances, err := manager.List()
		if err != nil {
			t.Fatalf("failed to list instances: %v", err)
		}
		if len(instances) != 1 {
			t.Errorf("List() returned %d instances, want 1", len(instances))
		}
	})

	t.Run("returns error for nil instance", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		manager := mustNewManager(t, tmpDir)

		_, err := manager.Add(nil)
		if err == nil {
			t.Error("Add() expected error for nil instance, got nil")
		}
	})

	t.Run("generates unique IDs when adding instances", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		// Create a sequential generator that returns duplicate ID first, then unique ones
		// Sequence: "duplicate-id", "duplicate-id", "unique-id-1"
		// When adding first instance: gets "duplicate-id"
		// When adding second instance: gets "duplicate-id" (skip), then "unique-id-1"
		gen := newFakeSequentialGenerator([]string{
			"duplicate-id-0000000000000000000000000000000000000000000000000000000a",
			"duplicate-id-0000000000000000000000000000000000000000000000000000000a",
			"unique-id-1-0000000000000000000000000000000000000000000000000000000b",
		})
		manager, err := newManagerWithFactory(tmpDir, fakeInstanceFactory, gen)
		if err != nil {
			t.Fatalf("failed to create manager: %v", err)
		}

		instanceTmpDir := t.TempDir()
		// Create instances without IDs (empty ID)
		inst1 := newFakeInstance(newFakeInstanceParams{
			SourceDir:  filepath.Join(instanceTmpDir, "source1"),
			ConfigDir:  filepath.Join(instanceTmpDir, "config1"),
			Accessible: true,
		})
		inst2 := newFakeInstance(newFakeInstanceParams{
			SourceDir:  filepath.Join(instanceTmpDir, "source2"),
			ConfigDir:  filepath.Join(instanceTmpDir, "config2"),
			Accessible: true,
		})

		added1 := mustAdd(t, manager, inst1)
		added2 := mustAdd(t, manager, inst2)

		id1 := added1.GetID()
		id2 := added2.GetID()

		if id1 == "" {
			t.Error("First instance has empty ID")
		}
		if id2 == "" {
			t.Error("Second instance has empty ID")
		}
		if id1 == id2 {
			t.Errorf("Manager generated duplicate IDs: %v", id1)
		}

		// Verify the manager skipped the duplicate and used the third ID
		expectedID1 := "duplicate-id-0000000000000000000000000000000000000000000000000000000a"
		expectedID2 := "unique-id-1-0000000000000000000000000000000000000000000000000000000b"
		if id1 != expectedID1 {
			t.Errorf("First instance ID = %v, want %v", id1, expectedID1)
		}
		if id2 != expectedID2 {
			t.Errorf("Second instance ID = %v, want %v", id2, expectedID2)
		}

		// Verify the generator was called 3 times (once for first instance, twice for second)
		if gen.callCount != 3 {
			t.Errorf("Generator was called %d times, want 3", gen.callCount)
		}
	})

	t.Run("persists data to JSON file", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		manager := mustNewManager(t, tmpDir)

		instanceTmpDir := t.TempDir()
		inst := newFakeInstance(newFakeInstanceParams{
			SourceDir:  filepath.Join(instanceTmpDir, "source"),
			ConfigDir:  filepath.Join(instanceTmpDir, "config"),
			Accessible: true,
		})
		mustAdd(t, manager, inst)

		// Check that JSON file exists and is readable
		storageFile := filepath.Join(tmpDir, DefaultStorageFileName)
		data, err := os.ReadFile(storageFile)
		if err != nil {
			t.Fatalf("Failed to read storage file: %v", err)
		}
		if len(data) == 0 {
			t.Error("Storage file is empty")
		}
	})

	t.Run("adds multiple instances", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		manager := mustNewManager(t, tmpDir)

		instanceTmpDir := t.TempDir()
		inst1 := newFakeInstance(newFakeInstanceParams{
			SourceDir:  filepath.Join(instanceTmpDir, "source1"),
			ConfigDir:  filepath.Join(instanceTmpDir, "config1"),
			Accessible: true,
		})
		inst2 := newFakeInstance(newFakeInstanceParams{
			SourceDir:  filepath.Join(instanceTmpDir, "source2"),
			ConfigDir:  filepath.Join(instanceTmpDir, "config2"),
			Accessible: true,
		})
		inst3 := newFakeInstance(newFakeInstanceParams{
			SourceDir:  filepath.Join(instanceTmpDir, "source3"),
			ConfigDir:  filepath.Join(instanceTmpDir, "config3"),
			Accessible: true,
		})

		mustAdd(t, manager, inst1)
		mustAdd(t, manager, inst2)
		mustAdd(t, manager, inst3)

		instances, err := manager.List()
		if err != nil {
			t.Fatalf("failed to list instances: %v", err)
		}
		if len(instances) != 3 {
			t.Errorf("List() returned %d instances, want 3", len(instances))
		}
	})
}

func TestManager_List(t *testing.T) {
	t.Parallel()

	t.Run("returns empty list when no instances exist", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		manager := mustNewManager(t, tmpDir)

		instances, err := manager.List()
		if err != nil {
			t.Fatalf("List() unexpected error = %v", err)
		}
		if len(instances) != 0 {
			t.Errorf("List() returned %d instances, want 0", len(instances))
		}
	})

	t.Run("returns all added instances", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		manager := mustNewManager(t, tmpDir)

		instanceTmpDir := t.TempDir()
		inst1 := newFakeInstance(newFakeInstanceParams{
			SourceDir:  filepath.Join(instanceTmpDir, "source1"),
			ConfigDir:  filepath.Join(instanceTmpDir, "config1"),
			Accessible: true,
		})
		inst2 := newFakeInstance(newFakeInstanceParams{
			SourceDir:  filepath.Join(instanceTmpDir, "source2"),
			ConfigDir:  filepath.Join(instanceTmpDir, "config2"),
			Accessible: true,
		})

		mustAdd(t, manager, inst1)
		mustAdd(t, manager, inst2)

		instances, err := manager.List()
		if err != nil {
			t.Fatalf("List() unexpected error = %v", err)
		}
		if len(instances) != 2 {
			t.Errorf("List() returned %d instances, want 2", len(instances))
		}
	})

	t.Run("returns empty list for empty storage file", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		manager := mustNewManager(t, tmpDir)

		// Create empty storage file
		storageFile := filepath.Join(tmpDir, DefaultStorageFileName)
		if err := os.WriteFile(storageFile, []byte{}, 0644); err != nil {
			t.Fatalf("failed to write empty storage file: %v", err)
		}

		instances, err := manager.List()
		if err != nil {
			t.Fatalf("List() unexpected error = %v", err)
		}
		if len(instances) != 0 {
			t.Errorf("List() returned %d instances, want 0 for empty file", len(instances))
		}
	})
}

func TestManager_Get(t *testing.T) {
	t.Parallel()

	t.Run("retrieves existing instance by ID", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		manager := mustNewManager(t, tmpDir)

		instanceTmpDir := t.TempDir()
		expectedSource := filepath.Join(instanceTmpDir, "source")
		expectedConfig := filepath.Join(instanceTmpDir, "config")
		inst := newFakeInstance(newFakeInstanceParams{
			SourceDir:  expectedSource,
			ConfigDir:  expectedConfig,
			Accessible: true,
		})
		added := mustAdd(t, manager, inst)

		generatedID := added.GetID()

		retrieved, err := manager.Get(generatedID)
		if err != nil {
			t.Fatalf("Get() unexpected error = %v", err)
		}
		if retrieved.GetID() != generatedID {
			t.Errorf("Get() returned instance with ID = %v, want %v", retrieved.GetID(), generatedID)
		}
		if retrieved.GetSourceDir() != expectedSource {
			t.Errorf("Get() returned instance with SourceDir = %v, want %v", retrieved.GetSourceDir(), expectedSource)
		}
		if retrieved.GetConfigDir() != expectedConfig {
			t.Errorf("Get() returned instance with ConfigDir = %v, want %v", retrieved.GetConfigDir(), expectedConfig)
		}
	})

	t.Run("returns error for nonexistent instance", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		manager := mustNewManager(t, tmpDir)

		_, err := manager.Get("nonexistent-id")
		if err != ErrInstanceNotFound {
			t.Errorf("Get() error = %v, want %v", err, ErrInstanceNotFound)
		}
	})
}

func TestManager_Delete(t *testing.T) {
	t.Parallel()

	t.Run("deletes existing instance successfully", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		manager := mustNewManager(t, tmpDir)

		instanceTmpDir := t.TempDir()
		sourceDir := filepath.Join(instanceTmpDir, "source")
		configDir := filepath.Join(instanceTmpDir, "config")
		inst := newFakeInstance(newFakeInstanceParams{
			SourceDir:  sourceDir,
			ConfigDir:  configDir,
			Accessible: true,
		})
		added := mustAdd(t, manager, inst)

		generatedID := added.GetID()

		err := manager.Delete(generatedID)
		if err != nil {
			t.Fatalf("Delete() unexpected error = %v", err)
		}

		// Verify instance was deleted
		_, err = manager.Get(generatedID)
		if err != ErrInstanceNotFound {
			t.Errorf("Get() after Delete() error = %v, want %v", err, ErrInstanceNotFound)
		}
	})

	t.Run("returns error for nonexistent instance", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		manager := mustNewManager(t, tmpDir)

		err := manager.Delete("nonexistent-id")
		if err != ErrInstanceNotFound {
			t.Errorf("Delete() error = %v, want %v", err, ErrInstanceNotFound)
		}
	})

	t.Run("deletes only specified instance", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		manager := mustNewManager(t, tmpDir)

		instanceTmpDir := t.TempDir()
		source1 := filepath.Join(instanceTmpDir, "source1")
		config1 := filepath.Join(instanceTmpDir, "config1")
		source2 := filepath.Join(instanceTmpDir, "source2")
		config2 := filepath.Join(instanceTmpDir, "config2")
		inst1 := newFakeInstance(newFakeInstanceParams{
			SourceDir:  source1,
			ConfigDir:  config1,
			Accessible: true,
		})
		inst2 := newFakeInstance(newFakeInstanceParams{
			SourceDir:  source2,
			ConfigDir:  config2,
			Accessible: true,
		})
		added1 := mustAdd(t, manager, inst1)
		added2 := mustAdd(t, manager, inst2)

		id1 := added1.GetID()
		id2 := added2.GetID()

		if err := manager.Delete(id1); err != nil {
			t.Fatalf("Delete(id1) unexpected error = %v", err)
		}

		// Verify inst2 still exists
		_, err := manager.Get(id2)
		if err != nil {
			t.Errorf("Get(id2) after Delete(id1) unexpected error = %v", err)
		}

		// Verify inst1 is gone
		_, err = manager.Get(id1)
		if err != ErrInstanceNotFound {
			t.Errorf("Get(id1) after Delete(id1) error = %v, want %v", err, ErrInstanceNotFound)
		}
	})

	t.Run("persists deletion across manager instances", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		manager1 := mustNewManager(t, tmpDir)

		inst := newFakeInstance(newFakeInstanceParams{
			SourceDir:  filepath.Join(string(filepath.Separator), "tmp", "source"),
			ConfigDir:  filepath.Join(string(filepath.Separator), "tmp", "config"),
			Accessible: true,
		})
		added := mustAdd(t, manager1, inst)

		generatedID := added.GetID()

		if err := manager1.Delete(generatedID); err != nil {
			t.Fatalf("Delete() unexpected error = %v", err)
		}

		// Create new manager with same storage
		manager2 := mustNewManager(t, tmpDir)
		_, err := manager2.Get(generatedID)
		if err != ErrInstanceNotFound {
			t.Errorf("Get() from new manager error = %v, want %v", err, ErrInstanceNotFound)
		}
	})
}

func TestManager_Reconcile(t *testing.T) {
	t.Parallel()

	t.Run("removes instances with inaccessible source directories", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		// Custom factory that creates inaccessible instances for testing
		inaccessibleFactory := func(data InstanceData) (Instance, error) {
			if data.ID == "" {
				return nil, errors.New("instance ID cannot be empty")
			}
			if data.Name == "" {
				return nil, errors.New("instance name cannot be empty")
			}
			if data.Paths.Source == "" || data.Paths.Configuration == "" {
				return nil, ErrInvalidPath
			}
			return &fakeInstance{
				id:         data.ID,
				name:       data.Name,
				sourceDir:  data.Paths.Source,
				configDir:  data.Paths.Configuration,
				accessible: false, // Always inaccessible for this test
			}, nil
		}
		manager, err := newManagerWithFactory(tmpDir, inaccessibleFactory, newFakeGenerator())
		if err != nil {
			t.Fatalf("failed to create manager: %v", err)
		}

		// Add instance that is inaccessible
		instanceTmpDir := t.TempDir()
		inst := newFakeInstance(newFakeInstanceParams{
			SourceDir:  filepath.Join(instanceTmpDir, "nonexistent-source"),
			ConfigDir:  filepath.Join(instanceTmpDir, "config"),
			Accessible: false,
		})
		mustAdd(t, manager, inst)

		removed, err := manager.Reconcile()
		if err != nil {
			t.Fatalf("Reconcile() unexpected error = %v", err)
		}

		if len(removed) != 1 {
			t.Errorf("Reconcile() removed %d instances, want 1", len(removed))
		}
	})

	t.Run("removes instances with inaccessible config directories", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		// Custom factory that creates inaccessible instances for testing
		inaccessibleFactory := func(data InstanceData) (Instance, error) {
			if data.ID == "" {
				return nil, errors.New("instance ID cannot be empty")
			}
			if data.Name == "" {
				return nil, errors.New("instance name cannot be empty")
			}
			if data.Paths.Source == "" || data.Paths.Configuration == "" {
				return nil, ErrInvalidPath
			}
			return &fakeInstance{
				id:         data.ID,
				name:       data.Name,
				sourceDir:  data.Paths.Source,
				configDir:  data.Paths.Configuration,
				accessible: false, // Always inaccessible for this test
			}, nil
		}
		manager, err := newManagerWithFactory(tmpDir, inaccessibleFactory, newFakeGenerator())
		if err != nil {
			t.Fatalf("failed to create manager: %v", err)
		}

		// Add instance that is inaccessible
		instanceTmpDir := t.TempDir()
		inst := newFakeInstance(newFakeInstanceParams{
			SourceDir:  filepath.Join(instanceTmpDir, "source"),
			ConfigDir:  filepath.Join(instanceTmpDir, "nonexistent-config"),
			Accessible: false,
		})
		mustAdd(t, manager, inst)

		removed, err := manager.Reconcile()
		if err != nil {
			t.Fatalf("Reconcile() unexpected error = %v", err)
		}

		if len(removed) != 1 {
			t.Errorf("Reconcile() removed %d instances, want 1", len(removed))
		}
	})

	t.Run("returns list of removed IDs", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		// Custom factory that creates inaccessible instances for testing
		inaccessibleFactory := func(data InstanceData) (Instance, error) {
			if data.ID == "" {
				return nil, errors.New("instance ID cannot be empty")
			}
			if data.Name == "" {
				return nil, errors.New("instance name cannot be empty")
			}
			if data.Paths.Source == "" || data.Paths.Configuration == "" {
				return nil, ErrInvalidPath
			}
			return &fakeInstance{
				id:         data.ID,
				name:       data.Name,
				sourceDir:  data.Paths.Source,
				configDir:  data.Paths.Configuration,
				accessible: false, // Always inaccessible for this test
			}, nil
		}
		manager, err := newManagerWithFactory(tmpDir, inaccessibleFactory, newFakeGenerator())
		if err != nil {
			t.Fatalf("failed to create manager: %v", err)
		}

		instanceTmpDir := t.TempDir()
		inaccessibleSource := filepath.Join(instanceTmpDir, "nonexistent-source")
		inst := newFakeInstance(newFakeInstanceParams{
			SourceDir:  inaccessibleSource,
			ConfigDir:  filepath.Join(instanceTmpDir, "config"),
			Accessible: false,
		})
		added := mustAdd(t, manager, inst)

		generatedID := added.GetID()

		removed, err := manager.Reconcile()
		if err != nil {
			t.Fatalf("Reconcile() unexpected error = %v", err)
		}

		if len(removed) != 1 || removed[0] != generatedID {
			t.Errorf("Reconcile() removed = %v, want [%v]", removed, generatedID)
		}
	})

	t.Run("keeps accessible instances", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		instanceTmpDir := t.TempDir()

		accessibleSource := filepath.Join(instanceTmpDir, "accessible-source")
		inaccessibleSource := filepath.Join(instanceTmpDir, "nonexistent-source")

		// Custom factory that checks source directory to determine accessibility
		mixedFactory := func(data InstanceData) (Instance, error) {
			if data.ID == "" {
				return nil, errors.New("instance ID cannot be empty")
			}
			if data.Paths.Source == "" || data.Paths.Configuration == "" {
				return nil, ErrInvalidPath
			}
			accessible := data.Paths.Source == accessibleSource
			return &fakeInstance{
				id:         data.ID,
				sourceDir:  data.Paths.Source,
				configDir:  data.Paths.Configuration,
				accessible: accessible,
			}, nil
		}
		manager, err := newManagerWithFactory(tmpDir, mixedFactory, newFakeGenerator())
		if err != nil {
			t.Fatalf("failed to create manager: %v", err)
		}

		accessibleConfig := filepath.Join(instanceTmpDir, "accessible-config")

		// Add accessible instance
		accessible := newFakeInstance(newFakeInstanceParams{
			SourceDir:  accessibleSource,
			ConfigDir:  accessibleConfig,
			Accessible: true,
		})
		mustAdd(t, manager, accessible)

		// Add inaccessible instance
		inaccessible := newFakeInstance(newFakeInstanceParams{
			SourceDir:  inaccessibleSource,
			ConfigDir:  filepath.Join(instanceTmpDir, "nonexistent-config"),
			Accessible: false,
		})
		mustAdd(t, manager, inaccessible)

		removed, err := manager.Reconcile()
		if err != nil {
			t.Fatalf("Reconcile() unexpected error = %v", err)
		}

		if len(removed) != 1 {
			t.Errorf("Reconcile() removed %d instances, want 1", len(removed))
		}

		// Verify accessible instance still exists
		instances, err := manager.List()
		if err != nil {
			t.Fatalf("failed to list instances: %v", err)
		}
		if len(instances) != 1 {
			t.Errorf("List() after Reconcile() returned %d instances, want 1", len(instances))
		}
		if instances[0].GetSourceDir() != accessibleSource {
			t.Errorf("Remaining instance SourceDir = %v, want %v", instances[0].GetSourceDir(), accessibleSource)
		}
	})

	t.Run("returns empty list when all instances are accessible", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		manager := mustNewManager(t, tmpDir)

		instanceTmpDir := t.TempDir()
		inst := newFakeInstance(newFakeInstanceParams{
			SourceDir:  filepath.Join(instanceTmpDir, "source"),
			ConfigDir:  filepath.Join(instanceTmpDir, "config"),
			Accessible: true,
		})
		mustAdd(t, manager, inst)

		removed, err := manager.Reconcile()
		if err != nil {
			t.Fatalf("Reconcile() unexpected error = %v", err)
		}

		if len(removed) != 0 {
			t.Errorf("Reconcile() removed %d instances, want 0", len(removed))
		}
	})

	t.Run("returns empty removed list when no instances exist", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		manager := mustNewManager(t, tmpDir)

		removed, err := manager.Reconcile()
		if err != nil {
			t.Fatalf("Reconcile() unexpected error = %v", err)
		}

		if len(removed) != 0 {
			t.Errorf("Reconcile() removed %d instances, want 0", len(removed))
		}
	})
}

func TestManager_Persistence(t *testing.T) {
	t.Parallel()

	t.Run("persists data across manager instances", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		instanceTmpDir := t.TempDir()

		// Create first manager and add instance
		manager1 := mustNewManager(t, tmpDir)
		expectedSource := filepath.Join(instanceTmpDir, "source")
		expectedConfig := filepath.Join(instanceTmpDir, "config")
		inst := newFakeInstance(newFakeInstanceParams{
			SourceDir:  expectedSource,
			ConfigDir:  expectedConfig,
			Accessible: true,
		})
		added := mustAdd(t, manager1, inst)

		generatedID := added.GetID()

		// Create second manager with same storage
		manager2 := mustNewManager(t, tmpDir)
		instances, err := manager2.List()
		if err != nil {
			t.Fatalf("List() from second manager unexpected error = %v", err)
		}

		if len(instances) != 1 {
			t.Errorf("List() from second manager returned %d instances, want 1", len(instances))
		}
		if instances[0].GetID() != generatedID {
			t.Errorf("Instance ID = %v, want %v", instances[0].GetID(), generatedID)
		}
		if instances[0].GetSourceDir() != expectedSource {
			t.Errorf("Instance SourceDir = %v, want %v", instances[0].GetSourceDir(), expectedSource)
		}
	})

	t.Run("serializes instances to correct JSON structure", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		manager := mustNewManager(t, tmpDir)

		instanceTmpDir := t.TempDir()
		expectedSource := filepath.Join(instanceTmpDir, "source")
		expectedConfig := filepath.Join(instanceTmpDir, "config")
		inst := newFakeInstance(newFakeInstanceParams{
			SourceDir:  expectedSource,
			ConfigDir:  expectedConfig,
			Accessible: true,
		})
		added := mustAdd(t, manager, inst)

		generatedID := added.GetID()

		// Read and verify JSON content
		storageFile := filepath.Join(tmpDir, DefaultStorageFileName)
		data, err := os.ReadFile(storageFile)
		if err != nil {
			t.Fatalf("Failed to read storage file: %v", err)
		}

		// Unmarshal JSON data
		var instances []InstanceData
		if err := json.Unmarshal(data, &instances); err != nil {
			t.Fatalf("Failed to unmarshal JSON: %v", err)
		}

		// Verify we have exactly one instance
		if len(instances) != 1 {
			t.Fatalf("Expected 1 instance in JSON, got %d", len(instances))
		}

		// Verify the instance values
		if instances[0].ID != generatedID {
			t.Errorf("JSON ID = %v, want %v", instances[0].ID, generatedID)
		}
		if instances[0].Paths.Source != expectedSource {
			t.Errorf("JSON Paths.Source = %v, want %v", instances[0].Paths.Source, expectedSource)
		}
		if instances[0].Paths.Configuration != expectedConfig {
			t.Errorf("JSON Paths.Configuration = %v, want %v", instances[0].Paths.Configuration, expectedConfig)
		}
	})
}

func TestManager_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	t.Run("thread safety with concurrent Add operations", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		manager := mustNewManager(t, tmpDir)

		instanceTmpDir := t.TempDir()
		var wg sync.WaitGroup
		numGoroutines := 10

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				sourceDir := filepath.Join(instanceTmpDir, "source", fmt.Sprintf("%d", id))
				configDir := filepath.Join(instanceTmpDir, "config", fmt.Sprintf("%d", id))
				inst := newFakeInstance(newFakeInstanceParams{
					SourceDir:  sourceDir,
					ConfigDir:  configDir,
					Accessible: true,
				})
				mustAdd(t, manager, inst)
			}(i)
		}

		wg.Wait()

		instances, err := manager.List()
		if err != nil {
			t.Fatalf("failed to list instances: %v", err)
		}
		if len(instances) != numGoroutines {
			t.Errorf("After concurrent adds, List() returned %d instances, want %d", len(instances), numGoroutines)
		}
	})

	t.Run("thread safety with concurrent reads", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		manager := mustNewManager(t, tmpDir)

		instanceTmpDir := t.TempDir()
		// Add some instances first
		for i := 0; i < 5; i++ {
			sourceDir := filepath.Join(instanceTmpDir, "source", fmt.Sprintf("%d", i))
			configDir := filepath.Join(instanceTmpDir, "config", fmt.Sprintf("%d", i))
			inst := newFakeInstance(newFakeInstanceParams{
				SourceDir:  sourceDir,
				ConfigDir:  configDir,
				Accessible: true,
			})
			mustAdd(t, manager, inst)
		}

		var wg sync.WaitGroup
		numGoroutines := 20

		// Concurrent List operations
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				manager.List()
			}()
		}

		wg.Wait()

		// Verify data is still consistent
		instances, err := manager.List()
		if err != nil {
			t.Fatalf("failed to list instances: %v", err)
		}
		if len(instances) != 5 {
			t.Errorf("After concurrent reads, List() returned %d instances, want 5", len(instances))
		}
	})

	t.Run("thread safety with concurrent reads and writes", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		manager := mustNewManager(t, tmpDir)

		instanceTmpDir := t.TempDir()
		var wg sync.WaitGroup
		numWriters := 5
		numReaders := 10

		// Launch writers
		for i := 0; i < numWriters; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				sourceDir := filepath.Join(instanceTmpDir, "source", fmt.Sprintf("rw-%d", id))
				configDir := filepath.Join(instanceTmpDir, "config", fmt.Sprintf("rw-%d", id))
				inst := newFakeInstance(newFakeInstanceParams{
					SourceDir:  sourceDir,
					ConfigDir:  configDir,
					Accessible: true,
				})
				mustAdd(t, manager, inst)
			}(i)
		}

		// Launch readers concurrently with writers
		for i := 0; i < numReaders; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				manager.List()
			}()
		}

		wg.Wait()

		// Verify all writes completed
		instances, err := manager.List()
		if err != nil {
			t.Fatalf("failed to list instances: %v", err)
		}
		if len(instances) != numWriters {
			t.Errorf("After concurrent reads+writes, List() returned %d instances, want %d", len(instances), numWriters)
		}
	})
}

func TestManager_ensureUniqueName(t *testing.T) {
	t.Parallel()

	t.Run("returns original name when no conflict", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		m := mustNewManager(t, tmpDir)

		// Cast to concrete type to access unexported methods
		mgr := m.(*manager)

		instances := []Instance{
			newFakeInstance(newFakeInstanceParams{
				ID:         "id1",
				Name:       "workspace1",
				SourceDir:  "/path/source1",
				ConfigDir:  "/path/config1",
				Accessible: true,
			}),
			newFakeInstance(newFakeInstanceParams{
				ID:         "id2",
				Name:       "workspace2",
				SourceDir:  "/path/source2",
				ConfigDir:  "/path/config2",
				Accessible: true,
			}),
		}

		result := mgr.ensureUniqueName("myworkspace", instances)

		if result != "myworkspace" {
			t.Errorf("ensureUniqueName() = %v, want myworkspace", result)
		}
	})

	t.Run("adds increment when name conflicts", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		m := mustNewManager(t, tmpDir)

		mgr := m.(*manager)

		instances := []Instance{
			newFakeInstance(newFakeInstanceParams{
				ID:         "id1",
				Name:       "myworkspace",
				SourceDir:  "/path/source1",
				ConfigDir:  "/path/config1",
				Accessible: true,
			}),
		}

		result := mgr.ensureUniqueName("myworkspace", instances)

		if result != "myworkspace-2" {
			t.Errorf("ensureUniqueName() = %v, want myworkspace-2", result)
		}
	})

	t.Run("increments until unique name is found", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		m := mustNewManager(t, tmpDir)

		mgr := m.(*manager)

		instances := []Instance{
			newFakeInstance(newFakeInstanceParams{
				ID:         "id1",
				Name:       "myworkspace",
				SourceDir:  "/path/source1",
				ConfigDir:  "/path/config1",
				Accessible: true,
			}),
			newFakeInstance(newFakeInstanceParams{
				ID:         "id2",
				Name:       "myworkspace-2",
				SourceDir:  "/path/source2",
				ConfigDir:  "/path/config2",
				Accessible: true,
			}),
			newFakeInstance(newFakeInstanceParams{
				ID:         "id3",
				Name:       "myworkspace-3",
				SourceDir:  "/path/source3",
				ConfigDir:  "/path/config3",
				Accessible: true,
			}),
		}

		result := mgr.ensureUniqueName("myworkspace", instances)

		if result != "myworkspace-4" {
			t.Errorf("ensureUniqueName() = %v, want myworkspace-4", result)
		}
	})

	t.Run("increments to double digits", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		m := mustNewManager(t, tmpDir)

		mgr := m.(*manager)

		// Create instances with names up to myworkspace-10
		instances := []Instance{}
		instances = append(instances, newFakeInstance(newFakeInstanceParams{
			ID:         "id0",
			Name:       "myworkspace",
			SourceDir:  "/path/source0",
			ConfigDir:  "/path/config0",
			Accessible: true,
		}))
		for i := 2; i <= 10; i++ {
			name := fmt.Sprintf("myworkspace-%d", i)
			id := fmt.Sprintf("id%d", i)
			instances = append(instances, newFakeInstance(newFakeInstanceParams{
				ID:         id,
				Name:       name,
				SourceDir:  fmt.Sprintf("/path/source%d", i),
				ConfigDir:  fmt.Sprintf("/path/config%d", i),
				Accessible: true,
			}))
		}

		result := mgr.ensureUniqueName("myworkspace", instances)

		if result != "myworkspace-11" {
			t.Errorf("ensureUniqueName() = %v, want myworkspace-11", result)
		}
	})

	t.Run("returns original name for empty instance list", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		m := mustNewManager(t, tmpDir)

		mgr := m.(*manager)

		instances := []Instance{}

		result := mgr.ensureUniqueName("myworkspace", instances)

		if result != "myworkspace" {
			t.Errorf("ensureUniqueName() = %v, want myworkspace", result)
		}
	})
}

func TestManager_generateUniqueName(t *testing.T) {
	t.Parallel()

	t.Run("generates name from source directory basename", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		m := mustNewManager(t, tmpDir)

		mgr := m.(*manager)

		instances := []Instance{}

		result := mgr.generateUniqueName("/home/user/myproject", instances)

		if result != "myproject" {
			t.Errorf("generateUniqueName() = %v, want myproject", result)
		}
	})

	t.Run("handles conflicting names from different paths", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		m := mustNewManager(t, tmpDir)

		mgr := m.(*manager)

		instances := []Instance{
			newFakeInstance(newFakeInstanceParams{
				ID:         "id1",
				Name:       "myproject",
				SourceDir:  "/home/user/myproject",
				ConfigDir:  "/home/user/myproject/.kortex",
				Accessible: true,
			}),
		}

		result := mgr.generateUniqueName("/home/otheruser/myproject", instances)

		if result != "myproject-2" {
			t.Errorf("generateUniqueName() = %v, want myproject-2", result)
		}
	})

	t.Run("handles Windows-style paths", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		m := mustNewManager(t, tmpDir)

		mgr := m.(*manager)

		instances := []Instance{}

		// filepath.Base works cross-platform
		result := mgr.generateUniqueName(filepath.Join("C:", "Users", "user", "myproject"), instances)

		if result != "myproject" {
			t.Errorf("generateUniqueName() = %v, want myproject", result)
		}
	})

	t.Run("handles current directory", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		m := mustNewManager(t, tmpDir)

		mgr := m.(*manager)

		instances := []Instance{}

		// Get the current working directory
		wd, _ := os.Getwd()
		expectedName := filepath.Base(wd)

		result := mgr.generateUniqueName(wd, instances)

		if result != expectedName {
			t.Errorf("generateUniqueName() = %v, want %v", result, expectedName)
		}
	})
}
