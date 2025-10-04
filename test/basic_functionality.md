# Basic Functionality Tests

This document outlines basic functionality tests for the Docker-CR checkpoint/restore system.

## Test Suite: Basic Operations

### Test 1: Simple Container Checkpoint

**Objective**: Verify basic checkpoint functionality with a simple container.

**Prerequisites**:
- Docker running
- CRIU installed
- docker-cr built

**Steps**:
```bash
# 1. Create a simple test container
docker run -d --name basic-test alpine:latest sleep 300

# 2. Verify container is running
docker ps | grep basic-test

# 3. Create checkpoint
sudo docker-cr checkpoint basic-test --output ./test-checkpoints --name simple-checkpoint

# 4. Verify checkpoint files were created
ls -la ./test-checkpoints/basic-test/simple-checkpoint/

# 5. Check checkpoint validation
docker-cr inspect ./test-checkpoints/basic-test/simple-checkpoint --summary
```

**Expected Results**:
- Container starts successfully
- Checkpoint directory is created
- Checkpoint contains required files:
  - `container_metadata.json`
  - `mount_mappings.json`
  - `checkpoint_metadata.json`
  - `images/` directory with CRIU files
- Inspection shows valid checkpoint summary

**Cleanup**:
```bash
docker stop basic-test && docker rm basic-test
rm -rf ./test-checkpoints/
```

---

### Test 2: Container Restore

**Objective**: Verify basic restore functionality.

**Prerequisites**:
- Completed Test 1 (checkpoint exists)

**Steps**:
```bash
# 1. Stop and remove original container
docker stop basic-test && docker rm basic-test

# 2. Verify container is gone
docker ps -a | grep basic-test || echo "Container removed"

# 3. Restore from checkpoint
sudo docker-cr restore --from ./test-checkpoints/basic-test/simple-checkpoint --new-name basic-restored

# 4. Verify restored container
docker ps | grep basic-restored

# 5. Check container logs and status
docker logs basic-restored
docker inspect basic-restored
```

**Expected Results**:
- Original container is successfully removed
- Restore completes without errors
- New container `basic-restored` is running
- Container has same configuration as original
- Process state is restored

**Cleanup**:
```bash
docker stop basic-restored && docker rm basic-restored
rm -rf ./test-checkpoints/
```

---

### Test 3: CLI Help and Version

**Objective**: Verify CLI interface functionality.

**Steps**:
```bash
# 1. Test version command
docker-cr version

# 2. Test help command
docker-cr --help

# 3. Test checkpoint help
docker-cr checkpoint --help

# 4. Test restore help
docker-cr restore --help

# 5. Test inspect help
docker-cr inspect --help
```

**Expected Results**:
- Version information displays correctly
- Help text is clear and informative
- All subcommands have proper help documentation
- Command examples are provided

---

### Test 4: Checkpoint Inspection

**Objective**: Verify checkpoint analysis capabilities.

**Prerequisites**:
- A valid checkpoint from Test 1

**Steps**:
```bash
# 1. Basic inspection
docker-cr inspect ./test-checkpoints/basic-test/simple-checkpoint --summary

# 2. Detailed inspection
docker-cr inspect ./test-checkpoints/basic-test/simple-checkpoint --all

# 3. Specific component inspection
docker-cr inspect ./test-checkpoints/basic-test/simple-checkpoint --ps-tree
docker-cr inspect ./test-checkpoints/basic-test/simple-checkpoint --env
docker-cr inspect ./test-checkpoints/basic-test/simple-checkpoint --files
docker-cr inspect ./test-checkpoints/basic-test/simple-checkpoint --mounts

# 4. JSON output
docker-cr inspect ./test-checkpoints/basic-test/simple-checkpoint --format=json --all
```

**Expected Results**:
- Summary shows container name, ID, image, creation time
- Process tree displays main process information
- Environment variables are listed
- File descriptors are shown
- Mount mappings are displayed
- JSON output is valid and complete

---

### Test 5: Leave Container Running

**Objective**: Test checkpoint with container remaining active.

**Steps**:
```bash
# 1. Create container
docker run -d --name running-test alpine:latest sleep 600

# 2. Checkpoint with leave-running=true
sudo docker-cr checkpoint running-test --output ./test-checkpoints --leave-running=true

# 3. Verify original container still running
docker ps | grep running-test

# 4. Restore to new container
sudo docker-cr restore --from ./test-checkpoints/running-test/checkpoint --new-name running-restored

# 5. Verify both containers running
docker ps | grep running
```

**Expected Results**:
- Original container continues running after checkpoint
- Checkpoint is created successfully
- Both original and restored containers are running
- Both containers have similar process states

**Cleanup**:
```bash
docker stop running-test running-restored
docker rm running-test running-restored
rm -rf ./test-checkpoints/
```

---

### Test 6: Multiple Checkpoints

**Objective**: Test creating multiple checkpoints of the same container.

**Steps**:
```bash
# 1. Create container
docker run -d --name multi-test alpine:latest sleep 600

# 2. Create first checkpoint
sudo docker-cr checkpoint multi-test --output ./test-checkpoints --name checkpoint-1

# 3. Wait and create second checkpoint
sleep 5
sudo docker-cr checkpoint multi-test --output ./test-checkpoints --name checkpoint-2

# 4. List checkpoints
ls -la ./test-checkpoints/multi-test/

# 5. Inspect both checkpoints
docker-cr inspect ./test-checkpoints/multi-test/checkpoint-1 --summary
docker-cr inspect ./test-checkpoints/multi-test/checkpoint-2 --summary

# 6. Restore from first checkpoint
docker stop multi-test && docker rm multi-test
sudo docker-cr restore --from ./test-checkpoints/multi-test/checkpoint-1 --new-name multi-restored-1

# 7. Restore from second checkpoint
sudo docker-cr restore --from ./test-checkpoints/multi-test/checkpoint-2 --new-name multi-restored-2
```

