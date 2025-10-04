package inspect

import (
	"docker-cr/pkg/checkpoint"
	"docker-cr/pkg/docker"
	"docker-cr/pkg/utils"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
)

type Analyzer struct {
	logger *logrus.Logger
}

type ProcessInfo struct {
	PID             int                    `json:"pid"`
	PPID            int                    `json:"ppid"`
	Command         string                 `json:"command"`
	Args            []string               `json:"args"`
	Environment     map[string]string      `json:"environment"`
	WorkingDir      string                 `json:"working_dir"`
	FileDescriptors []FileDescriptor       `json:"file_descriptors"`
	Sockets         []SocketInfo           `json:"sockets"`
	MemoryMaps      []MemoryMap            `json:"memory_maps"`
	Children        []ProcessInfo          `json:"children"`
	State           string                 `json:"state"`
	StartTime       string                 `json:"start_time"`
}

type FileDescriptor struct {
	FD      int    `json:"fd"`
	Type    string `json:"type"`
	Path    string `json:"path"`
	Mode    string `json:"mode"`
	Flags   string `json:"flags"`
	Pos     int64  `json:"pos"`
	IsPipe  bool   `json:"is_pipe"`
	IsSocket bool  `json:"is_socket"`
}

type SocketInfo struct {
	FD          int    `json:"fd"`
	Type        string `json:"type"`        // TCP, UDP, UNIX
	Family      string `json:"family"`      // AF_INET, AF_INET6, AF_UNIX
	State       string `json:"state"`       // ESTABLISHED, LISTEN, etc.
	LocalAddr   string `json:"local_addr"`
	LocalPort   int    `json:"local_port"`
	RemoteAddr  string `json:"remote_addr"`
	RemotePort  int    `json:"remote_port"`
	SendBuffer  int    `json:"send_buffer"`
	RecvBuffer  int    `json:"recv_buffer"`
	Protocol    string `json:"protocol"`
}

type MemoryMap struct {
	StartAddr   string `json:"start_addr"`
	EndAddr     string `json:"end_addr"`
	Permissions string `json:"permissions"`
	Offset      string `json:"offset"`
	Device      string `json:"device"`
	Inode       string `json:"inode"`
	Path        string `json:"path"`
	Size        int64  `json:"size"`
}

type CheckpointAnalysis struct {
	Metadata      *checkpoint.CheckpointMetadata `json:"metadata"`
	ProcessTree   *ProcessInfo                   `json:"process_tree"`
	MountMappings []docker.MountMapping          `json:"mount_mappings"`
	NetworkInfo   *NetworkInfo                   `json:"network_info"`
	ResourceUsage *ResourceUsage                 `json:"resource_usage"`
	CRIUInfo      *CRIUInfo                      `json:"criu_info"`
}

type NetworkInfo struct {
	Interfaces []NetworkInterface `json:"interfaces"`
	Routes     []Route            `json:"routes"`
	IPTables   []IPTableRule      `json:"iptables"`
}

type NetworkInterface struct {
	Name    string   `json:"name"`
	Type    string   `json:"type"`
	MTU     int      `json:"mtu"`
	State   string   `json:"state"`
	Addrs   []string `json:"addrs"`
	MAC     string   `json:"mac"`
}

type Route struct {
	Destination string `json:"destination"`
	Gateway     string `json:"gateway"`
	Interface   string `json:"interface"`
	Metric      int    `json:"metric"`
}

type IPTableRule struct {
	Table  string `json:"table"`
	Chain  string `json:"chain"`
	Rule   string `json:"rule"`
	Target string `json:"target"`
}

type ResourceUsage struct {
	MemoryUsage int64              `json:"memory_usage"`
	CPUTime     string             `json:"cpu_time"`
	OpenFiles   int                `json:"open_files"`
	Processes   int                `json:"processes"`
	Threads     int                `json:"threads"`
	Cgroups     map[string]string  `json:"cgroups"`
}

type CRIUInfo struct {
	Version        string            `json:"version"`
	Features       []string          `json:"features"`
	LogPath        string            `json:"log_path"`
	ImagesPath     string            `json:"images_path"`
	Statistics     map[string]string `json:"statistics"`
	Errors         []string          `json:"errors"`
	Warnings       []string          `json:"warnings"`
}

func NewAnalyzer(logger *logrus.Logger) *Analyzer {
	return &Analyzer{
		logger: logger,
	}
}

