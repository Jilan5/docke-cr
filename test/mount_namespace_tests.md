# Mount Namespace Tests

This document contains specific tests for mount namespace handling - the core issue that docker-cr was designed to solve.

## Test Suite: Mount Namespace Error Resolution

These tests specifically address the `"mnt: No mapping for 390:(null) mountpoint"` error that occurs during container restoration.

### Test 1: Basic Mount Error Reproduction

**Objective**: Reproduce the original mount namespace error and verify the fix.

**Steps**:
```bash
# 1. Create a container with basic mounts
docker run -d --name mount-test-basic busybox:latest sleep 300

# 2. Checkpoint the container
sudo docker-cr checkpoint mount-test-basic --output ./mount-tests

# 3. Stop and remove original container
docker stop mount-test-basic && docker rm mount-test-basic

# 4. Attempt restore with auto-fix disabled (should show how we handle the error)
sudo docker-cr restore --from ./mount-tests/mount-test-basic/checkpoint \
    --new-name mount-restored-basic \
    --auto-fix-mounts=false \
    --validate-env=true

# 5. Attempt restore with auto-fix enabled (should succeed)
sudo docker-cr restore --from ./mount-tests/mount-test-basic/checkpoint \
    --new-name mount-restored-fixed \
    --auto-fix-mounts=true \
    --validate-env=true

# 6. Verify both approaches
docker ps | grep mount-restored
```

**Expected Results**:
- Checkpoint succeeds
- First restore may show warnings about missing mounts but handles gracefully
- Second restore succeeds with auto-fix
- At least one restore creates a working container

**Cleanup**:
```bash
docker stop mount-restored-basic mount-restored-fixed 2>/dev/null || true
docker rm mount-restored-basic mount-restored-fixed 2>/dev/null || true
rm -rf ./mount-tests/
```

---

### Test 2: Bind Mount Container

**Objective**: Test checkpoint/restore with bind mounts.

**Steps**:
```bash
# 1. Create host directory and content
mkdir -p /tmp/mount-test-data
echo "test data" > /tmp/mount-test-data/test.txt

# 2. Create container with bind mount
docker run -d --name bind-mount-test \
    -v /tmp/mount-test-data:/app/data \
    alpine:latest sleep 300

# 3. Verify mount inside container
docker exec bind-mount-test ls -la /app/data/

# 4. Checkpoint container
sudo docker-cr checkpoint bind-mount-test --output ./mount-tests

# 5. Inspect mount mappings in checkpoint
docker-cr inspect ./mount-tests/bind-mount-test/checkpoint --mounts

# 6. Stop original container
docker stop bind-mount-test && docker rm bind-mount-test

# 7. Restore container
sudo docker-cr restore --from ./mount-tests/bind-mount-test/checkpoint \
    --new-name bind-mount-restored \
    --auto-fix-mounts=true

# 8. Verify mount in restored container
docker exec bind-mount-restored ls -la /app/data/
docker exec bind-mount-restored cat /app/data/test.txt
```

**Expected Results**:
- Container with bind mount starts correctly
- Checkpoint captures mount information
- Mount mappings are visible in inspection
- Restored container has access to the same host directory
- Data is accessible in restored container

**Cleanup**:
```bash
docker stop bind-mount-restored 2>/dev/null || true
docker rm bind-mount-restored 2>/dev/null || true
rm -rf ./mount-tests/
rm -rf /tmp/mount-test-data/
```

---

### Test 3: Missing Mount Source Handling

**Objective**: Test behavior when mount source doesn't exist at restore time.

