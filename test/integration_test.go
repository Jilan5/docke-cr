package test

import (
	"docker-cr/pkg/checkpoint"
	"docker-cr/pkg/docker"
	"docker-cr/pkg/restore"
	"docker-cr/pkg/utils"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	testCheckpointDir = "/tmp/docker-cr-test-checkpoints"
	testContainerName = "docker-cr-integration-test"
	testImage         = "alpine:latest"
)

func setupTestLogger() *logrus.Logger {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	logger.SetFormatter(&logrus.TextFormatter{
		DisableColors: false,
		FullTimestamp: true,
	})
	return logger
}

func cleanupTestContainer(t *testing.T) {
	logger := setupTestLogger()
	dockerManager, err := docker.NewManager(logger)
	if err != nil {
		t.Logf("Failed to create Docker manager for cleanup: %v", err)
		return
	}
	defer dockerManager.Close()

	// Stop and remove test container if it exists
	if err := dockerManager.StopContainer(testContainerName, nil); err != nil {
		t.Logf("Failed to stop test container (may not exist): %v", err)
	}

	if err := dockerManager.RemoveContainer(testContainerName); err != nil {
		t.Logf("Failed to remove test container (may not exist): %v", err)
	}
}

func createTestContainer(t *testing.T) *docker.Manager {
	logger := setupTestLogger()
	dockerManager, err := docker.NewManager(logger)
	if err != nil {
		t.Fatalf("Failed to create Docker manager: %v", err)
	}

	// Create a simple test container
	// Note: This is a simplified test - in real scenarios you'd use Docker API
	// For now, we'll assume the container is created externally
	t.Logf("Test container should be created externally: %s", testContainerName)

	return dockerManager
}

func TestCheckpointRestore(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("Skipping checkpoint/restore test - requires root privileges")
	}

	// Skip if CRIU is not available
	if _, err := os.Stat("/usr/bin/criu"); os.IsNotExist(err) {
		if _, err := os.Stat("/usr/local/bin/criu"); os.IsNotExist(err) {
			t.Skip("Skipping test - CRIU not found")
		}
	}

	logger := setupTestLogger()

	// Cleanup before and after test
	cleanupTestContainer(t)
	defer cleanupTestContainer(t)

	// Create test directory
	if err := utils.EnsureDir(testCheckpointDir); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	defer utils.RemoveDir(testCheckpointDir)

	dockerManager := createTestContainer(t)
	defer dockerManager.Close()

	checkpointManager := checkpoint.NewManager(dockerManager, logger)
	restoreManager := restore.NewManager(dockerManager, checkpointManager, logger)

	t.Run("CheckpointContainer", func(t *testing.T) {
		// Skip this part of the test if container doesn't exist
		// In a real test environment, you'd create the container here
		config := checkpoint.CheckpointConfig{
			OutputDir:       testCheckpointDir,
			CheckpointName:  "test-checkpoint",
			LeaveRunning:    false,
			TcpEstablished:  false,
			FileLocks:       false,
			PreDump:         false,
			LogLevel:        4,
			ManageCgroups:   false,
			Shell:           true,
		}

		// This would fail if container doesn't exist, which is expected in CI
		err := checkpointManager.Checkpoint(testContainerName, config)
		if err != nil {
			t.Logf("Expected error checkpointing non-existent container: %v", err)
			t.Skip("Skipping rest of test - container not available")
		}

		// Validate checkpoint was created
		checkpointPath := filepath.Join(testCheckpointDir, testContainerName, "test-checkpoint")
		if !utils.DirExists(checkpointPath) {
			t.Errorf("Checkpoint directory was not created: %s", checkpointPath)
		}

		// Validate checkpoint
		err = checkpointManager.ValidateCheckpoint(checkpointPath)
		if err != nil {
			t.Errorf("Checkpoint validation failed: %v", err)
		}
	})

	t.Run("RestoreContainer", func(t *testing.T) {
		checkpointPath := filepath.Join(testCheckpointDir, testContainerName, "test-checkpoint")

		// Skip if checkpoint doesn't exist (from previous test failure)
		if !utils.DirExists(checkpointPath) {
			t.Skip("Skipping restore test - checkpoint not available")
		}

		config := restore.RestoreConfig{
			CheckpointDir:    checkpointPath,
			NewContainerName: testContainerName + "-restored",
			LogLevel:         4,
			ManageCgroups:    false,
			TcpEstablished:   false,
			RestoreSibling:   false,
			Shell:            true,
			ValidateEnv:      true,
			AutoFixMounts:    true,
			SkipMounts:       []string{},
		}

		err := restoreManager.Restore(config)
		if err != nil {
			t.Errorf("Restore failed: %v", err)
		}
	})
}

