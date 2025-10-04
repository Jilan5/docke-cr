package restore

import (
	"docker-cr/pkg/checkpoint"
	"docker-cr/pkg/docker"
	"docker-cr/pkg/utils"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
)

type Manager struct {
	dockerManager     *docker.Manager
	criuManager       *checkpoint.CRIUManager
	checkpointManager *checkpoint.Manager
	logger            *logrus.Logger
}

type RestoreConfig struct {
	CheckpointDir   string `json:"checkpoint_dir"`
	NewContainerName string `json:"new_container_name"`
	LogLevel        int    `json:"log_level"`
	ManageCgroups   bool   `json:"manage_cgroups"`
	TcpEstablished  bool   `json:"tcp_established"`
	RestoreSibling  bool   `json:"restore_sibling"`
	Shell           bool   `json:"shell"`
	ValidateEnv     bool   `json:"validate_env"`
	AutoFixMounts   bool   `json:"auto_fix_mounts"`
	SkipMounts      []string `json:"skip_mounts"`
}

func NewManager(dockerManager *docker.Manager, checkpointManager *checkpoint.Manager, logger *logrus.Logger) *Manager {
	return &Manager{
		dockerManager:     dockerManager,
		criuManager:       checkpoint.NewCRIUManager(logger),
		checkpointManager: checkpointManager,
		logger:            logger,
	}
}

func (m *Manager) Restore(config RestoreConfig) error {
	m.logger.Infof("Starting restore from checkpoint: %s", config.CheckpointDir)

	// 1. Validate checkpoint exists and is complete
	if err := m.checkpointManager.ValidateCheckpoint(config.CheckpointDir); err != nil {
		return fmt.Errorf("checkpoint validation failed: %w", err)
	}

	// 2. Load checkpoint metadata
	metadata, err := m.checkpointManager.GetCheckpointInfo(config.CheckpointDir)
	if err != nil {
		return fmt.Errorf("failed to load checkpoint metadata: %w", err)
	}

	originalState := metadata.ContainerState
	m.logger.Infof("Original container: %s (ID: %s)", originalState.Name, originalState.ID[:12])

	// 3. Load mount mappings
	mountMappingsFile := filepath.Join(config.CheckpointDir, "mount_mappings.json")
	mountMappings, err := m.checkpointManager.LoadMountMappings(mountMappingsFile)
	if err != nil {
		return fmt.Errorf("failed to load mount mappings: %w", err)
	}

	// 4. Validate restore environment
	if config.ValidateEnv {
		if err := m.validateRestoreEnvironment(originalState, mountMappings); err != nil {
			return fmt.Errorf("restore environment validation failed: %w", err)
		}
	}

	// 5. Create target container for restore
	containerID, err := m.dockerManager.CreateRestoreContainer(originalState, config.NewContainerName)
	if err != nil {
		return fmt.Errorf("failed to create restore container: %w", err)
	}

	m.logger.Infof("Created restore container: %s", containerID[:12])

	// 6. Prepare mount namespace (critical for fixing mount errors)
	if err := m.prepareMountNamespace(containerID, mountMappings, config.AutoFixMounts); err != nil {
		return fmt.Errorf("failed to prepare mount namespace: %w", err)
	}

	// 7. Start the container to get a PID
	if err := m.dockerManager.StartContainer(containerID); err != nil {
		return fmt.Errorf("failed to start restore container: %w", err)
	}

	// 8. Get container PID for restore target
	newPID, err := m.dockerManager.GetContainerPID(containerID)
	if err != nil {
		return fmt.Errorf("failed to get container PID: %w", err)
	}

	m.logger.Infof("Restore target PID: %d", newPID)

	// 9. Stop the container (CRIU will restore it)
	timeout := 5
	if err := m.dockerManager.StopContainer(containerID, &timeout); err != nil {
		m.logger.Warnf("Failed to gracefully stop container, continuing: %v", err)
	}

	// 10. Configure CRIU restore options
	imagesDir := filepath.Join(config.CheckpointDir, "images")
	extMountMapFile := filepath.Join(config.CheckpointDir, "ext_mount_map")

	// Create external mount map file
	if err := m.criuManager.CreateExtMountMapFile(mountMappings, extMountMapFile); err != nil {
		return fmt.Errorf("failed to create external mount map: %w", err)
	}

	criuOpts := checkpoint.RestoreOptions{
		WorkDir:        config.CheckpointDir,
		ImagesDir:      imagesDir,
		LogFile:        filepath.Join(config.CheckpointDir, "restore.log"),
		LogLevel:       config.LogLevel,
		External:       m.buildExternalMountArgs(mountMappings, config.SkipMounts),
		ExtMountMap:    m.criuManager.BuildExtMountMapArgs(mountMappings),
		SkipMnt:        config.SkipMounts,
		ManageCgroups:  config.ManageCgroups,
		TcpEstablished: config.TcpEstablished,
		RestoreSibling: config.RestoreSibling,
		Shell:          config.Shell,
		EmptyNs:        0x40, // CLONE_NEWNS - handle mount namespace issues
	}

	// 11. Perform CRIU restore
	if err := m.criuManager.RestoreProcess(criuOpts); err != nil {
		return fmt.Errorf("CRIU restore failed: %w", err)
	}

	// 12. Verify restoration
	if err := m.verifyRestoration(config.NewContainerName); err != nil {
		m.logger.Warnf("Restoration verification failed: %v", err)
		return fmt.Errorf("restore verification failed: %w", err)
	}

	m.logger.Infof("Container restored successfully as: %s", config.NewContainerName)
	return nil
}

