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
	"testing"
)

func TestNewListCmd(t *testing.T) {
	t.Parallel()

	t.Run("delegates to workspace list command", func(t *testing.T) {
		t.Parallel()

		cmd := NewListCmd()
		if cmd == nil {
			t.Fatal("NewListCmd() returned nil")
		}

		if cmd.Use != "list" {
			t.Errorf("expected Use to be 'list', got '%s'", cmd.Use)
		}

		// Verify it shares behavior with workspace list
		wsListCmd := NewWorkspaceListCmd()
		if cmd.Short != wsListCmd.Short {
			t.Errorf("expected Short to match workspace list (%q), got %q", wsListCmd.Short, cmd.Short)
		}
	})
}
