package inspect

import (
	"docker-cr/pkg/docker"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/sirupsen/logrus"
)

type Viewer struct {
	analyzer *Analyzer
	logger   *logrus.Logger
}

type ViewOptions struct {
	ShowProcessTree bool
	ShowEnvironment bool
	ShowFiles       bool
	ShowSockets     bool
	ShowMounts      bool
	ShowAll         bool
	OutputFormat    string // "text", "json", "tree"
	Verbose         bool
}

func NewViewer(logger *logrus.Logger) *Viewer {
	return &Viewer{
		analyzer: NewAnalyzer(logger),
		logger:   logger,
	}
}

func (v *Viewer) ShowCheckpoint(checkpointDir string, options ViewOptions) (string, error) {
	analysis, err := v.analyzer.AnalyzeCheckpoint(checkpointDir)
	if err != nil {
		return "", fmt.Errorf("failed to analyze checkpoint: %w", err)
	}

	switch options.OutputFormat {
	case "json":
		return v.formatJSON(analysis, options)
	case "tree":
		return v.formatTree(analysis, options)
	default:
		return v.formatText(analysis, options)
	}
}

func (v *Viewer) formatText(analysis *CheckpointAnalysis, options ViewOptions) (string, error) {
	var output strings.Builder

	// Show basic checkpoint info
	if analysis.Metadata != nil {
		output.WriteString("=== Checkpoint Information ===\n")
		state := analysis.Metadata.ContainerState
		output.WriteString(fmt.Sprintf("Container Name: %s\n", state.Name))
		output.WriteString(fmt.Sprintf("Container ID: %s\n", state.ID[:12]))
		output.WriteString(fmt.Sprintf("Image: %s\n", state.Image))
		output.WriteString(fmt.Sprintf("Created: %s\n", analysis.Metadata.CreatedAt))
		output.WriteString(fmt.Sprintf("Runtime: %s\n", state.Runtime))
		output.WriteString(fmt.Sprintf("Main PID: %d\n", state.ProcessPID))
		output.WriteString("\n")
	}

	// Show process tree
	if (options.ShowProcessTree || options.ShowAll) && analysis.ProcessTree != nil {
		output.WriteString("=== Process Tree ===\n")
		v.formatProcessTree(analysis.ProcessTree, "", &output, options.Verbose)
		output.WriteString("\n")
	}

	// Show environment variables
	if (options.ShowEnvironment || options.ShowAll) && analysis.ProcessTree != nil {
		output.WriteString("=== Environment Variables ===\n")
		env := analysis.ProcessTree.Environment
		var keys []string
		for k := range env {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, key := range keys {
			output.WriteString(fmt.Sprintf("%s=%s\n", key, env[key]))
		}
		output.WriteString("\n")
	}

	// Show file descriptors
	if (options.ShowFiles || options.ShowAll) && analysis.ProcessTree != nil {
		output.WriteString("=== File Descriptors ===\n")
		fds, err := v.analyzer.GetFileDescriptors("")
		if err == nil {
			for _, fd := range fds {
				output.WriteString(fmt.Sprintf("FD %d: %s %s (%s)\n",
					fd.FD, fd.Type, fd.Path, fd.Mode))
			}
		}
		output.WriteString("\n")
	}

	// Show sockets
	if (options.ShowSockets || options.ShowAll) && analysis.ProcessTree != nil {
		output.WriteString("=== Sockets ===\n")
		sockets, err := v.analyzer.GetSockets("")
		if err == nil {
			for _, socket := range sockets {
				if socket.Type == "TCP" || socket.Type == "UDP" {
					output.WriteString(fmt.Sprintf("FD %d: %s %s %s:%d -> %s:%d (%s)\n",
						socket.FD, socket.Type, socket.State,
						socket.LocalAddr, socket.LocalPort,
						socket.RemoteAddr, socket.RemotePort,
						socket.Family))
				} else {
					output.WriteString(fmt.Sprintf("FD %d: %s %s (%s)\n",
						socket.FD, socket.Type, socket.State, socket.Family))
				}
			}
		}
		output.WriteString("\n")
	}

	// Show mount mappings
	if (options.ShowMounts || options.ShowAll) && len(analysis.MountMappings) > 0 {
		output.WriteString("=== Mount Mappings ===\n")
		for _, mount := range analysis.MountMappings {
			external := ""
			if mount.IsExternal {
				external = " (external)"
			}
			output.WriteString(fmt.Sprintf("%s -> %s (%s)%s\n",
				mount.ContainerPath, mount.HostPath, mount.Type, external))
		}
		output.WriteString("\n")
	}

	// Show CRIU info
	if options.Verbose && analysis.CRIUInfo != nil {
		output.WriteString("=== CRIU Information ===\n")
		output.WriteString(fmt.Sprintf("Version: %s\n", analysis.CRIUInfo.Version))
		output.WriteString(fmt.Sprintf("Images Path: %s\n", analysis.CRIUInfo.ImagesPath))
		output.WriteString("Features: " + strings.Join(analysis.CRIUInfo.Features, ", ") + "\n")

		if len(analysis.CRIUInfo.Statistics) > 0 {
			output.WriteString("Statistics:\n")
			for key, value := range analysis.CRIUInfo.Statistics {
				output.WriteString(fmt.Sprintf("  %s: %s\n", key, value))
			}
		}
		output.WriteString("\n")
	}

	// Show resource usage
	if options.Verbose && analysis.ResourceUsage != nil {
		output.WriteString("=== Resource Usage ===\n")
		usage := analysis.ResourceUsage
		if usage.MemoryUsage > 0 {
			output.WriteString(fmt.Sprintf("Memory: %d bytes\n", usage.MemoryUsage))
		}
		output.WriteString(fmt.Sprintf("Processes: %d\n", usage.Processes))
		output.WriteString(fmt.Sprintf("Open Files: %d\n", usage.OpenFiles))

		if len(usage.Cgroups) > 0 {
			output.WriteString("Cgroups:\n")
			for controller, path := range usage.Cgroups {
				output.WriteString(fmt.Sprintf("  %s: %s\n", controller, path))
			}
		}
		output.WriteString("\n")
	}

	return output.String(), nil
}

func (v *Viewer) formatJSON(analysis *CheckpointAnalysis, options ViewOptions) (string, error) {
	data, err := json.MarshalIndent(analysis, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal analysis to JSON: %w", err)
	}
	return string(data), nil
}

func (v *Viewer) formatTree(analysis *CheckpointAnalysis, options ViewOptions) (string, error) {
	var output strings.Builder

	if analysis.ProcessTree != nil {
		output.WriteString("Process Tree:\n")
		v.formatProcessTree(analysis.ProcessTree, "", &output, true)
	}

	return output.String(), nil
}

func (v *Viewer) formatProcessTree(process *ProcessInfo, prefix string, output *strings.Builder, verbose bool) {
	// Format process info
	output.WriteString(fmt.Sprintf("%s├─ PID %d: %s", prefix, process.PID, process.Command))

	if len(process.Args) > 0 && verbose {
		output.WriteString(" " + strings.Join(process.Args, " "))
	}
	output.WriteString("\n")

	if verbose {
		// Show additional process details
		if process.WorkingDir != "" {
			output.WriteString(fmt.Sprintf("%s│  Working Dir: %s\n", prefix, process.WorkingDir))
		}

		if len(process.FileDescriptors) > 0 {
			output.WriteString(fmt.Sprintf("%s│  File Descriptors: %d\n", prefix, len(process.FileDescriptors)))
		}

		if len(process.Sockets) > 0 {
			output.WriteString(fmt.Sprintf("%s│  Sockets: %d\n", prefix, len(process.Sockets)))
		}

		if len(process.Environment) > 0 {
			output.WriteString(fmt.Sprintf("%s│  Environment Variables: %d\n", prefix, len(process.Environment)))
		}
	}

	// Format children
	for i, child := range process.Children {
		childPrefix := prefix + "│  "
		if i == len(process.Children)-1 {
			childPrefix = prefix + "   "
		}
		v.formatProcessTree(&child, childPrefix, output, verbose)
	}
}

func (v *Viewer) ShowMountMappings(mappings []docker.MountMapping, format string) (string, error) {
	if format == "json" {
		data, err := json.MarshalIndent(mappings, "", "  ")
		if err != nil {
			return "", err
		}
		return string(data), nil
	}

	var output strings.Builder
	output.WriteString("Mount Mappings:\n")

	for _, mapping := range mappings {
		external := ""
		if mapping.IsExternal {
			external = " [EXTERNAL]"
		}

		readOnly := ""
		if mapping.ReadOnly {
			readOnly = " [RO]"
		}

		output.WriteString(fmt.Sprintf("  %s -> %s (%s)%s%s\n",
			mapping.ContainerPath, mapping.HostPath, mapping.Type, external, readOnly))
	}

	return output.String(), nil
}

func (v *Viewer) ShowFileDescriptors(fds []FileDescriptor, format string) (string, error) {
	if format == "json" {
		data, err := json.MarshalIndent(fds, "", "  ")
		if err != nil {
			return "", err
		}
		return string(data), nil
	}

	var output strings.Builder
	output.WriteString("File Descriptors:\n")

	for _, fd := range fds {
		special := ""
		if fd.IsPipe {
			special = " [PIPE]"
		} else if fd.IsSocket {
			special = " [SOCKET]"
		}

		output.WriteString(fmt.Sprintf("  FD %d: %s %s (%s %s)%s\n",
			fd.FD, fd.Type, fd.Path, fd.Mode, fd.Flags, special))
	}

	return output.String(), nil
}

func (v *Viewer) ShowSockets(sockets []SocketInfo, format string) (string, error) {
	if format == "json" {
		data, err := json.MarshalIndent(sockets, "", "  ")
		if err != nil {
			return "", err
		}
		return string(data), nil
	}

	var output strings.Builder
	output.WriteString("Socket Information:\n")

	for _, socket := range sockets {
		output.WriteString(fmt.Sprintf("  FD %d: %s %s (%s)\n",
			socket.FD, socket.Type, socket.Family, socket.State))

		if socket.Type == "TCP" || socket.Type == "UDP" {
			output.WriteString(fmt.Sprintf("    Local:  %s:%d\n", socket.LocalAddr, socket.LocalPort))
			output.WriteString(fmt.Sprintf("    Remote: %s:%d\n", socket.RemoteAddr, socket.RemotePort))
			output.WriteString(fmt.Sprintf("    Buffers: Send=%d, Recv=%d\n", socket.SendBuffer, socket.RecvBuffer))
		}
		output.WriteString("\n")
	}

	return output.String(), nil
}

func (v *Viewer) ShowEnvironment(env map[string]string, format string) (string, error) {
	if format == "json" {
		data, err := json.MarshalIndent(env, "", "  ")
		if err != nil {
			return "", err
		}
		return string(data), nil
	}

	var output strings.Builder
	output.WriteString("Environment Variables:\n")

	var keys []string
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		output.WriteString(fmt.Sprintf("  %s=%s\n", key, env[key]))
	}

	return output.String(), nil
}