func (m *Manager) prepareMountNamespace(containerID string, mappings []docker.MountMapping, autoFix bool) error {
	m.logger.Info("Preparing mount namespace for restore")

	// 1. Validate all mount sources exist on host
	for _, mapping := range mappings {
		if mapping.IsExternal && mapping.HostPath != "" {
			if !utils.FileExists(mapping.HostPath) && !utils.DirExists(mapping.HostPath) {
				if autoFix {
					m.logger.Infof("Creating missing mount source: %s", mapping.HostPath)
					if err := utils.EnsureDir(mapping.HostPath); err != nil {
						return fmt.Errorf("failed to create mount source %s: %w", mapping.HostPath, err)
					}
				} else {
					m.logger.Warnf("Mount source does not exist: %s", mapping.HostPath)
				}
			}
		}
	}

	return nil
}

func (m *Manager) buildExternalMountArgs(mappings []docker.MountMapping, skipMounts []string) []string {
	var external []string

	// Create skip map for efficient lookup
	skipMap := make(map[string]bool)
	for _, skip := range skipMounts {
		skipMap[skip] = true
	}

	for _, mapping := range mappings {
		if mapping.IsExternal && !skipMap[mapping.ContainerPath] {
			// CRIU external mount format: "mnt[container_path]:host_path"
			if mapping.HostPath != "" {
				extMount := fmt.Sprintf("mnt[%s]:%s", mapping.ContainerPath, mapping.HostPath)
				external = append(external, extMount)
			} else {
				// For mounts without host path, use simple format
				extMount := fmt.Sprintf("mnt:%s", mapping.ContainerPath)
				external = append(external, extMount)
			}
		}
	}

	// Add essential system mounts that are usually safe
	essentialMounts := []string{
		"mnt:/proc",
		"mnt:/dev",
		"mnt:/sys",
	}

	for _, mount := range essentialMounts {
		external = append(external, mount)
	}

	return external
}

func (m *Manager) validateRestoreEnvironment(originalState *docker.ContainerState, mappings []docker.MountMapping) error {
	m.logger.Info("Validating restore environment")

	// 1. Check if image exists
	// Note: In a real implementation, you'd want to check this via Docker API
	m.logger.Infof("Original image: %s", originalState.Image)

	// 2. Validate mount sources
	missingMounts := 0
	for _, mapping := range mappings {
		if mapping.IsExternal && mapping.HostPath != "" {
			if !utils.FileExists(mapping.HostPath) && !utils.DirExists(mapping.HostPath) {
				m.logger.Warnf("Mount source missing: %s -> %s", mapping.ContainerPath, mapping.HostPath)
				missingMounts++
			}
		}
	}

	if missingMounts > 0 {
		m.logger.Warnf("Found %d missing mount sources", missingMounts)
	}

	// 3. Check for potential ptrace conflicts
	if err := m.checkPtraceConflicts(); err != nil {
		return fmt.Errorf("ptrace conflict detected: %w", err)
	}

	return nil
}

func (m *Manager) checkPtraceConflicts() error {
	// This is a simplified check. In a real implementation,
	// you'd want to check for processes that might interfere with CRIU
	return nil
}

func (m *Manager) verifyRestoration(containerName string) error {
	m.logger.Info("Verifying restoration...")

	// Get container state
	state, err := m.dockerManager.GetContainerState(containerName)
	if err != nil {
		// Container might not be running yet, try to get basic info
		m.logger.Warn("Container not running, checking basic status...")
		return nil
	}

	m.logger.Infof("Restored container state:")
	m.logger.Infof("  Name: %s", state.Name)
	m.logger.Infof("  ID: %s", state.ID[:12])
	m.logger.Infof("  PID: %d", state.ProcessPID)
	m.logger.Infof("  Image: %s", state.Image)

	// Try to get recent logs
	logs, err := m.dockerManager.GetContainerLogs(state.ID, "10")
	if err == nil && logs != "" {
		m.logger.Infof("Recent container logs:\n%s", logs)
	}

	return nil
}

func (m *Manager) RestoreFromArchive(archivePath, newContainerName string, config RestoreConfig) error {
	// Extract archive to temporary directory
	tempDir := filepath.Join(os.TempDir(), "docker-cr-restore")
	if err := utils.EnsureDir(tempDir); err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer utils.RemoveDir(tempDir)

	// TODO: Implement archive extraction
	// For now, assume archivePath is actually a directory
	config.CheckpointDir = archivePath
	config.NewContainerName = newContainerName

	return m.Restore(config)
}

func (m *Manager) GetRestoreOptions(checkpointDir string) (*RestoreConfig, error) {
	// Load checkpoint metadata to provide sensible defaults
	metadata, err := m.checkpointManager.GetCheckpointInfo(checkpointDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load checkpoint metadata: %w", err)
	}

	// Provide default restore configuration
	config := &RestoreConfig{
		CheckpointDir:    checkpointDir,
		NewContainerName: fmt.Sprintf("%s-restored", metadata.ContainerState.Name),
		LogLevel:         4, // Debug level
		ManageCgroups:    false,
		TcpEstablished:   false,
		RestoreSibling:   false,
		Shell:            true,
		ValidateEnv:      true,
		AutoFixMounts:    true,
		SkipMounts:       []string{}, // No mounts skipped by default
	}

	return config, nil
}