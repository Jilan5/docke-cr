package checkpoint

import (
	"fmt"
	"os/exec"
	"strings"
)

// CheckpointProcessCmd performs checkpoint using CRIU command-line tool
// This is an alternative to the go-criu library approach
func (cm *CRIUManager) CheckpointProcessCmd(pid int, opts CheckpointOptions) error {
	cm.logger.Infof("Starting CRIU checkpoint via command for PID %d", pid)

	// Build CRIU command arguments
	args := []string{
		"dump",
		"-t", fmt.Sprintf("%d", pid),
		"-D", opts.ImagesDir,
		"--log-file", opts.LogFile,
		"-v4",  // Verbose logging
		"--tcp-established",
		"--file-locks",
		"--link-remap",
		"--force-irmap",
		"--manage-cgroups",
		"--enable-external-sharing",
		"--enable-external-masters",
		"--enable-fs", "hugetlbfs",
		"--enable-fs", "tracefs",
		"--skip-mnt", "/proc",
		"--skip-mnt", "/dev",
		"--skip-mnt", "/sys",
		"--skip-mnt", "/run",
	}

	// Add leave-running if specified
	if opts.LeaveRunning {
		args = append(args, "--leave-running")
	}

	// Add external mounts
	for _, ext := range opts.External {
		args = append(args, "--external", ext)
	}

	// Execute CRIU command
	cmd := exec.Command("criu", args...)
	cmd.Dir = opts.WorkDir

	cm.logger.Debugf("Executing: criu %s", strings.Join(args, " "))

	output, err := cmd.CombinedOutput()
	if err != nil {
		cm.logger.Errorf("CRIU command failed: %s", string(output))
		return fmt.Errorf("CRIU checkpoint failed: %w\nOutput: %s", err, string(output))
	}

	cm.logger.Info("CRIU checkpoint completed successfully")
	return nil
}

// RestoreProcessCmd performs restore using CRIU command-line tool
func (cm *CRIUManager) RestoreProcessCmd(opts RestoreOptions) error {
	cm.logger.Info("Starting CRIU restore via command")

	// Build CRIU restore command arguments
	args := []string{
		"restore",
		"-D", opts.ImagesDir,
		"--log-file", opts.LogFile,
		"-v4",
		"--tcp-established",
		"--file-locks",
		"--link-remap",
		"--manage-cgroups",
		"--enable-external-sharing",
		"--enable-external-masters",
		"--enable-fs", "hugetlbfs",
		"--enable-fs", "tracefs",
	}

	// Add restore-sibling if specified
	if opts.RestoreSibling {
		args = append(args, "--restore-sibling")
	}

	// Add shell-job if specified
	if opts.Shell {
		args = append(args, "--shell-job")
	}

	// Add PID file if specified
	if opts.PidFile != "" {
		args = append(args, "--pidfile", opts.PidFile)
	}

	// Add external mounts and mappings
	for _, ext := range opts.External {
		args = append(args, "--external", ext)
	}

	for _, mapping := range opts.ExtMountMap {
		args = append(args, "--ext-mount-map", mapping)
	}

	// Execute CRIU command
	cmd := exec.Command("criu", args...)
	cmd.Dir = opts.WorkDir

	cm.logger.Debugf("Executing: criu %s", strings.Join(args, " "))

	output, err := cmd.CombinedOutput()
	if err != nil {
		cm.logger.Errorf("CRIU restore command failed: %s", string(output))
		return fmt.Errorf("CRIU restore failed: %w\nOutput: %s", err, string(output))
	}

	cm.logger.Info("CRIU restore completed successfully")
	return nil
}

// TryCommandLineFallback attempts to use command-line CRIU if go-criu fails
func (cm *CRIUManager) TryCommandLineFallback(pid int, opts CheckpointOptions) error {
	cm.logger.Warn("Falling back to command-line CRIU execution")
	return cm.CheckpointProcessCmd(pid, opts)
}

// BuildCRIUCommandArgs builds CRIU command arguments for debugging
func BuildCRIUCommandArgs(pid int, opts CheckpointOptions) []string {
	args := []string{
		"criu", "dump",
		"-t", fmt.Sprintf("%d", pid),
		"-D", opts.ImagesDir,
		"--log-file", opts.LogFile,
		"-v4",
	}

	if opts.LeaveRunning {
		args = append(args, "--leave-running")
	}
	if opts.TcpEstablished {
		args = append(args, "--tcp-established")
	}
	if opts.FileLocks {
		args = append(args, "--file-locks")
	}
	if opts.ManageCgroups {
		args = append(args, "--manage-cgroups")
	}
	if opts.Shell {
		args = append(args, "--shell-job")
	}

	for _, ext := range opts.External {
		args = append(args, "--external", ext)
	}

	return args
}