# Docker-CR: Simple Docker Checkpoint/Restore Tool

A lightweight, non-daemon Docker checkpoint and restore system built with Go-CRIU. Inspired by Prajwal S N's presentation on "Container checkpoints with Go and CRIU".

## Features

- **Simple Architecture**: No daemon required, direct CLI tool
- **Mount Namespace Handling**: Proper external mount mapping to fix restore errors
- **Go-CRIU Integration**: Uses the official Go-CRIU library v7
- **Inspection Tools**: Built-in checkpoint analysis (similar to checkpointctl)
- **Docker API Integration**: Seamless Docker container management
- **Forensic Analysis**: Process tree, file descriptors, sockets inspection

## Architecture

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│   CLI Tool   │────▶│  Go-CRIU     │────▶│   Docker     │
│  (docker-cr) │     │   Library    │     │   API        │
└──────────────┘     └──────────────┘     └──────────────┘
        │                    │                     │
        └────────────────────┼─────────────────────┘
                            ▼
                    ┌──────────────┐
                    │     CRIU     │
                    │   (Native)   │
                    └──────────────┘
```

## Installation

### Prerequisites

- **CRIU** >= 3.17 with Docker support
- **Docker** >= 20.10
- **Go** >= 1.21 (for building from source)
- **Root privileges** (required for CRIU operations)

#### Install CRIU (Ubuntu/Debian)

```bash
sudo apt update
sudo apt install criu
```

#### Install CRIU (CentOS/RHEL)

```bash
sudo yum install criu
# or for newer versions
sudo dnf install criu
```

### Build from Source

```bash
# Clone the repository
git clone <repository-url>
cd docker-cr

# Build the binary
make build

# Install to system (optional)
sudo make install
```

### Quick Setup

```bash
# Setup development environment
make dev-setup

# Check dependencies
make check-deps

# Run quick test
make quick-test
```

## Usage

### Basic Commands

```bash
# Checkpoint a running container
sudo docker-cr checkpoint <container-name> [options]

# Restore from checkpoint
sudo docker-cr restore --from <checkpoint-dir> --new-name <new-name>

# Inspect checkpoint
docker-cr inspect <checkpoint-dir> [options]

# Show version
docker-cr version
```

### Checkpoint Examples

```bash
# Basic checkpoint
sudo docker-cr checkpoint my-container

# Checkpoint with custom location and name
sudo docker-cr checkpoint my-container --output ./checkpoints --name backup-1

# Checkpoint without stopping container
sudo docker-cr checkpoint my-container --leave-running=true

# Checkpoint with TCP connections
sudo docker-cr checkpoint my-container --tcp=true --file-locks=true
```

### Restore Examples

```bash
# Basic restore
sudo docker-cr restore --from ./checkpoints/my-container/checkpoint --new-name my-container-restored

# Restore with mount validation disabled
sudo docker-cr restore --from ./checkpoints/my-container/checkpoint --new-name restored --validate-env=false

# Restore skipping problematic mounts
sudo docker-cr restore --from ./checkpoints/my-container/checkpoint --new-name restored --skip-mounts=/problematic/mount
```

### Inspection Examples

```bash
# Quick summary
docker-cr inspect ./checkpoints/my-container/checkpoint --summary

# Show process tree
docker-cr inspect ./checkpoints/my-container/checkpoint --ps-tree

# Show all information
docker-cr inspect ./checkpoints/my-container/checkpoint --all

# Export to JSON
docker-cr inspect ./checkpoints/my-container/checkpoint --format=json --all > checkpoint-data.json

