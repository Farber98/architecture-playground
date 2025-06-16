package store_test

import (
	"context"
	"encoding/json"
	"task-orchestrator/tasks"
	"task-orchestrator/tasks/store"
	"testing"

	"github.com/stretchr/testify/require"
	"gotest.tools/v3/assert"
)

func newTestTask(id string) *tasks.Task {
	return &tasks.Task{
		ID:      id,
		Type:    "print",
		Status:  tasks.StatusSubmitted, // Use TaskStatus constant instead of string
		Result:  "",
		Payload: json.RawMessage(`{"message":"test"}`), // Add valid payload
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
				got, err := s.Get(context.Background(), taskID)
				require.NoError(t, err)
				assert.Equal(t, task1.ID, got.ID)
				assert.Equal(t, task1.Status, got.Status)
			},
		},
		{
			name: "save duplicate",
			storeSetup: func() *store.MemoryTaskStore {
				s := store.NewMemoryTaskStore()
				err := s.Save(context.Background(), taskExisting)
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
			err := s.Save(context.Background(), tc.taskToSave)

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
				require.NoError(t, s.Save(context.Background(), task1))
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
				require.NoError(t, s.Save(context.Background(), taskForCopyTest))
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
			got, err := s.Get(context.Background(), tc.idToGet)

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
					got.Status = tasks.StatusDone
					got.Result = "modified copy"

					// Get the task again from the store
					original, errGetAgain := s.Get(context.Background(), tc.idToGet)
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

	testCases := []struct {
		name         string
		storeSetup   func() *store.MemoryTaskStore
		idToUpdate   string
		newStatus    tasks.TaskStatus
		newResult    string
		expectErr    bool
		errContains  string
		expectedTask *tasks.Task // Expected state after update
	}{
		{
			name: "successful update: submitted to running",
			storeSetup: func() *store.MemoryTaskStore {
				s := store.NewMemoryTaskStore()
				// Save a copy to avoid modifying taskToUpdate directly in setup
				taskCopy := *taskToUpdate
				require.NoError(t, s.Save(context.Background(), &taskCopy))
				return s
			},
			idToUpdate: "task-update-1",
			newStatus:  tasks.StatusRunning,
			newResult:  "task is now running",
			expectErr:  false,
			expectedTask: &tasks.Task{
				ID:     "task-update-1",
				Type:   taskToUpdate.Type,
				Status: tasks.StatusRunning,
				Result: "task is now running",
			},
		},
		{
			name: "successful update: running to done",
			storeSetup: func() *store.MemoryTaskStore {
				s := store.NewMemoryTaskStore()
				// Create task and advance it to running state
				task := newTestTask("task-update-1")
				require.NoError(t, s.Save(context.Background(), task))
				require.NoError(t, s.Update(context.Background(), "task-update-1", tasks.StatusRunning, "now running"))
				return s
			},
			idToUpdate: "task-update-1",
			newStatus:  tasks.StatusDone,
			newResult:  "task completed successfully",
			expectErr:  false,
			expectedTask: &tasks.Task{
				ID:     "task-update-1",
				Type:   taskToUpdate.Type,
				Status: tasks.StatusDone,
				Result: "task completed successfully",
			},
		},
		{
			name: "update not found",
			storeSetup: func() *store.MemoryTaskStore {
				return store.NewMemoryTaskStore()
			},
			idToUpdate:  "missing-id",
			newStatus:   tasks.StatusDone,
			newResult:   "noop",
			expectErr:   true,
			errContains: "not found",
		},
		{
			name: "store trusts orchestrator: submitted to done (previously invalid)",
			storeSetup: func() *store.MemoryTaskStore {
				s := store.NewMemoryTaskStore()
				taskCopy := *taskToUpdate
				require.NoError(t, s.Save(context.Background(), &taskCopy))
				return s
			},
			idToUpdate: "task-update-1",
			newStatus:  tasks.StatusDone,
			newResult:  "orchestrator managed this transition",
			expectErr:  false,
			expectedTask: &tasks.Task{
				ID:     "task-update-1",
				Type:   taskToUpdate.Type,
				Status: tasks.StatusDone,
				Result: "orchestrator managed this transition",
			},
		},
		{
			name: "store trusts orchestrator: same status update",
			storeSetup: func() *store.MemoryTaskStore {
				s := store.NewMemoryTaskStore()
				taskCopy := *taskToUpdate
				require.NoError(t, s.Save(context.Background(), &taskCopy))
				return s
			},
			idToUpdate: "task-update-1",
			newStatus:  tasks.StatusSubmitted, // Same as original
			newResult:  "orchestrator says this is fine",
			expectErr:  false,
			expectedTask: &tasks.Task{
				ID:     "task-update-1",
				Type:   taskToUpdate.Type,
				Status: tasks.StatusSubmitted,
				Result: "orchestrator says this is fine",
			},
		},
		{
			name: "store trusts orchestrator: any transition",
			storeSetup: func() *store.MemoryTaskStore {
				s := store.NewMemoryTaskStore()
				task := newTestTask("task-update-1")
				// Set to done first
				task.Status = tasks.StatusDone
				require.NoError(t, s.Save(context.Background(), task))
				return s
			},
			idToUpdate: "task-update-1",
			newStatus:  tasks.StatusRunning, // "Backwards" transition
			newResult:  "orchestrator manages all transitions",
			expectErr:  false,
			expectedTask: &tasks.Task{
				ID:     "task-update-1",
				Type:   taskToUpdate.Type,
				Status: tasks.StatusRunning,
				Result: "orchestrator manages all transitions",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			s := tc.storeSetup()
			err := s.Update(context.Background(), tc.idToUpdate, tc.newStatus, tc.newResult)

			if tc.expectErr {
				require.Error(t, err)
				if tc.errContains != "" {
					require.ErrorContains(t, err, tc.errContains)
				}
			} else {
				require.NoError(t, err)
				// Verify the task was updated correctly
				updatedTask, getErr := s.Get(context.Background(), tc.idToUpdate)
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
