package checkpoint

import (
	"docker-cr/pkg/docker"
	"docker-cr/pkg/utils"
	"fmt"
	"os"
	"strings"

	criu "github.com/checkpoint-restore/go-criu/v7"
	"github.com/checkpoint-restore/go-criu/v7/rpc"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/proto"
)

type CRIUManager struct {
	criuClient *criu.Criu
	logger     *logrus.Logger
}

type CheckpointOptions struct {
	WorkDir         string   `json:"work_dir"`
	ImagesDir       string   `json:"images_dir"`
	LogFile         string   `json:"log_file"`
	LogLevel        int      `json:"log_level"`
	External        []string `json:"external"`
	ManageCgroups   bool     `json:"manage_cgroups"`
	TcpEstablished  bool     `json:"tcp_established"`
	FileLocks       bool     `json:"file_locks"`
	LeaveRunning    bool     `json:"leave_running"`
	Shell           bool     `json:"shell"`
	PreDump         bool     `json:"pre_dump"`
	TrackMem        bool     `json:"track_mem"`
}

type RestoreOptions struct {
	WorkDir        string   `json:"work_dir"`
	ImagesDir      string   `json:"images_dir"`
	LogFile        string   `json:"log_file"`
	LogLevel       int      `json:"log_level"`
	External       []string `json:"external"`
	ExtMountMap    []string `json:"ext_mount_map"`
	SkipMnt        []string `json:"skip_mnt"`
	PidFile        string   `json:"pid_file"`
	ManageCgroups  bool     `json:"manage_cgroups"`
	TcpEstablished bool     `json:"tcp_established"`
	RestoreSibling bool     `json:"restore_sibling"`
	Shell          bool     `json:"shell"`
	EmptyNs        uint32   `json:"empty_ns"`
}

func NewCRIUManager(logger *logrus.Logger) *CRIUManager {
	criuClient := criu.MakeCriu()
	criuClient.SetCriuPath("criu")

	return &CRIUManager{
		criuClient: criuClient,
		logger:     logger,
	}
}

func (cm *CRIUManager) CheckpointProcess(pid int, opts CheckpointOptions) error {
	cm.logger.Infof("Starting CRIU checkpoint for PID %d", pid)

	// Ensure directories exist
	if err := utils.EnsureDir(opts.WorkDir); err != nil {
		return fmt.Errorf("failed to create work directory: %w", err)
	}

	if err := utils.EnsureDir(opts.ImagesDir); err != nil {
		return fmt.Errorf("failed to create images directory: %w", err)
	}

	// Build CRIU options with proper Docker-specific settings
	criuOpts := &rpc.CriuOpts{
		Pid:                proto.Int32(int32(pid)),
		LogLevel:           proto.Int32(int32(opts.LogLevel)),
		LogFile:            proto.String(opts.LogFile),
		ManageCgroups:      proto.Bool(opts.ManageCgroups),
		TcpEstablished:     proto.Bool(opts.TcpEstablished),
		FileLocks:          proto.Bool(opts.FileLocks),
		LeaveRunning:       proto.Bool(opts.LeaveRunning),
		ShellJob:           proto.Bool(opts.Shell),
		External:           opts.External,
		ExtUnixSk:          proto.Bool(true),
		GhostLimit:         proto.Uint32(0),
		ManageCgroupsMode:  rpc.CriuCgMode_SOFT.Enum(),
	}

	// Set working directory
	workDir, err := os.Open(opts.WorkDir)
	if err != nil {
		return fmt.Errorf("failed to open work directory: %w", err)
	}
	defer workDir.Close()

	criuOpts.WorkDirFd = proto.Int32(int32(workDir.Fd()))

	// Set images directory
	imagesDir, err := os.Open(opts.ImagesDir)
	if err != nil {
		return fmt.Errorf("failed to open images directory: %w", err)
	}
	defer imagesDir.Close()

	criuOpts.ImagesDirFd = proto.Int32(int32(imagesDir.Fd()))

	// Pre-dump if requested
	if opts.PreDump {
		cm.logger.Info("Performing pre-dump...")
		preDumpOpts := *criuOpts
		preDumpOpts.TrackMem = proto.Bool(opts.TrackMem)
		preDumpOpts.TcpEstablished = proto.Bool(false)

		if err := cm.criuClient.PreDump(&preDumpOpts, nil); err != nil {
			return fmt.Errorf("pre-dump failed: %w", err)
		}
	}

	// Perform checkpoint
	cm.logger.Info("Performing checkpoint...")
	if err := cm.criuClient.Dump(criuOpts, nil); err != nil {
		// Try to read and log CRIU error details
		cm.logCRIUError(opts.LogFile)

		// Try command-line fallback
		cm.logger.Warnf("go-criu library failed, trying command-line fallback: %v", err)
		if cmdErr := cm.CheckpointProcessCmd(pid, opts); cmdErr != nil {
			return fmt.Errorf("both go-criu and command-line CRIU failed.\nLibrary error: %w\nCommand error: %v", err, cmdErr)
		}

		cm.logger.Info("CRIU checkpoint completed successfully via command-line")
		return nil
	}

	cm.logger.Info("CRIU checkpoint completed successfully")
	return nil
}