# Show specific components
docker-cr inspect ./checkpoints/my-container/checkpoint --files --sockets --env
```

## Key Features Solving Mount Namespace Issues

### External Mount Mapping

The tool automatically handles mount namespace issues by:

1. **Pre-validating mount sources** before restore
2. **Creating external mount mappings** using CRIU's `--ext-mount-map`
3. **Auto-creating missing mount points** when `--auto-fix-mounts` is enabled
4. **Skipping problematic mounts** with `--skip-mounts` option

### Mount Namespace Preparation

```go
// Example of how mount namespace issues are resolved
func (m *Manager) prepareMountNamespace(containerID string, mappings []docker.MountMapping, autoFix bool) error {
    // 1. Validate all mount sources exist on host
    for _, mapping := range mappings {
        if mapping.IsExternal && mapping.HostPath != "" {
            if !utils.FileExists(mapping.HostPath) && !utils.DirExists(mapping.HostPath) {
                if autoFix {
                    // Create missing mount source
                    if err := utils.EnsureDir(mapping.HostPath); err != nil {
                        return fmt.Errorf("failed to create mount source %s: %w", mapping.HostPath, err)
                    }
                } else {
                    // Log warning about missing mount
                    m.logger.Warnf("Mount source does not exist: %s", mapping.HostPath)
                }
            }
        }
    }
    return nil
}
```

## Advanced Usage

### Complete Demo

```bash
# Run the complete demo
make demo
```

This will:
1. Create a test nginx container
2. Checkpoint the running container
3. Inspect the checkpoint
4. Stop the original container
5. Restore as a new container
6. Verify the restoration

### Integration Testing

```bash
# Run full integration tests
make run-tests

# Run unit tests only
make test-unit

# Run with coverage
make test-coverage
```

### Development

```bash
# Build with debug symbols
make build-debug

# Format and check code
make check

# Run benchmarks
make benchmark
```

## Configuration

### Default Checkpoint Options

- **Output Directory**: `/tmp/docker-checkpoints`
- **Leave Running**: `true` (container continues after checkpoint)
- **TCP Established**: `true` (checkpoint TCP connections)
- **File Locks**: `true` (checkpoint file locks)
- **Manage Cgroups**: `true` (handle cgroup settings)

### Default Restore Options

- **Validate Environment**: `true` (check restore compatibility)
- **Auto Fix Mounts**: `true` (create missing mount sources)
- **Manage Cgroups**: `false` (simplified cgroup handling)
- **TCP Established**: `false` (don't restore TCP connections by default)

## Troubleshooting

### Common Issues

#### Mount Namespace Error: "No mapping for X:(null) mountpoint"

**Solution**: Use auto-fix mounts or skip problematic mounts:

```bash
sudo docker-cr restore --from ./checkpoint --new-name restored --auto-fix-mounts=true
# or
sudo docker-cr restore --from ./checkpoint --new-name restored --skip-mounts=/problematic/path
```

#### Permission Denied

**Solution**: Ensure you're running with sudo for checkpoint/restore operations:

```bash
sudo docker-cr checkpoint my-container
sudo docker-cr restore --from ./checkpoint --new-name restored
```

#### CRIU Not Found

**Solution**: Install CRIU and ensure it's in PATH:

```bash
# Ubuntu/Debian
sudo apt install criu

# Verify installation
which criu
criu --version
```

#### Container Not Running

**Solution**: Ensure the container is in running state before checkpointing:

```bash
docker ps  # Check container status
docker start my-container  # Start if stopped
```

### Debug Mode

Enable verbose logging for troubleshooting:

```bash
sudo docker-cr checkpoint my-container --verbose --log-level=debug
```

## Project Structure

```
docker-cr/
├── cmd/
│   └── main.go              # CLI entry point
├── pkg/
│   ├── checkpoint/          # Checkpoint operations
│   │   ├── manager.go       # Main checkpoint logic
│   │   └── criu.go          # CRIU wrapper
│   ├── restore/             # Restore operations
│   │   └── manager.go       # Restore logic with mount fixes
│   ├── docker/              # Docker integration
│   │   └── manager.go       # Docker API wrapper
│   ├── inspect/             # Checkpoint inspection
│   │   ├── analyzer.go      # Analysis engine
│   │   └── viewer.go        # Output formatting
│   └── utils/               # Utilities
├── test/                    # Test files
├── Makefile                 # Build system
├── go.mod                   # Go modules
└── README.md               # This file
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Run tests: `make test`
5. Submit a pull request

## License

This project is open source. Please check the LICENSE file for details.

## Acknowledgments

- **Prajwal S N** for the excellent presentation on Container checkpoints with Go and CRIU
- **CRIU Project** for the amazing checkpoint/restore technology
- **Go-CRIU Library** for the Go integration
- **Docker** for the container platform

## References

- [CRIU Project](https://criu.org/)
- [Go-CRIU Library](https://github.com/checkpoint-restore/go-criu)
- [Container checkpoints with Go and CRIU - Prajwal S N](https://youtu.be/UGQgcIz9xGc)
- [Docker API Documentation](https://docs.docker.com/engine/api/)