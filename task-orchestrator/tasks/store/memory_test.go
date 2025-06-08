package store_test

import (
	"task-orchestrator/tasks"
	"task-orchestrator/tasks/store"
	"testing"

	"github.com/stretchr/testify/require"
	"gotest.tools/v3/assert"
)

func newTestTask(id string) *tasks.Task {
	return &tasks.Task{
		ID:     id,
		Type:   "print",
		Status: "submitted",
		Result: "",
	}
}

func TestMemoryTaskStore_Save(t *testing.T) {
	t.Parallel()
	task1 := newTestTask("task-save-1")
	taskExisting := newTestTask("task-existing")

	testCases := []struct {
		name        string
		storeSetup  func() *store.MemoryTaskStore
		taskToSave  *tasks.Task
		expectErr   bool
		errContains string
		postCheck   func(t *testing.T, s *store.MemoryTaskStore, taskID string) // Optional check after save
	}{
		{
			name: "successful save",
			storeSetup: func() *store.MemoryTaskStore {
				return store.NewMemoryTaskStore()
			},
			taskToSave: task1,
			expectErr:  false,
			postCheck: func(t *testing.T, s *store.MemoryTaskStore, taskID string) {
				got, err := s.Get(taskID)
				require.NoError(t, err)
				assert.Equal(t, task1.ID, got.ID)
				assert.Equal(t, task1.Status, got.Status)
			},
		},
		{
			name: "save duplicate",
			storeSetup: func() *store.MemoryTaskStore {
				s := store.NewMemoryTaskStore()
				err := s.Save(taskExisting)
				require.NoError(t, err, "Setup: failed to save initial task")
				return s
			},
			taskToSave:  taskExisting, // Attempt to save the same task again
			expectErr:   true,
			errContains: "already exists",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			s := tc.storeSetup()
			err := s.Save(tc.taskToSave)

			if tc.expectErr {
				require.Error(t, err)
				if tc.errContains != "" {
					require.ErrorContains(t, err, tc.errContains)
				}
			} else {
				require.NoError(t, err)
				if tc.postCheck != nil {
					tc.postCheck(t, s, tc.taskToSave.ID)
				}
			}
		})
	}
}

func TestMemoryTaskStore_Get(t *testing.T) {
	t.Parallel()
	task1 := newTestTask("task-get-1")
	taskForCopyTest := newTestTask("task-get-copy")

	testCases := []struct {
		name        string
		storeSetup  func() *store.MemoryTaskStore
		idToGet     string
		expectTask  *tasks.Task
		expectErr   bool
		errContains string
		checkIsCopy bool // Special flag to test if a copy is returned
	}{
		{
			name: "successful get",
			storeSetup: func() *store.MemoryTaskStore {
				s := store.NewMemoryTaskStore()
				require.NoError(t, s.Save(task1))
				return s
			},
			idToGet:    "task-get-1",
			expectTask: task1,
			expectErr:  false,
		},
		{
			name: "get not found",
			storeSetup: func() *store.MemoryTaskStore {
				return store.NewMemoryTaskStore()
			},
			idToGet:     "does-not-exist",
			expectErr:   true,
			errContains: "not found",
		},
		{
			name: "get returns a copy",
			storeSetup: func() *store.MemoryTaskStore {
				s := store.NewMemoryTaskStore()
				require.NoError(t, s.Save(taskForCopyTest))
				return s
			},
			idToGet:     "task-get-copy",
			expectTask:  taskForCopyTest, // We expect the original values before mutation
			expectErr:   false,
			checkIsCopy: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			s := tc.storeSetup()
			got, err := s.Get(tc.idToGet)

			if tc.expectErr {
				require.Error(t, err)
				if tc.errContains != "" {
					require.ErrorContains(t, err, tc.errContains)
				}
				require.Nil(t, got)
			} else {
				require.NoError(t, err)
				require.NotNil(t, got)
				assert.Equal(t, tc.expectTask.ID, got.ID)
				assert.Equal(t, tc.expectTask.Type, got.Type)
				assert.Equal(t, tc.expectTask.Status, got.Status)
				assert.Equal(t, tc.expectTask.Result, got.Result)

				if tc.checkIsCopy {
					// Mutate the retrieved task
					got.Status = "hacked"
					got.Result = "oops"

					// Get the task again from the store
					original, errGetAgain := s.Get(tc.idToGet)
					require.NoError(t, errGetAgain, "Failed to get task again for copy check")
					require.NotNil(t, original)

					// Assert that the original task in the store was not modified
					assert.Equal(t, tc.expectTask.Status, original.Status, "Original task status was modified")
					assert.Equal(t, tc.expectTask.Result, original.Result, "Original task result was modified")
					require.NotEqual(t, got.Status, original.Status, "Retrieved copy status should differ from original after mutation")
				}
			}
		})
	}
}