**Steps**:
```bash
# 1. Create temporary mount source
mkdir -p /tmp/temporary-mount
echo "temporary data" > /tmp/temporary-mount/temp.txt

# 2. Create container with this mount
docker run -d --name missing-mount-test \
    -v /tmp/temporary-mount:/data \
    alpine:latest sleep 300

# 3. Checkpoint container
sudo docker-cr checkpoint missing-mount-test --output ./mount-tests

# 4. Stop container and remove mount source
docker stop missing-mount-test && docker rm missing-mount-test
rm -rf /tmp/temporary-mount

# 5. Attempt restore without auto-fix (should warn)
sudo docker-cr restore --from ./mount-tests/missing-mount-test/checkpoint \
    --new-name missing-mount-no-fix \
    --auto-fix-mounts=false 2>&1 | tee restore-no-fix.log

# 6. Attempt restore with auto-fix (should create missing directory)
sudo docker-cr restore --from ./mount-tests/missing-mount-test/checkpoint \
    --new-name missing-mount-with-fix \
    --auto-fix-mounts=true 2>&1 | tee restore-with-fix.log

# 7. Check if mount directory was created
ls -la /tmp/temporary-mount/ || echo "Mount source not created"

# 8. Check restoration logs
cat restore-no-fix.log
echo "---"
cat restore-with-fix.log
```

**Expected Results**:
- Checkpoint succeeds with mount source present
- First restore warns about missing mount source
- Second restore creates missing mount directory
- Auto-fix restore succeeds

**Cleanup**:
```bash
docker stop missing-mount-no-fix missing-mount-with-fix 2>/dev/null || true
docker rm missing-mount-no-fix missing-mount-with-fix 2>/dev/null || true
rm -rf ./mount-tests/
rm -rf /tmp/temporary-mount/
rm -f restore-*.log
```

---

### Test 4: Skip Problematic Mounts

**Objective**: Test skipping specific mounts during restore.

**Steps**:
```bash
# 1. Create container with multiple mounts
mkdir -p /tmp/mount-good /tmp/mount-problematic
echo "good data" > /tmp/mount-good/data.txt
echo "problematic data" > /tmp/mount-problematic/data.txt

docker run -d --name skip-mount-test \
    -v /tmp/mount-good:/good \
    -v /tmp/mount-problematic:/problematic \
    alpine:latest sleep 300

# 2. Checkpoint container
sudo docker-cr checkpoint skip-mount-test --output ./mount-tests

# 3. Stop container and make one mount problematic
docker stop skip-mount-test && docker rm skip-mount-test
sudo rm -rf /tmp/mount-problematic
sudo touch /tmp/mount-problematic  # Create file instead of directory

# 4. Restore while skipping problematic mount
sudo docker-cr restore --from ./mount-tests/skip-mount-test/checkpoint \
    --new-name skip-mount-restored \
    --skip-mounts="/problematic" \
    --auto-fix-mounts=true

# 5. Verify restoration
docker exec skip-mount-restored ls -la /good/ || echo "Good mount not accessible"
docker exec skip-mount-restored ls -la /problematic/ || echo "Problematic mount skipped (expected)"

# 6. Check mount status in container
docker exec skip-mount-restored mount | grep -E "(good|problematic)"
```

**Expected Results**:
- Container with multiple mounts checkpoints successfully
- Restore succeeds when skipping problematic mount
- Good mount is accessible in restored container
- Problematic mount is skipped (not accessible)

**Cleanup**:
```bash
docker stop skip-mount-restored 2>/dev/null || true
docker rm skip-mount-restored 2>/dev/null || true
rm -rf ./mount-tests/
rm -rf /tmp/mount-good/
rm -f /tmp/mount-problematic
```

---

### Test 5: Complex Mount Scenario

**Objective**: Test complex mount scenarios with nested mounts and special filesystems.