func (a *Analyzer) AnalyzeCheckpoint(checkpointDir string) (*CheckpointAnalysis, error) {
	a.logger.Infof("Analyzing checkpoint: %s", checkpointDir)

	analysis := &CheckpointAnalysis{}

	// 1. Load checkpoint metadata
	metadataPath := filepath.Join(checkpointDir, "checkpoint_metadata.json")
	if utils.FileExists(metadataPath) {
		metadata, err := a.loadCheckpointMetadata(metadataPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load checkpoint metadata: %w", err)
		}
		analysis.Metadata = metadata
	}

	// 2. Load mount mappings
	mountMappingsPath := filepath.Join(checkpointDir, "mount_mappings.json")
	if utils.FileExists(mountMappingsPath) {
		mappings, err := a.loadMountMappings(mountMappingsPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load mount mappings: %w", err)
		}
		analysis.MountMappings = mappings
	}

	// 3. Analyze CRIU images (simplified - in real implementation would parse protobuf)
	imagesDir := filepath.Join(checkpointDir, "images")
	if utils.DirExists(imagesDir) {
		criuInfo, err := a.analyzeCRIUImages(imagesDir)
		if err != nil {
			a.logger.Warnf("Failed to analyze CRIU images: %v", err)
		} else {
			analysis.CRIUInfo = criuInfo
		}
	}

	// 4. Build process tree (simplified)
	if analysis.Metadata != nil {
		processTree := a.buildProcessTree(analysis.Metadata.ContainerState)
		analysis.ProcessTree = processTree
	}

	// 5. Analyze resource usage
	resourceUsage := a.analyzeResourceUsage(checkpointDir, analysis.Metadata)
	analysis.ResourceUsage = resourceUsage

	return analysis, nil
}

func (a *Analyzer) GetProcessTree(checkpointDir string) (*ProcessInfo, error) {
	analysis, err := a.AnalyzeCheckpoint(checkpointDir)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze checkpoint: %w", err)
	}

	if analysis.ProcessTree == nil {
		return nil, fmt.Errorf("no process tree found in checkpoint")
	}

	return analysis.ProcessTree, nil
}

func (a *Analyzer) GetFileDescriptors(checkpointDir string) ([]FileDescriptor, error) {
	processTree, err := a.GetProcessTree(checkpointDir)
	if err != nil {
		return nil, err
	}

	var allFDs []FileDescriptor
	a.collectFileDescriptors(processTree, &allFDs)
	return allFDs, nil
}

func (a *Analyzer) GetSockets(checkpointDir string) ([]SocketInfo, error) {
	processTree, err := a.GetProcessTree(checkpointDir)
	if err != nil {
		return nil, err
	}

	var allSockets []SocketInfo
	a.collectSockets(processTree, &allSockets)
	return allSockets, nil
}

func (a *Analyzer) GetEnvironmentVariables(checkpointDir string) (map[string]string, error) {
	processTree, err := a.GetProcessTree(checkpointDir)
	if err != nil {
		return nil, err
	}

	// Return environment of the main process
	return processTree.Environment, nil
}

func (a *Analyzer) loadCheckpointMetadata(filePath string) (*checkpoint.CheckpointMetadata, error) {
	data, err := utils.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var metadata checkpoint.CheckpointMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, err
	}

	return &metadata, nil
}

func (a *Analyzer) loadMountMappings(filePath string) ([]docker.MountMapping, error) {
	data, err := utils.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var mappings []docker.MountMapping
	if err := json.Unmarshal(data, &mappings); err != nil {
		return nil, err
	}

	return mappings, nil
}

func (a *Analyzer) buildProcessTree(containerState *docker.ContainerState) *ProcessInfo {
	// Build a simplified process tree from container metadata
	// In a real implementation, this would parse CRIU's pstree.img

	envMap := containerState.Environment
	if envMap == nil {
		envMap = make(map[string]string)
	}

	// Extract command and args
	command := "unknown"
	args := []string{}

	if containerState.Config != nil {
		if len(containerState.Config.Cmd) > 0 {
			command = containerState.Config.Cmd[0]
			if len(containerState.Config.Cmd) > 1 {
				args = containerState.Config.Cmd[1:]
			}
		}

		if len(containerState.Config.Entrypoint) > 0 {
			command = containerState.Config.Entrypoint[0]
			args = append(containerState.Config.Entrypoint[1:], args...)
		}
	}

	process := &ProcessInfo{
		PID:             containerState.ProcessPID,
		PPID:            1, // Container init process
		Command:         command,
		Args:            args,
		Environment:     envMap,
		WorkingDir:      containerState.Config.WorkingDir,
		FileDescriptors: a.buildMockFileDescriptors(),
		Sockets:         a.buildMockSockets(),
		MemoryMaps:      a.buildMockMemoryMaps(),
		Children:        []ProcessInfo{}, // Would be populated from real analysis
		State:           "running",
		StartTime:       containerState.Created.Format("2006-01-02 15:04:05"),
	}

	return process
}