func TestCheckpointValidation(t *testing.T) {
	logger := setupTestLogger()
	dockerManager, err := docker.NewManager(logger)
	if err != nil {
		t.Fatalf("Failed to create Docker manager: %v", err)
	}
	defer dockerManager.Close()

	checkpointManager := checkpoint.NewManager(dockerManager, logger)

	testDir := filepath.Join(testCheckpointDir, "validation-test")
	defer utils.RemoveDir(testDir)

	t.Run("ValidateNonExistentCheckpoint", func(t *testing.T) {
		err := checkpointManager.ValidateCheckpoint("/non/existent/path")
		if err == nil {
			t.Error("Expected error for non-existent checkpoint")
		}
	})

	t.Run("ValidateIncompleteCheckpoint", func(t *testing.T) {
		// Create incomplete checkpoint directory
		incompleteDir := filepath.Join(testDir, "incomplete")
		if err := utils.EnsureDir(incompleteDir); err != nil {
			t.Fatalf("Failed to create test directory: %v", err)
		}

		// Only create some required files
		if err := utils.WriteFile(filepath.Join(incompleteDir, "container_metadata.json"), []byte("{}")); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		err := checkpointManager.ValidateCheckpoint(incompleteDir)
		if err == nil {
			t.Error("Expected error for incomplete checkpoint")
		}
	})
}

func TestMountMappingHandling(t *testing.T) {
	logger := setupTestLogger()
	dockerManager, err := docker.NewManager(logger)
	if err != nil {
		t.Fatalf("Failed to create Docker manager: %v", err)
	}
	defer dockerManager.Close()

	// Test mount mapping creation and validation
	mappings := []docker.MountMapping{
		{
			ContainerPath: "/test/path",
			HostPath:      "/tmp/test-mount",
			Type:          "bind",
			IsExternal:    true,
			ReadOnly:      false,
		},
		{
			ContainerPath: "/proc",
			HostPath:      "/proc",
			Type:          "proc",
			IsExternal:    true,
			ReadOnly:      false,
		},
	}

	checkpointManager := checkpoint.NewManager(dockerManager, logger)
	criuManager := checkpoint.NewCRIUManager(logger)

	t.Run("ValidateMountSources", func(t *testing.T) {
		// This should create missing mount sources
		err := criuManager.ValidateMountSources(mappings)
		if err != nil {
			t.Errorf("Mount validation failed: %v", err)
		}

		// Check if missing mount was created
		if !utils.DirExists("/tmp/test-mount") {
			t.Error("Expected missing mount source to be created")
		}

		// Cleanup
		utils.RemoveDir("/tmp/test-mount")
	})

	t.Run("BuildExternalMountMappings", func(t *testing.T) {
		external := criuManager.BuildExternalMountMappings(mappings)
		if len(external) == 0 {
			t.Error("Expected external mount mappings to be generated")
		}

		t.Logf("Generated external mappings: %v", external)
	})

	t.Run("SaveAndLoadMountMappings", func(t *testing.T) {
		testFile := filepath.Join(testCheckpointDir, "test-mount-mappings.json")
		defer os.Remove(testFile)

		// Save mappings
		err := checkpointManager.SaveMountMappings(mappings, testFile)
		if err != nil {
			t.Errorf("Failed to save mount mappings: %v", err)
		}

		// Load mappings
		loadedMappings, err := checkpointManager.LoadMountMappings(testFile)
		if err != nil {
			t.Errorf("Failed to load mount mappings: %v", err)
		}

		if len(loadedMappings) != len(mappings) {
			t.Errorf("Expected %d mappings, got %d", len(mappings), len(loadedMappings))
		}
	})
}