**Steps**:
```bash
# 1. Create complex mount structure
mkdir -p /tmp/complex-mount/{data,config,logs}
echo "app data" > /tmp/complex-mount/data/app.txt
echo "config=value" > /tmp/complex-mount/config/app.conf
echo "log entry" > /tmp/complex-mount/logs/app.log

# 2. Create container with multiple mount types
docker run -d --name complex-mount-test \
    -v /tmp/complex-mount/data:/app/data \
    -v /tmp/complex-mount/config:/app/config:ro \
    -v /tmp/complex-mount/logs:/app/logs \
    --tmpfs /app/temp:size=100m \
    alpine:latest sleep 300

# 3. Add some data to tmpfs
docker exec complex-mount-test touch /app/temp/tmpfile.txt
docker exec complex-mount-test echo "temp data" > /app/temp/tmpfile.txt

# 4. Checkpoint container
sudo docker-cr checkpoint complex-mount-test --output ./mount-tests

# 5. Inspect mount mappings
docker-cr inspect ./mount-tests/complex-mount-test/checkpoint --mounts --format=json > mount-mappings.json
cat mount-mappings.json

# 6. Stop original container
docker stop complex-mount-test && docker rm complex-mount-test

# 7. Restore container
sudo docker-cr restore --from ./mount-tests/complex-mount-test/checkpoint \
    --new-name complex-mount-restored \
    --auto-fix-mounts=true

# 8. Verify all mounts work
docker exec complex-mount-restored ls -la /app/data/
docker exec complex-mount-restored ls -la /app/config/
docker exec complex-mount-restored ls -la /app/logs/
docker exec complex-mount-restored ls -la /app/temp/ || echo "tmpfs may not be restored (expected)"

# 9. Test read-only mount
docker exec complex-mount-restored touch /app/config/new-file.txt 2>&1 | grep -i "read-only" || echo "Read-only not enforced"
```

**Expected Results**:
- Complex mount setup checkpoints successfully
- Mount mappings JSON shows all mount types
- Restored container has access to persistent mounts
- Read-only mount restrictions are preserved
- tmpfs behavior may vary (documented limitation)

**Cleanup**:
```bash
docker stop complex-mount-restored 2>/dev/null || true
docker rm complex-mount-restored 2>/dev/null || true
rm -rf ./mount-tests/
rm -rf /tmp/complex-mount/
rm -f mount-mappings.json
```

---

### Test 6: External Mount Map Validation

**Objective**: Test CRIU external mount mapping functionality.

**Steps**:
```bash
# 1. Create container with external-style mounts
mkdir -p /tmp/external-test/{proc-like,sys-like,dev-like}

docker run -d --name external-mount-test \
    -v /tmp/external-test/proc-like:/proc-external \
    -v /tmp/external-test/sys-like:/sys-external \
    -v /tmp/external-test/dev-like:/dev-external \
    alpine:latest sleep 300

# 2. Checkpoint with detailed logging
sudo docker-cr checkpoint external-mount-test \
    --output ./mount-tests \
    --verbose

# 3. Check the generated external mount map
cat ./mount-tests/external-mount-test/checkpoint/ext_mount_map

# 4. Check CRIU logs for mount handling
grep -i "mount" ./mount-tests/external-mount-test/checkpoint/checkpoint.log || echo "No mount entries in log"

# 5. Stop and restore
docker stop external-mount-test && docker rm external-mount-test

sudo docker-cr restore --from ./mount-tests/external-mount-test/checkpoint \
    --new-name external-mount-restored \
    --verbose

# 6. Check restore logs for mount handling
grep -i "mount" ./mount-tests/external-mount-test/checkpoint/restore.log || echo "No mount entries in restore log"

# 7. Verify mounts in restored container
docker exec external-mount-restored ls -la /proc-external/ || echo "Proc-external not accessible"
docker exec external-mount-restored ls -la /sys-external/ || echo "Sys-external not accessible"
docker exec external-mount-restored ls -la /dev-external/ || echo "Dev-external not accessible"
```

**Expected Results**:
- External mount map file is created
- CRIU logs show mount processing
- Restore handles external mounts correctly
- Restored container has access to mapped mounts

**Cleanup**:
```bash
docker stop external-mount-restored 2>/dev/null || true
docker rm external-mount-restored 2>/dev/null || true
rm -rf ./mount-tests/
rm -rf /tmp/external-test/
```

---

### Test 7: Mount Namespace Validation

**Objective**: Test the mount namespace validation and preparation.

