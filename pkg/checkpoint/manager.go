package checkpoint

import (
	"docker-cr/pkg/docker"
	"docker-cr/pkg/utils"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/sirupsen/logrus"
)

type Manager struct {
	dockerManager *docker.Manager
	criuManager   *CRIUManager
	logger        *logrus.Logger
}

type CheckpointConfig struct {
	OutputDir       string `json:"output_dir"`
	CheckpointName  string `json:"checkpoint_name"`
	LeaveRunning    bool   `json:"leave_running"`
	TcpEstablished  bool   `json:"tcp_established"`
	FileLocks       bool   `json:"file_locks"`
	PreDump         bool   `json:"pre_dump"`
	LogLevel        int    `json:"log_level"`
	ManageCgroups   bool   `json:"manage_cgroups"`
	Shell           bool   `json:"shell"`
}

type CheckpointMetadata struct {
	ContainerState *docker.ContainerState `json:"container_state"`
	MountMappings  []docker.MountMapping  `json:"mount_mappings"`
	CheckpointPath string                 `json:"checkpoint_path"`
	CreatedAt      string                 `json:"created_at"`
	Version        string                 `json:"version"`
}

func NewManager(dockerManager *docker.Manager, logger *logrus.Logger) *Manager {
	return &Manager{
		dockerManager: dockerManager,
		criuManager:   NewCRIUManager(logger),
		logger:        logger,
	}
}

func (m *Manager) Checkpoint(containerName string, config CheckpointConfig) error {
	m.logger.Infof("Starting checkpoint of container: %s", containerName)

	// 1. Get container state from Docker
	state, err := m.dockerManager.GetContainerState(containerName)
	if err != nil {
		return fmt.Errorf("failed to get container state: %w", err)
	}

	m.logger.Infof("Container info - ID: %s, PID: %d", state.ID[:12], state.ProcessPID)

	// 2. Prepare checkpoint directory
	checkpointDir := filepath.Join(config.OutputDir, state.Name, config.CheckpointName)
	imagesDir := filepath.Join(checkpointDir, "images")

	if err := utils.EnsureDir(imagesDir); err != nil {
		return fmt.Errorf("failed to create checkpoint directory: %w", err)
	}

	m.logger.Infof("Checkpoint directory: %s", checkpointDir)

	// 3. Get mount mappings
	mountMappings, err := m.dockerManager.GetMountMappings(state)
	if err != nil {
		return fmt.Errorf("failed to get mount mappings: %w", err)
	}

	// 4. Validate mount sources and prepare external mounts
	if err := m.criuManager.ValidateMountSources(mountMappings); err != nil {
		return fmt.Errorf("mount validation failed: %w", err)
	}

	externalMounts := m.criuManager.BuildExternalMountMappings(mountMappings)

	// 5. Save mount mappings for restore
	mountMappingsFile := filepath.Join(checkpointDir, "mount_mappings.json")
	if err := m.SaveMountMappings(mountMappings, mountMappingsFile); err != nil {
		return fmt.Errorf("failed to save mount mappings: %w", err)
	}

	// 6. Save container metadata
	metadataFile := filepath.Join(checkpointDir, "container_metadata.json")
	if err := m.dockerManager.SaveContainerMetadata(state, metadataFile); err != nil {
		return fmt.Errorf("failed to save container metadata: %w", err)
	}

	// 7. Configure CRIU checkpoint options
	criuOpts := CheckpointOptions{
		WorkDir:         checkpointDir,
		ImagesDir:       imagesDir,
		LogFile:         filepath.Join(checkpointDir, "dump.log"),  // Use dump.log like working version
		LogLevel:        config.LogLevel,
		External:        externalMounts,
		ManageCgroups:   config.ManageCgroups,
		TcpEstablished:  config.TcpEstablished,
		FileLocks:       config.FileLocks,
		LeaveRunning:    config.LeaveRunning,
		Shell:           config.Shell,
		PreDump:         config.PreDump,
		TrackMem:        config.PreDump, // Enable memory tracking for pre-dump
	}

	// 8. Perform CRIU checkpoint
	if err := m.criuManager.CheckpointProcess(state.ProcessPID, criuOpts); err != nil {
		return fmt.Errorf("CRIU checkpoint failed: %w", err)
	}

	// 9. Save checkpoint metadata
	metadata := CheckpointMetadata{
		ContainerState: state,
		MountMappings:  mountMappings,
		CheckpointPath: checkpointDir,
		CreatedAt:      utils.GetCurrentTimestamp(),
		Version:        "1.0",
	}

	metadataPath := filepath.Join(checkpointDir, "checkpoint_metadata.json")
	if err := m.saveCheckpointMetadata(metadata, metadataPath); err != nil {
		return fmt.Errorf("failed to save checkpoint metadata: %w", err)
	}

	m.logger.Infof("Checkpoint completed successfully: %s", checkpointDir)
	return nil
}