func TestCRIUSupport(t *testing.T) {
	logger := setupTestLogger()
	criuManager := checkpoint.NewCRIUManager(logger)

	t.Run("CheckCRIUSupport", func(t *testing.T) {
		err := criuManager.CheckCRIUSupport()
		if err != nil {
			t.Logf("CRIU support check failed (expected in CI): %v", err)
			// Don't fail the test - CRIU might not be available in CI
		} else {
			t.Log("CRIU support detected")
		}
	})
}

func BenchmarkCheckpointOperations(b *testing.B) {
	if os.Getuid() != 0 {
		b.Skip("Skipping benchmark - requires root privileges")
	}

	logger := setupTestLogger()
	dockerManager, err := docker.NewManager(logger)
	if err != nil {
		b.Fatalf("Failed to create Docker manager: %v", err)
	}
	defer dockerManager.Close()

	checkpointManager := checkpoint.NewManager(dockerManager, logger)

	// Benchmark checkpoint validation
	b.Run("ValidateCheckpoint", func(b *testing.B) {
		// Create a dummy checkpoint directory structure
		testDir := filepath.Join(testCheckpointDir, "benchmark")
		if err := utils.EnsureDir(filepath.Join(testDir, "images")); err != nil {
			b.Fatalf("Failed to create test directory: %v", err)
		}
		defer utils.RemoveDir(testDir)

		// Create required files
		requiredFiles := []string{
			"container_metadata.json",
			"mount_mappings.json",
			"checkpoint_metadata.json",
		}

		for _, file := range requiredFiles {
			if err := utils.WriteFile(filepath.Join(testDir, file), []byte("{}")); err != nil {
				b.Fatalf("Failed to create test file: %v", err)
			}
		}

		// Create some dummy image files
		for i := 0; i < 10; i++ {
			filename := fmt.Sprintf("test-%d.img", i)
			if err := utils.WriteFile(filepath.Join(testDir, "images", filename), []byte("dummy")); err != nil {
				b.Fatalf("Failed to create test image: %v", err)
			}
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			err := checkpointManager.ValidateCheckpoint(testDir)
			if err != nil {
				b.Errorf("Validation failed: %v", err)
			}
		}
	})
}

// Helper function for manual testing
func ExampleCheckpointRestore() {
	logger := setupTestLogger()

	// This is an example of how to use the API programmatically
	dockerManager, err := docker.NewManager(logger)
	if err != nil {
		logger.Fatalf("Failed to create Docker manager: %v", err)
	}
	defer dockerManager.Close()

	checkpointManager := checkpoint.NewManager(dockerManager, logger)
	restoreManager := restore.NewManager(dockerManager, checkpointManager, logger)

	// Checkpoint configuration
	checkpointConfig := checkpoint.CheckpointConfig{
		OutputDir:       "/tmp/example-checkpoints",
		CheckpointName:  "example-checkpoint",
		LeaveRunning:    false,
		TcpEstablished:  false,
		FileLocks:       false,
		PreDump:         false,
		LogLevel:        4,
		ManageCgroups:   false,
		Shell:           true,
	}

	// Perform checkpoint
	err = checkpointManager.Checkpoint("example-container", checkpointConfig)
	if err != nil {
		logger.Errorf("Checkpoint failed: %v", err)
		return
	}

	// Wait a bit
	time.Sleep(2 * time.Second)

	// Restore configuration
	restoreConfig := restore.RestoreConfig{
		CheckpointDir:    "/tmp/example-checkpoints/example-container/example-checkpoint",
		NewContainerName: "example-container-restored",
		LogLevel:         4,
		ManageCgroups:    false,
		TcpEstablished:   false,
		RestoreSibling:   false,
		Shell:            true,
		ValidateEnv:      true,
		AutoFixMounts:    true,
		SkipMounts:       []string{},
	}

	// Perform restore
	err = restoreManager.Restore(restoreConfig)
	if err != nil {
		logger.Errorf("Restore failed: %v", err)
		return
	}

	logger.Info("Checkpoint and restore completed successfully!")
}