**Steps**:
```bash
# 1. Create container with various mount scenarios
mkdir -p /tmp/validation-test/{existing,missing-will-create}
echo "exists" > /tmp/validation-test/existing/file.txt

docker run -d --name validation-test \
    -v /tmp/validation-test/existing:/mnt/existing \
    -v /tmp/validation-test/missing-will-create:/mnt/missing \
    alpine:latest sleep 300

# 2. Checkpoint container
sudo docker-cr checkpoint validation-test --output ./mount-tests

# 3. Stop container and remove one mount source
docker stop validation-test && docker rm validation-test
rm -rf /tmp/validation-test/missing-will-create

# 4. Test validation with auto-fix disabled
sudo docker-cr restore --from ./mount-tests/validation-test/checkpoint \
    --new-name validation-no-fix \
    --auto-fix-mounts=false \
    --validate-env=true 2>&1 | tee validation-output.txt

# 5. Test validation with auto-fix enabled
sudo docker-cr restore --from ./mount-tests/validation-test/checkpoint \
    --new-name validation-with-fix \
    --auto-fix-mounts=true \
    --validate-env=true 2>&1 | tee -a validation-output.txt

# 6. Check validation output
echo "=== Validation Output ==="
cat validation-output.txt

# 7. Verify mount source was created
ls -la /tmp/validation-test/missing-will-create/ && echo "Missing mount source was created"

# 8. Test container functionality
docker exec validation-with-fix ls -la /mnt/existing/
docker exec validation-with-fix ls -la /mnt/missing/
```

**Expected Results**:
- Validation detects missing mount sources
- Auto-fix creates missing directories
- Both containers can be restored (with different handling)
- Validation output provides clear information

**Cleanup**:
```bash
docker stop validation-no-fix validation-with-fix 2>/dev/null || true
docker rm validation-no-fix validation-with-fix 2>/dev/null || true
rm -rf ./mount-tests/
rm -rf /tmp/validation-test/
rm -f validation-output.txt
```

---

## Mount Error Test Automation

### Automated Mount Test Runner

Create `test/scripts/test_mount_issues.sh`:

