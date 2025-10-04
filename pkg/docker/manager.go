package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/sirupsen/logrus"
)

type Manager struct {
	client *client.Client
	logger *logrus.Logger
}

type ContainerState struct {
	ID            string                          `json:"id"`
	Name          string                          `json:"name"`
	Image         string                          `json:"image"`
	Config        *container.Config               `json:"config"`
	HostConfig    *container.HostConfig           `json:"host_config"`
	NetworkConfig map[string]*network.EndpointSettings `json:"network_config"`
	Mounts        []types.MountPoint              `json:"mounts"`
	ProcessPID    int                             `json:"process_pid"`
	Created       time.Time                       `json:"created"`
	RootFS        string                          `json:"rootfs"`
	Runtime       string                          `json:"runtime"`
	BundlePath    string                          `json:"bundle_path"`
	CgroupPath    string                          `json:"cgroup_path"`
	Namespaces    map[string]string               `json:"namespaces"`
	Environment   map[string]string               `json:"environment"`
	Labels        map[string]string               `json:"labels"`
}

type MountMapping struct {
	ContainerPath string `json:"container_path"`
	HostPath      string `json:"host_path"`
	Type          string `json:"type"`
	Options       string `json:"options"`
	IsExternal    bool   `json:"is_external"`
	ReadOnly      bool   `json:"read_only"`
}

func NewManager(logger *logrus.Logger) (*Manager, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	return &Manager{
		client: cli,
		logger: logger,
	}, nil
}

func (m *Manager) GetContainerState(nameOrID string) (*ContainerState, error) {
	ctx := context.Background()

	containerJSON, err := m.client.ContainerInspect(ctx, nameOrID)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect container: %w", err)
	}

	if !containerJSON.State.Running {
		return nil, fmt.Errorf("container %s is not running", nameOrID)
	}

	runtime := containerJSON.HostConfig.Runtime
	if runtime == "" {
		runtime = "runc"
	}

	// Parse environment variables
	envMap := make(map[string]string)
	for _, env := range containerJSON.Config.Env {
		if parts := strings.SplitN(env, "=", 2); len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}

	// Parse labels
	labelMap := make(map[string]string)
	if containerJSON.Config.Labels != nil {
		labelMap = containerJSON.Config.Labels
	}

	// Parse the created time
	createdTime, err := time.Parse(time.RFC3339Nano, containerJSON.Created)
	if err != nil {
		// Fallback to current time if parsing fails
		createdTime = time.Now()
	}

	state := &ContainerState{
		ID:            containerJSON.ID,
		Name:          strings.TrimPrefix(containerJSON.Name, "/"),
		Image:         containerJSON.Config.Image,
		Config:        containerJSON.Config,
		HostConfig:    containerJSON.HostConfig,
		NetworkConfig: containerJSON.NetworkSettings.Networks,
		Mounts:        containerJSON.Mounts,
		ProcessPID:    containerJSON.State.Pid,
		Created:       createdTime,
		RootFS:        containerJSON.GraphDriver.Data["MergedDir"],
		Runtime:       runtime,
		BundlePath:    fmt.Sprintf("/run/docker/runtime-%s/moby/%s", runtime, containerJSON.ID),
		CgroupPath:    containerJSON.HostConfig.CgroupParent,
		Namespaces:    make(map[string]string),
		Environment:   envMap,
		Labels:        labelMap,
	}

	// Get namespace information
	nsTypes := []string{"ipc", "mnt", "net", "pid", "user", "uts", "cgroup"}
	for _, ns := range nsTypes {
		state.Namespaces[ns] = fmt.Sprintf("/proc/%d/ns/%s", state.ProcessPID, ns)
	}

	return state, nil
}

func (m *Manager) GetMountMappings(state *ContainerState) ([]MountMapping, error) {
	var mappings []MountMapping

	for _, mount := range state.Mounts {
		mapping := MountMapping{
			ContainerPath: mount.Destination,
			HostPath:      mount.Source,
			Type:          string(mount.Type),
			Options:       mount.Mode,
			IsExternal:    true,
			ReadOnly:      !mount.RW,
		}

		mappings = append(mappings, mapping)
	}

	// Add standard system mounts that need external mapping
	systemMounts := []MountMapping{
		{ContainerPath: "/proc", HostPath: "/proc", Type: "proc", IsExternal: true},
		{ContainerPath: "/sys", HostPath: "/sys", Type: "sysfs", IsExternal: true},
		{ContainerPath: "/dev", HostPath: "/dev", Type: "devtmpfs", IsExternal: true},
		{ContainerPath: "/dev/shm", HostPath: "/dev/shm", Type: "tmpfs", IsExternal: true},
		{ContainerPath: "/dev/pts", HostPath: "/dev/pts", Type: "devpts", IsExternal: true},
		{ContainerPath: "/dev/mqueue", HostPath: "/dev/mqueue", Type: "mqueue", IsExternal: true},
		{ContainerPath: "/sys/fs/cgroup", HostPath: "/sys/fs/cgroup", Type: "cgroup", IsExternal: true},
	}

	mappings = append(mappings, systemMounts...)

	return mappings, nil
}