func TestMemoryTaskStore_Update(t *testing.T) {
	t.Parallel()
	taskToUpdate := newTestTask("task-update-1")
	initialStatus := taskToUpdate.Status
	initialResult := taskToUpdate.Result

	testCases := []struct {
		name         string
		storeSetup   func() *store.MemoryTaskStore
		idToUpdate   string
		newStatus    string
		newResult    string
		expectErr    bool
		errContains  string
		expectedTask *tasks.Task // Expected state after update
	}{
		{
			name: "successful update",
			storeSetup: func() *store.MemoryTaskStore {
				s := store.NewMemoryTaskStore()
				// Save a copy to avoid modifying taskToUpdate directly in setup
				taskCopy := *taskToUpdate
				require.NoError(t, s.Save(&taskCopy))
				return s
			},
			idToUpdate: "task-update-1",
			newStatus:  "done",
			newResult:  "updated successfully",
			expectErr:  false,
			expectedTask: &tasks.Task{
				ID:     "task-update-1",
				Type:   taskToUpdate.Type, // Type should not change on update
				Status: "done",
				Result: "updated successfully",
			},
		},
		{
			name: "update not found",
			storeSetup: func() *store.MemoryTaskStore {
				return store.NewMemoryTaskStore()
			},
			idToUpdate:  "missing-id",
			newStatus:   "done",
			newResult:   "noop",
			expectErr:   true,
			errContains: "not found",
		},
		{
			name: "update with no actual change in status or result", // Verifies it still works
			storeSetup: func() *store.MemoryTaskStore {
				s := store.NewMemoryTaskStore()
				taskCopy := *taskToUpdate
				require.NoError(t, s.Save(&taskCopy))
				return s
			},
			idToUpdate: "task-update-1",
			newStatus:  initialStatus, // Same as original
			newResult:  initialResult, // Same as original
			expectErr:  false,
			expectedTask: &tasks.Task{
				ID:     "task-update-1",
				Type:   taskToUpdate.Type,
				Status: initialStatus,
				Result: initialResult,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			s := tc.storeSetup()
			err := s.Update(tc.idToUpdate, tc.newStatus, tc.newResult)

			if tc.expectErr {
				require.Error(t, err)
				if tc.errContains != "" {
					require.ErrorContains(t, err, tc.errContains)
				}
			} else {
				require.NoError(t, err)
				// Verify the task was updated correctly
				updatedTask, getErr := s.Get(tc.idToUpdate)
				require.NoError(t, getErr, "Failed to get task after update")
				require.NotNil(t, updatedTask)
				assert.Equal(t, tc.expectedTask.ID, updatedTask.ID)
				assert.Equal(t, tc.expectedTask.Type, updatedTask.Type) // Type should remain unchanged
				assert.Equal(t, tc.expectedTask.Status, updatedTask.Status)
				assert.Equal(t, tc.expectedTask.Result, updatedTask.Result)
			}
		})
	}
}
