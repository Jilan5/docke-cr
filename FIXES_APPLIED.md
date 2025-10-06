# Docker-CR Fixes Applied

## Issues Identified and Fixed

### 1. **CRIU Working Directory Setup**
- **Problem**: Only setting `ImagesDirFd` without `WorkDirFd`
- **Fix**: Added proper working directory file descriptor setup in `criu.go`
- **File**: `pkg/checkpoint/criu.go:87-93`

### 2. **External Mount Format**
- **Problem**: Incorrect external mount format causing CRIU to fail with Docker containers
- **Fix**: Updated to simpler format with standard Docker mounts
- **File**: `pkg/checkpoint/criu.go:179-213`
- Changed from `mnt[path]:host_path` to `mnt[path]` format
- Added proper Docker-specific mounts like `/etc/hosts`, `/etc/hostname`, etc.

### 3. **CRIU Options Enhancement**
- **Problem**: Missing important CRIU options for Docker containers
- **Fix**: Added Docker-specific CRIU options:
  - `ExtUnixSk: true` for external unix sockets
  - `GhostLimit: 0` to handle ghost file limits
  - `ManageCgroupsMode: SOFT` for better cgroup handling
- **File**: `pkg/checkpoint/criu.go:74-88`

### 4. **Default Checkpoint Flags**
- **Problem**: Some flags defaulted to `true` which could cause issues
- **Fix**: Changed default values to `false` for:
  - `tcpEstablished`
  - `fileLocks`
  - `manageCgroups`
  - `shell`
- **File**: `cmd/main.go:148-155`

### 5. **Command-Line Fallback**
- **Problem**: go-criu library may fail in certain scenarios
- **Fix**: Added fallback to direct CRIU command execution
- **New File**: `pkg/checkpoint/criu_cmd.go`
- **Integration**: Modified `criu.go:122-133` to try command-line if library fails

### 6. **Log File Naming**
- **Problem**: Using `checkpoint.log` instead of standard `dump.log`
- **Fix**: Changed to `dump.log` to match working implementation
- **File**: `pkg/checkpoint/manager.go:97`

## How to Build and Test

```bash
# Build the application
cd docker-cr
go build -o docker-cr cmd/main.go

# Run checkpoint (ensure you have a running container)
sudo ./docker-cr checkpoint myapp

# If checkpoint still fails, check the logs
cat /tmp/docker-checkpoints/myapp/checkpoint1/dump.log
```

## Debugging Tips

1. **Check CRIU version**: Ensure CRIU 3.x or higher is installed
   ```bash
   criu --version
   ```

2. **Verify container is running**:
   ```bash
   docker ps | grep myapp
   ```

3. **Run with verbose logging**:
   ```bash
   sudo ./docker-cr checkpoint myapp --log-level debug -v
   ```

4. **Manual CRIU test**: The command-line fallback executes something like:
   ```bash
   sudo criu dump -t <PID> -D /tmp/docker-checkpoints/myapp/checkpoint1/images \
     --log-file dump.log -v4 --tcp-established --file-locks --leave-running \
     --external mnt[/proc/sys] --external mnt[/sys] ...
   ```

## Key Differences from Working Version

The working `docker-checkpoint-fixed` likely uses direct CRIU command execution throughout, while this version attempts to use the go-criu library first with a command-line fallback. The external mount handling has been simplified to match what works with Docker containers.