func (m *Manager) CreateRestoreContainer(originalState *ContainerState, newName string) (string, error) {
	ctx := context.Background()

	// Create container config based on original but simplified
	config := &container.Config{
		Image:        originalState.Image,
		Cmd:          originalState.Config.Cmd,
		Entrypoint:   originalState.Config.Entrypoint,
		Env:          originalState.Config.Env,
		WorkingDir:   originalState.Config.WorkingDir,
		User:         originalState.Config.User,
		ExposedPorts: originalState.Config.ExposedPorts,
		Labels:       originalState.Config.Labels,
		Tty:          originalState.Config.Tty,
		OpenStdin:    originalState.Config.OpenStdin,
		StdinOnce:    originalState.Config.StdinOnce,
	}

	// Simplified host config for restore
	hostConfig := &container.HostConfig{
		Privileged:  true,
		PidMode:     "host",
		IpcMode:     "host",
		NetworkMode: "host",
		SecurityOpt: []string{"seccomp=unconfined"},
		CapAdd:      []string{"SYS_PTRACE", "SYS_ADMIN"},
		// Copy important settings from original
		Resources: originalState.HostConfig.Resources,
		RestartPolicy: originalState.HostConfig.RestartPolicy,
	}

	resp, err := m.client.ContainerCreate(ctx, config, hostConfig, nil, nil, newName)
	if err != nil {
		return "", fmt.Errorf("failed to create restore container: %w", err)
	}

	m.logger.Infof("Created restore container: %s", resp.ID[:12])
	return resp.ID, nil
}

func (m *Manager) GetContainerPID(containerID string) (int, error) {
	ctx := context.Background()

	containerJSON, err := m.client.ContainerInspect(ctx, containerID)
	if err != nil {
		return 0, fmt.Errorf("failed to inspect container: %w", err)
	}

	if containerJSON.State.Pid == 0 {
		return 0, fmt.Errorf("container has no PID (not running)")
	}

	return containerJSON.State.Pid, nil
}

func (m *Manager) StartContainer(containerID string) error {
	ctx := context.Background()

	if err := m.client.ContainerStart(ctx, containerID, types.ContainerStartOptions{}); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	return nil
}

func (m *Manager) StopContainer(containerID string, timeout *int) error {
	ctx := context.Background()

	stopOptions := container.StopOptions{}
	if timeout != nil {
		stopOptions.Timeout = timeout
	}

	if err := m.client.ContainerStop(ctx, containerID, stopOptions); err != nil {
		return fmt.Errorf("failed to stop container: %w", err)
	}

	return nil
}

func (m *Manager) RemoveContainer(containerID string) error {
	ctx := context.Background()

	if err := m.client.ContainerRemove(ctx, containerID, types.ContainerRemoveOptions{Force: true}); err != nil {
		return fmt.Errorf("failed to remove container: %w", err)
	}

	return nil
}

func (m *Manager) SaveContainerMetadata(state *ContainerState, filePath string) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal container metadata: %w", err)
	}

	if err := writeFile(filePath, data); err != nil {
		return fmt.Errorf("failed to write metadata file: %w", err)
	}

	return nil
}

func (m *Manager) LoadContainerMetadata(filePath string) (*ContainerState, error) {
	data, err := readFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata file: %w", err)
	}

	var state ContainerState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal container metadata: %w", err)
	}

	return &state, nil
}

func (m *Manager) GetContainerLogs(containerID string, tail string) (string, error) {
	ctx := context.Background()

	options := types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       tail,
	}

	logs, err := m.client.ContainerLogs(ctx, containerID, options)
	if err != nil {
		return "", fmt.Errorf("failed to get container logs: %w", err)
	}
	defer logs.Close()

	buf := make([]byte, 4096)
	n, _ := logs.Read(buf)
	return string(buf[:n]), nil
}

func (m *Manager) Close() error {
	return m.client.Close()
}