func (cm *CRIUManager) RestoreProcess(opts RestoreOptions) error {
	cm.logger.Info("Starting CRIU restore")

	// Ensure directories exist
	if err := utils.EnsureDir(opts.WorkDir); err != nil {
		return fmt.Errorf("failed to create work directory: %w", err)
	}

	if !utils.DirExists(opts.ImagesDir) {
		return fmt.Errorf("images directory does not exist: %s", opts.ImagesDir)
	}

	// Build CRIU restore options
	criuOpts := &rpc.CriuOpts{
		LogLevel:       proto.Int32(int32(opts.LogLevel)),
		LogFile:        proto.String(opts.LogFile),
		ManageCgroups:  proto.Bool(opts.ManageCgroups),
		TcpEstablished: proto.Bool(opts.TcpEstablished),
		RstSibling:     proto.Bool(opts.RestoreSibling),
		ShellJob:       proto.Bool(opts.Shell),
		External:       opts.External,
		EmptyNs:        proto.Uint32(opts.EmptyNs),
	}

	// Set images directory
	workDir, err := os.Open(opts.ImagesDir)
	if err != nil {
		return fmt.Errorf("failed to open images directory: %w", err)
	}
	defer workDir.Close()

	criuOpts.ImagesDirFd = proto.Int32(int32(workDir.Fd()))

	// Add external mount mappings if provided
	if len(opts.ExtMountMap) > 0 {
		cm.logger.Infof("Using external mount mappings: %v", opts.ExtMountMap)
		criuOpts.External = append(criuOpts.External, opts.ExtMountMap...)
	}

	// Perform restore
	cm.logger.Info("Performing restore...")
	if err := cm.criuClient.Restore(criuOpts, nil); err != nil {
		// Try to read and log CRIU error details
		cm.logCRIUError(opts.LogFile)
		return fmt.Errorf("CRIU restore failed: %w", err)
	}

	cm.logger.Info("CRIU restore completed successfully")
	return nil
}

func (cm *CRIUManager) BuildExternalMountMappings(mappings []docker.MountMapping) []string {
	var external []string

	// Use simpler format that works better with Docker containers
	// Format: "mnt[path]:key"
	standardMounts := []string{
		"mnt[/proc/sys]",
		"mnt[/proc/sysrq-trigger]",
		"mnt[/proc/irq]",
		"mnt[/proc/bus]",
		"mnt[/sys/fs/cgroup]",
		"mnt[/sys]",
		"mnt[/dev]",
		"mnt[.dockerenv]",
		"mnt[/etc/hosts]",
		"mnt[/etc/hostname]",
		"mnt[/etc/resolv.conf]",
	}

	external = standardMounts

	// Add user-defined volume mounts
	for _, mapping := range mappings {
		if mapping.IsExternal && mapping.HostPath != "" &&
		   !strings.HasPrefix(mapping.ContainerPath, "/proc") &&
		   !strings.HasPrefix(mapping.ContainerPath, "/sys") &&
		   !strings.HasPrefix(mapping.ContainerPath, "/dev") {
			// Add user volumes
			extMount := fmt.Sprintf("mnt[%s]", mapping.ContainerPath)
			external = append(external, extMount)
		}
	}

	return external
}

func (cm *CRIUManager) BuildExtMountMapArgs(mappings []docker.MountMapping) []string {
	var args []string

	for _, mapping := range mappings {
		if mapping.IsExternal && mapping.HostPath != "" {
			// CRIU ext-mount-map format: "auto:container_path:host_path"
			arg := fmt.Sprintf("auto:%s:%s", mapping.ContainerPath, mapping.HostPath)
			args = append(args, arg)
		}
	}

	return args
}

func (cm *CRIUManager) ValidateMountSources(mappings []docker.MountMapping) error {
	for _, mapping := range mappings {
		if mapping.IsExternal && mapping.HostPath != "" {
			if !utils.FileExists(mapping.HostPath) && !utils.DirExists(mapping.HostPath) {
				cm.logger.Warnf("Mount source does not exist, will create placeholder: %s", mapping.HostPath)

				// Create placeholder directory
				if err := utils.EnsureDir(mapping.HostPath); err != nil {
					return fmt.Errorf("failed to create mount source placeholder %s: %w", mapping.HostPath, err)
				}
			}
		}
	}

	return nil
}

func (cm *CRIUManager) CreateExtMountMapFile(mappings []docker.MountMapping, filePath string) error {
	content := "# External mount map for Docker container restore\n"
	content += "# Format: container_path:host_path\n"

	for _, mapping := range mappings {
		if mapping.IsExternal && mapping.HostPath != "" {
			content += fmt.Sprintf("%s:%s\n", mapping.ContainerPath, mapping.HostPath)
		}
	}

	// Add standard mappings
	standardMappings := map[string]string{
		"/proc":          "/proc",
		"/sys":           "/sys",
		"/dev":           "/dev",
		"/dev/shm":       "/dev/shm",
		"/dev/pts":       "/dev/pts",
		"/dev/mqueue":    "/dev/mqueue",
		"/sys/fs/cgroup": "/sys/fs/cgroup",
	}

	for containerPath, hostPath := range standardMappings {
		content += fmt.Sprintf("%s:%s\n", containerPath, hostPath)
	}

	return utils.WriteFile(filePath, []byte(content))
}

func (cm *CRIUManager) logCRIUError(logFile string) {
	if logFile == "" {
		return
	}

	if utils.FileExists(logFile) {
		if logData, err := utils.ReadFile(logFile); err == nil {
			cm.logger.Errorf("CRIU error log:\n%s", string(logData))
		}
	}
}

func (cm *CRIUManager) GetCRIUVersion() (string, error) {
	// This would require implementing version check via CRIU RPC
	// For now, return a placeholder
	return "4.x.x", nil
}

func (cm *CRIUManager) CheckCRIUSupport() error {
	// Basic check to see if CRIU is available
	// This is a simplified check - in real implementation,
	// you'd want to call CRIU's check functionality

	if _, err := os.Stat("/usr/bin/criu"); err != nil {
		if _, err := os.Stat("/usr/local/bin/criu"); err != nil {
			return fmt.Errorf("CRIU binary not found in standard locations")
		}
	}

	return nil
}