func (a *Analyzer) buildMockFileDescriptors() []FileDescriptor {
	// Mock file descriptors - in real implementation would parse fdinfo images
	return []FileDescriptor{
		{FD: 0, Type: "pipe", Path: "stdin", Mode: "r", Flags: "O_RDONLY"},
		{FD: 1, Type: "pipe", Path: "stdout", Mode: "w", Flags: "O_WRONLY"},
		{FD: 2, Type: "pipe", Path: "stderr", Mode: "w", Flags: "O_WRONLY"},
		{FD: 3, Type: "regular", Path: "/dev/null", Mode: "rw", Flags: "O_RDWR"},
	}
}

func (a *Analyzer) buildMockSockets() []SocketInfo {
	// Mock sockets - in real implementation would parse socket images
	return []SocketInfo{
		{
			FD:         4,
			Type:       "TCP",
			Family:     "AF_INET",
			State:      "LISTEN",
			LocalAddr:  "0.0.0.0",
			LocalPort:  80,
			RemoteAddr: "0.0.0.0",
			RemotePort: 0,
			SendBuffer: 65536,
			RecvBuffer: 65536,
			Protocol:   "tcp",
		},
	}
}

func (a *Analyzer) buildMockMemoryMaps() []MemoryMap {
	// Mock memory maps - in real implementation would parse memory images
	return []MemoryMap{
		{
			StartAddr:   "0x400000",
			EndAddr:     "0x401000",
			Permissions: "r-xp",
			Offset:      "0x0",
			Device:      "08:01",
			Inode:       "12345",
			Path:        "/bin/busybox",
			Size:        4096,
		},
	}
}

func (a *Analyzer) analyzeCRIUImages(imagesDir string) (*CRIUInfo, error) {
	files, err := utils.ListFiles(imagesDir)
	if err != nil {
		return nil, err
	}

	criuInfo := &CRIUInfo{
		Version:    "4.x.x", // Would get from actual CRIU
		Features:   []string{"tcp", "unix-sockets", "pid-ns", "net-ns", "mnt-ns"},
		ImagesPath: imagesDir,
		Statistics: make(map[string]string),
		Errors:     []string{},
		Warnings:   []string{},
	}

	// Count different types of image files
	imageTypes := make(map[string]int)
	for _, file := range files {
		ext := filepath.Ext(file.Name())
		if ext == ".img" {
			base := strings.TrimSuffix(file.Name(), ext)
			parts := strings.Split(base, "-")
			if len(parts) > 0 {
				imageTypes[parts[0]]++
			}
		}
	}

	// Convert counts to statistics
	for imageType, count := range imageTypes {
		criuInfo.Statistics[imageType] = fmt.Sprintf("%d files", count)
	}

	criuInfo.Statistics["total_files"] = fmt.Sprintf("%d files", len(files))

	return criuInfo, nil
}

func (a *Analyzer) analyzeResourceUsage(checkpointDir string, metadata *checkpoint.CheckpointMetadata) *ResourceUsage {
	usage := &ResourceUsage{
		Cgroups: make(map[string]string),
	}

	if metadata != nil && metadata.ContainerState != nil {
		state := metadata.ContainerState

		// Extract resource info from container config
		if state.HostConfig != nil && state.HostConfig.Resources.Memory > 0 {
			usage.MemoryUsage = state.HostConfig.Resources.Memory
		}

		usage.Processes = 1 // At least the main process
		usage.OpenFiles = len(a.buildMockFileDescriptors())

		// Mock cgroup info
		usage.Cgroups["memory"] = fmt.Sprintf("/docker/%s", state.ID[:12])
		usage.Cgroups["cpu"] = fmt.Sprintf("/docker/%s", state.ID[:12])
	}

	return usage
}

func (a *Analyzer) collectFileDescriptors(process *ProcessInfo, allFDs *[]FileDescriptor) {
	*allFDs = append(*allFDs, process.FileDescriptors...)
	for _, child := range process.Children {
		a.collectFileDescriptors(&child, allFDs)
	}
}

func (a *Analyzer) collectSockets(process *ProcessInfo, allSockets *[]SocketInfo) {
	*allSockets = append(*allSockets, process.Sockets...)
	for _, child := range process.Children {
		a.collectSockets(&child, allSockets)
	}
}