**Expected Results**:
- Multiple checkpoint directories are created
- Each checkpoint has different timestamps
- Both checkpoints can be restored independently
- Restored containers work correctly

**Cleanup**:
```bash
docker stop multi-restored-1 multi-restored-2 2>/dev/null || true
docker rm multi-restored-1 multi-restored-2 2>/dev/null || true
rm -rf ./test-checkpoints/
```

---

### Test 7: Error Handling

**Objective**: Test error handling for common failure scenarios.

**Steps**:
```bash
# 1. Test checkpoint of non-existent container
docker-cr checkpoint non-existent-container 2>&1 | tee error-log.txt

# 2. Test restore from non-existent checkpoint
docker-cr restore --from /non/existent/path --new-name test 2>&1 | tee -a error-log.txt

# 3. Test inspect of invalid checkpoint
docker-cr inspect /invalid/path 2>&1 | tee -a error-log.txt

# 4. Test checkpoint without permissions (if not root)
docker-cr checkpoint test-container 2>&1 | tee -a error-log.txt

# 5. Review error messages
cat error-log.txt
```

**Expected Results**:
- Clear error messages for each failure case
- No crashes or panic messages
- Helpful suggestions for resolving issues
- Proper exit codes for different error types

**Cleanup**:
```bash
rm -f error-log.txt
```

---

### Test 8: Configuration Options

**Objective**: Test various checkpoint configuration options.

**Steps**:
```bash
# 1. Create test container
docker run -d --name config-test alpine:latest sleep 300

# 2. Test with TCP connections disabled
sudo docker-cr checkpoint config-test --output ./test-checkpoints --name no-tcp --tcp=false

# 3. Test with file locks disabled
sudo docker-cr checkpoint config-test --output ./test-checkpoints --name no-locks --file-locks=false

# 4. Test with cgroups disabled
sudo docker-cr checkpoint config-test --output ./test-checkpoints --name no-cgroups --manage-cgroups=false

# 5. Test with custom output directory
sudo docker-cr checkpoint config-test --output /tmp/custom-checkpoints --name custom-dir

# 6. Verify all checkpoints
ls -la ./test-checkpoints/config-test/
ls -la /tmp/custom-checkpoints/config-test/
```

**Expected Results**:
- All checkpoint variants are created successfully
- Different configurations produce valid checkpoints
- Custom output directories work correctly
- Options are properly applied

**Cleanup**:
```bash
docker stop config-test && docker rm config-test
rm -rf ./test-checkpoints/
rm -rf /tmp/custom-checkpoints/
```

---

## Test Execution Scripts

### Automated Test Runner

Create `test/scripts/run_basic_tests.sh`:

```bash
#!/bin/bash
set -e

echo "=== Basic Functionality Test Suite ==="

# Test counter
TESTS_RUN=0
TESTS_PASSED=0
TESTS_FAILED=0

# Function to run a test
run_test() {
    local test_name="$1"
    local test_command="$2"

    echo
    echo "Running: $test_name"
    echo "----------------------------------------"

    TESTS_RUN=$((TESTS_RUN + 1))

    if eval "$test_command"; then
        echo "✓ PASSED: $test_name"
        TESTS_PASSED=$((TESTS_PASSED + 1))
    else
        echo "✗ FAILED: $test_name"
        TESTS_FAILED=$((TESTS_FAILED + 1))
    fi
}

# Run basic tests
run_test "CLI Version" "docker-cr version"
run_test "CLI Help" "docker-cr --help > /dev/null"
run_test "Simple Checkpoint" "./test_simple_checkpoint.sh"
run_test "Basic Restore" "./test_basic_restore.sh"
run_test "Checkpoint Inspection" "./test_inspection.sh"

# Summary
echo
echo "=== Test Summary ==="
echo "Tests Run: $TESTS_RUN"
echo "Passed: $TESTS_PASSED"
echo "Failed: $TESTS_FAILED"

if [ $TESTS_FAILED -eq 0 ]; then
    echo "All tests passed! ✓"
    exit 0
else
    echo "Some tests failed! ✗"
    exit 1
fi
```

### Individual Test Scripts

Create individual test scripts in `test/scripts/` for each test case above, following the pattern of isolating each test and providing clear output.

## Performance Benchmarks

### Checkpoint Time Measurement

```bash
# Measure checkpoint time
time sudo docker-cr checkpoint test-container --output ./benchmarks

# Measure restore time
time sudo docker-cr restore --from ./benchmarks/test-container/checkpoint --new-name restored
```

### Resource Usage Monitoring

```bash
# Monitor during checkpoint
top -p $(pgrep docker-cr) &
sudo docker-cr checkpoint test-container --output ./monitoring
killall top
```

## Validation Criteria

A test passes if:
1. **Command executes successfully** (exit code 0)
2. **Expected files are created** in correct locations
3. **Container state is preserved** across checkpoint/restore
4. **No error messages** in logs (unless testing error cases)
5. **Resource cleanup** completes successfully

A test fails if:
1. **Command crashes** or returns non-zero exit code
2. **Required files are missing** or corrupted
3. **Container state is lost** or incorrect after restore
4. **Memory leaks** or resource issues occur
5. **Cleanup fails** leaving system in inconsistent state