```bash
#!/bin/bash
# Test script specifically for mount namespace issues

set -e

echo "=== Mount Namespace Error Resolution Tests ==="
echo "Testing the fixes for 'mnt: No mapping for X:(null) mountpoint' errors"
echo

# Setup
TEST_DIR="./mount-tests-$(date +%s)"
TEMP_MOUNT_BASE="/tmp/docker-cr-mount-tests"

# Cleanup function
cleanup() {
    echo "Cleaning up test containers and data..."
    docker stop $(docker ps -q --filter "name=mount-test-") 2>/dev/null || true
    docker rm $(docker ps -aq --filter "name=mount-test-") 2>/dev/null || true
    rm -rf "$TEST_DIR"
    rm -rf "$TEMP_MOUNT_BASE"
}

# Trap cleanup on exit
trap cleanup EXIT

# Test 1: Basic mount error reproduction and fix
test_basic_mount_fix() {
    echo "Test 1: Basic mount error fix"

    # Create container
    docker run -d --name mount-test-basic alpine:latest sleep 180

    # Checkpoint
    sudo docker-cr checkpoint mount-test-basic --output "$TEST_DIR"

    # Stop and restore
    docker stop mount-test-basic && docker rm mount-test-basic

    # Test restore with auto-fix
    if sudo docker-cr restore --from "$TEST_DIR/mount-test-basic/checkpoint" \
        --new-name mount-test-basic-restored \
        --auto-fix-mounts=true; then
        echo "✓ Basic mount restore succeeded"
        return 0
    else
        echo "✗ Basic mount restore failed"
        return 1
    fi
}

# Test 2: Missing mount source handling
test_missing_mount_source() {
    echo "Test 2: Missing mount source handling"

    # Create mount source
    mkdir -p "$TEMP_MOUNT_BASE/test-source"
    echo "test data" > "$TEMP_MOUNT_BASE/test-source/file.txt"

    # Create container with mount
    docker run -d --name mount-test-missing \
        -v "$TEMP_MOUNT_BASE/test-source:/data" \
        alpine:latest sleep 180

    # Checkpoint
    sudo docker-cr checkpoint mount-test-missing --output "$TEST_DIR"

    # Remove container and mount source
    docker stop mount-test-missing && docker rm mount-test-missing
    rm -rf "$TEMP_MOUNT_BASE/test-source"

    # Test restore with auto-fix (should create missing directory)
    if sudo docker-cr restore --from "$TEST_DIR/mount-test-missing/checkpoint" \
        --new-name mount-test-missing-restored \
        --auto-fix-mounts=true; then

        # Check if missing directory was created
        if [ -d "$TEMP_MOUNT_BASE/test-source" ]; then
            echo "✓ Missing mount source was auto-created"
            return 0
        else
            echo "✗ Missing mount source was not created"
            return 1
        fi
    else
        echo "✗ Restore with missing mount source failed"
        return 1
    fi
}

# Test 3: Skip mount functionality
test_skip_mount() {
    echo "Test 3: Skip mount functionality"

    # Create mount sources
    mkdir -p "$TEMP_MOUNT_BASE"/{good,bad}
    echo "good" > "$TEMP_MOUNT_BASE/good/file.txt"
    echo "bad" > "$TEMP_MOUNT_BASE/bad/file.txt"

    # Create container
    docker run -d --name mount-test-skip \
        -v "$TEMP_MOUNT_BASE/good:/good" \
        -v "$TEMP_MOUNT_BASE/bad:/bad" \
        alpine:latest sleep 180

    # Checkpoint
    sudo docker-cr checkpoint mount-test-skip --output "$TEST_DIR"

    # Stop container and make one mount problematic
    docker stop mount-test-skip && docker rm mount-test-skip
    rm -rf "$TEMP_MOUNT_BASE/bad"
    touch "$TEMP_MOUNT_BASE/bad"  # Create file instead of directory

    # Test restore while skipping problematic mount
    if sudo docker-cr restore --from "$TEST_DIR/mount-test-skip/checkpoint" \
        --new-name mount-test-skip-restored \
        --skip-mounts="/bad" \
        --auto-fix-mounts=true; then
        echo "✓ Skip mount functionality works"
        return 0
    else
        echo "✗ Skip mount functionality failed"
        return 1
    fi
}

# Run tests
TESTS_PASSED=0
TESTS_TOTAL=3

echo "Running mount namespace tests..."

if test_basic_mount_fix; then
    TESTS_PASSED=$((TESTS_PASSED + 1))
fi

if test_missing_mount_source; then
    TESTS_PASSED=$((TESTS_PASSED + 1))
fi

if test_skip_mount; then
    TESTS_PASSED=$((TESTS_PASSED + 1))
fi

# Summary
echo
echo "=== Mount Test Results ==="
echo "Passed: $TESTS_PASSED/$TESTS_TOTAL"

if [ $TESTS_PASSED -eq $TESTS_TOTAL ]; then
    echo "All mount namespace tests passed! ✓"
    echo "The mount namespace error fixes are working correctly."
    exit 0
else
    echo "Some mount namespace tests failed! ✗"
    exit 1
fi
```

---

## Mount Error Debugging Guide

### Common Mount Errors and Solutions

1. **"No mapping for X:(null) mountpoint"**
   - **Cause**: CRIU cannot find mount source
   - **Solution**: Use `--auto-fix-mounts=true`

2. **"Permission denied" on mount**
   - **Cause**: Insufficient privileges
   - **Solution**: Run with `sudo`

3. **"Device or resource busy"**
   - **Cause**: Mount point in use
   - **Solution**: Stop processes using mount or use `--skip-mounts`

4. **"Invalid argument" on mount**
   - **Cause**: Mount options incompatible
   - **Solution**: Check mount options or skip problematic mounts

### Debug Commands

```bash
# Check mount mappings in checkpoint
docker-cr inspect checkpoint-dir --mounts --format=json

# Check CRIU logs for mount issues
grep -i mount checkpoint-dir/checkpoint.log
grep -i mount checkpoint-dir/restore.log

# Check current mounts in container
docker exec container-name mount

# Check mount namespace
docker exec container-name cat /proc/mounts
```

This comprehensive test suite ensures that the mount namespace fixes work correctly and handles the specific errors that docker-cr was designed to solve.