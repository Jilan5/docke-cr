# Docker-CR Testing Guide

This directory contains comprehensive tests and testing documentation for the Docker-CR checkpoint/restore system.

## Test Structure

```
test/
├── README.md                    # This file - testing guide
├── integration_test.go          # Go integration tests
├── basic_functionality.md       # Basic functionality tests
├── mount_namespace_tests.md     # Mount namespace specific tests
├── edge_cases.md               # Edge case scenarios
├── performance_tests.md        # Performance and load tests
└── scripts/                    # Test automation scripts
    ├── test_basic.sh           # Basic functionality test
    ├── test_mount_issues.sh    # Mount namespace error tests
    ├── test_network.sh         # Network container tests
    └── test_volumes.sh         # Volume mount tests
```

## Prerequisites

Before running tests, ensure you have:

1. **CRIU installed and working**:
   ```bash
   criu --version
   sudo criu check
   ```

2. **Docker with experimental features**:
   ```bash
   docker version
   docker info | grep -i experimental
   ```

3. **Root privileges** for checkpoint/restore operations

4. **Docker-CR built and accessible**:
   ```bash
   make build
   # or if installed system-wide
   which docker-cr
   ```

## Quick Test

Run the basic functionality test:

```bash
cd test/scripts
./test_basic.sh
```

## Test Categories

### 1. Basic Functionality Tests
- Container checkpoint creation
- Container restoration
- Checkpoint validation
- Basic CLI operations

### 2. Mount Namespace Tests
- External mount mapping
- Missing mount source handling
- Mount validation and auto-fix
- Skip mount functionality

### 3. Network Tests
- Simple network containers
- Port mapping preservation
- Network namespace handling

### 4. Volume Tests
- Bind mount containers
- Named volume containers
- Complex mount scenarios

### 5. Edge Cases
- Large containers
- Multi-process containers
- Long-running containers
- Container state variations

### 6. Performance Tests
- Checkpoint time measurement
- Restore time measurement
- Resource usage monitoring
- Scalability tests

## Running Tests

### Individual Test Scripts

```bash
# Basic functionality
./scripts/test_basic.sh

# Mount namespace specific
./scripts/test_mount_issues.sh

# Network containers
./scripts/test_network.sh

# Volume containers
./scripts/test_volumes.sh
```

### Go Integration Tests

```bash
# Run all Go tests
make test

# Run integration tests only
go test ./test/ -v

# Run with verbose output
go test ./test/ -v -race

# Run specific test
go test ./test/ -run TestCheckpointRestore -v
```

### Makefile Targets

```bash
# Quick functionality test
make quick-test

# Full integration test suite
make run-tests

# Demo with complete workflow
make demo

# Check dependencies
make check-deps
```

## Test Environments

### Minimal Test Environment

For basic testing with minimal containers:

```bash
# Alpine containers (small, fast)
docker run -d --name test-alpine alpine:latest sleep 300

# BusyBox containers (minimal)
docker run -d --name test-busybox busybox:latest sleep 300
```

### Application Test Environment

For realistic application testing:

```bash
# Web server
docker run -d --name test-nginx -p 8080:80 nginx:alpine

# Database
docker run -d --name test-redis redis:alpine

# Application with volumes
docker run -d --name test-app -v /tmp/data:/data alpine:latest sleep 300
```

### Complex Test Environment

For advanced testing scenarios:

```bash
# Multi-container application
docker-compose up -d

# Containers with custom networks
docker network create test-network
docker run -d --name test-net --network test-network alpine:latest sleep 300
```

## Expected Results

### Successful Checkpoint

```
=== Docker-CR Basic Test ===
1. Creating test container: test-container
2. Waiting for container to be ready...
3. Checkpointing container...
✓ Checkpoint created successfully
4. Inspecting checkpoint...
✓ Checkpoint validation passed
5. Restoring container...
✓ Container restored successfully
6. Verifying restored container...
✓ Restored container is running
=== Test Complete ===
```

### Common Issues and Solutions

#### 1. Permission Errors
```bash
# Error: Permission denied
# Solution: Run with sudo
sudo docker-cr checkpoint container-name
```

#### 2. CRIU Not Found
```bash
# Error: CRIU binary not found
# Solution: Install CRIU
sudo apt install criu
```

#### 3. Mount Namespace Errors
```bash
# Error: mnt: No mapping for X:(null) mountpoint
# Solution: Use auto-fix mounts
sudo docker-cr restore --from checkpoint --auto-fix-mounts=true
```

#### 4. Container Not Running
```bash
# Error: container is not running
# Solution: Start container first
docker start container-name
```

## Test Data and Cleanup

### Test Data Location

```bash
# Default test checkpoint location
~/docker-cr-tests/checkpoints/

# Temporary test data
/tmp/docker-cr-test-*

# Build artifacts
./bin/
./dist/
```

### Cleanup Commands

```bash
# Remove test containers
docker stop $(docker ps -q --filter "name=docker-cr-test") 2>/dev/null || true
docker rm $(docker ps -aq --filter "name=docker-cr-test") 2>/dev/null || true

# Remove test checkpoints
rm -rf ~/docker-cr-tests/
rm -rf /tmp/docker-cr-test-*

# Clean build artifacts
make clean
```

## Automated Testing

### Continuous Integration

Create a CI pipeline using the test scripts:

```yaml
# .github/workflows/test.yml
name: Docker-CR Tests
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
      - name: Setup
        run: |
          sudo apt update
          sudo apt install -y criu docker.io
          ./deploy_docker_cr.sh
      - name: Test
        run: make run-tests
```

### Local Test Automation

```bash
# Run all tests in sequence
./scripts/run_all_tests.sh

# Run tests with specific tags
./scripts/run_tests.sh --tag=mount-namespace

# Run performance tests
./scripts/run_performance_tests.sh
```

## Debugging Failed Tests

### Enable Debug Logging

```bash
# Run with verbose logging
docker-cr checkpoint container-name --verbose --log-level=debug

# Check CRIU logs
cat /path/to/checkpoint/checkpoint.log
cat /path/to/checkpoint/restore.log
```

### Common Debug Commands

```bash
# Check container state
docker inspect container-name

# Check CRIU support
sudo criu check --all

# Check mount points
cat /proc/mounts | grep container-id

# Check process tree
pstree -p container-pid
```

## Contributing Tests

When adding new tests:

1. **Follow naming conventions**: `test_feature_name.sh` or `TestFeatureName` for Go tests
2. **Include cleanup**: Always clean up test containers and data
3. **Add documentation**: Update this README with new test descriptions
4. **Test both success and failure cases**
5. **Include performance benchmarks** where relevant

## Test Reports

Generate test reports:

```bash
# Generate coverage report
make test-coverage

# Generate benchmark report
make benchmark > test-results.txt

# Generate performance report
./scripts/generate_performance_report.sh
```

## Security Considerations

- **Run tests in isolated environments** when possible
- **Avoid sensitive data** in test containers
- **Clean up thoroughly** to prevent data leaks
- **Use minimal privileges** where feasible
- **Monitor resource usage** during tests

## Support

If tests fail or you encounter issues:

1. Check the [main README](../README.md) for troubleshooting
2. Review test logs and error messages
3. Ensure all prerequisites are met
4. Run dependency checks: `make check-deps`
5. Report issues with full logs and environment details