func (m *Manager) ListCheckpointFiles(checkpointDir string) ([]string, error) {
	imagesDir := filepath.Join(checkpointDir, "images")
	if !utils.DirExists(imagesDir) {
		return nil, fmt.Errorf("checkpoint images directory not found: %s", imagesDir)
	}

	files, err := utils.ListFiles(imagesDir)
	if err != nil {
		return nil, fmt.Errorf("failed to list checkpoint files: %w", err)
	}

	var fileList []string
	for _, file := range files {
		fileList = append(fileList, fmt.Sprintf("%s (%d bytes)", file.Name(), file.Size()))
	}

	return fileList, nil
}

func (m *Manager) ValidateCheckpoint(checkpointDir string) error {
	// Check if checkpoint directory exists
	if !utils.DirExists(checkpointDir) {
		return fmt.Errorf("checkpoint directory does not exist: %s", checkpointDir)
	}

	// Check for required files
	requiredFiles := []string{
		"container_metadata.json",
		"mount_mappings.json",
		"checkpoint_metadata.json",
		"images",
	}

	for _, file := range requiredFiles {
		filePath := filepath.Join(checkpointDir, file)
		if file == "images" {
			if !utils.DirExists(filePath) {
				return fmt.Errorf("missing required directory: %s", file)
			}
		} else {
			if !utils.FileExists(filePath) {
				return fmt.Errorf("missing required file: %s", file)
			}
		}
	}

	// Check if images directory has content
	imagesDir := filepath.Join(checkpointDir, "images")
	files, err := utils.ListFiles(imagesDir)
	if err != nil {
		return fmt.Errorf("failed to list images directory: %w", err)
	}

	if len(files) == 0 {
		return fmt.Errorf("checkpoint images directory is empty")
	}

	m.logger.Infof("Checkpoint validation successful: %d image files found", len(files))
	return nil
}

func (m *Manager) GetCheckpointInfo(checkpointDir string) (*CheckpointMetadata, error) {
	metadataPath := filepath.Join(checkpointDir, "checkpoint_metadata.json")
	if !utils.FileExists(metadataPath) {
		return nil, fmt.Errorf("checkpoint metadata file not found: %s", metadataPath)
	}

	data, err := utils.ReadFile(metadataPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read checkpoint metadata: %w", err)
	}

	var metadata CheckpointMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("failed to parse checkpoint metadata: %w", err)
	}

	return &metadata, nil
}

func (m *Manager) SaveMountMappings(mappings []docker.MountMapping, filePath string) error {
	data, err := json.MarshalIndent(mappings, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal mount mappings: %w", err)
	}

	return utils.WriteFile(filePath, data)
}

func (m *Manager) LoadMountMappings(filePath string) ([]docker.MountMapping, error) {
	if !utils.FileExists(filePath) {
		return nil, fmt.Errorf("mount mappings file not found: %s", filePath)
	}

	data, err := utils.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read mount mappings: %w", err)
	}

	var mappings []docker.MountMapping
	if err := json.Unmarshal(data, &mappings); err != nil {
		return nil, fmt.Errorf("failed to parse mount mappings: %w", err)
	}

	return mappings, nil
}

func (m *Manager) saveCheckpointMetadata(metadata CheckpointMetadata, filePath string) error {
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal checkpoint metadata: %w", err)
	}

	return utils.WriteFile(filePath, data)
}

func (m *Manager) CheckCRIUSupport() error {
	return m.criuManager.CheckCRIUSupport()
}