func (v *Viewer) GetSummary(checkpointDir string) (string, error) {
	analysis, err := v.analyzer.AnalyzeCheckpoint(checkpointDir)
	if err != nil {
		return "", err
	}

	var output strings.Builder

	if analysis.Metadata != nil {
		state := analysis.Metadata.ContainerState
		output.WriteString(fmt.Sprintf("Checkpoint: %s (%s)\n", state.Name, state.ID[:12]))
		output.WriteString(fmt.Sprintf("Image: %s\n", state.Image))
		output.WriteString(fmt.Sprintf("Created: %s\n", analysis.Metadata.CreatedAt))
	}

	if analysis.ProcessTree != nil {
		output.WriteString(fmt.Sprintf("Main Process: PID %d (%s)\n",
			analysis.ProcessTree.PID, analysis.ProcessTree.Command))
		output.WriteString(fmt.Sprintf("Environment Variables: %d\n",
			len(analysis.ProcessTree.Environment)))
		output.WriteString(fmt.Sprintf("File Descriptors: %d\n",
			len(analysis.ProcessTree.FileDescriptors)))
		output.WriteString(fmt.Sprintf("Sockets: %d\n",
			len(analysis.ProcessTree.Sockets)))
	}

	if len(analysis.MountMappings) > 0 {
		output.WriteString(fmt.Sprintf("Mount Mappings: %d\n", len(analysis.MountMappings)))
	}

	if analysis.CRIUInfo != nil {
		output.WriteString(fmt.Sprintf("CRIU Version: %s\n", analysis.CRIUInfo.Version))
		if totalFiles, exists := analysis.CRIUInfo.Statistics["total_files"]; exists {
			output.WriteString(fmt.Sprintf("Checkpoint Files: %s\n", totalFiles))
		}
	}

	return output.String(), nil
}