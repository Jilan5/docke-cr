#!/bin/bash
# Basic test script for docker-cr - similar to the one created by deploy script

set -e

echo "=== Docker-CR Basic Functionality Test ==="
echo

# Configuration
TEST_CONTAINER="docker-cr-test-$(date +%s)"
CHECKPOINT_DIR="./test-checkpoints"
RESTORED_CONTAINER="${TEST_CONTAINER}-restored"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    local status=$1
    local message=$2
    case $status in
        "SUCCESS")
            echo -e "${GREEN}✓${NC} $message"
            ;;
        "ERROR")
            echo -e "${RED}✗${NC} $message"
            ;;
        "INFO")
            echo -e "${YELLOW}ℹ${NC} $message"
            ;;
    esac
}

# Function to check if command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Cleanup function
cleanup() {
    echo
    print_status "INFO" "Cleaning up test containers and data..."

    # Stop and remove containers
    docker stop "$TEST_CONTAINER" "$RESTORED_CONTAINER" 2>/dev/null || true
    docker rm "$TEST_CONTAINER" "$RESTORED_CONTAINER" 2>/dev/null || true

    # Remove checkpoint data
    rm -rf "$CHECKPOINT_DIR"

    print_status "INFO" "Cleanup completed"
}

# Trap cleanup on exit
trap cleanup EXIT

# Check prerequisites
echo "Checking prerequisites..."

if ! command_exists docker; then
    print_status "ERROR" "Docker not found. Please install Docker first."
    exit 1
fi

if ! command_exists docker-cr; then
    if [ ! -f "./bin/docker-cr" ]; then
        print_status "ERROR" "docker-cr not found. Please build it first with 'make build'"
        exit 1
    fi
    DOCKER_CR="./bin/docker-cr"
else
    DOCKER_CR="docker-cr"
fi

if ! command_exists criu; then
    print_status "ERROR" "CRIU not found. Please install CRIU first."
    exit 1
fi

print_status "SUCCESS" "All prerequisites satisfied"

# Test 1: Check docker-cr version
echo
echo "Test 1: Checking docker-cr version"
echo "-----------------------------------"

if $DOCKER_CR version; then
    print_status "SUCCESS" "Version command works"
else
    print_status "ERROR" "Version command failed"
    exit 1
fi

# Test 2: Create test container
echo
echo "Test 2: Creating test container"
echo "------------------------------"

print_status "INFO" "Creating test container: $TEST_CONTAINER"

if docker run -d --name "$TEST_CONTAINER" alpine:latest sleep 300; then
    print_status "SUCCESS" "Test container created"
else
    print_status "ERROR" "Failed to create test container"
    exit 1
fi

# Wait for container to be ready
print_status "INFO" "Waiting for container to be ready..."
sleep 2

# Verify container is running
if docker ps | grep -q "$TEST_CONTAINER"; then
    print_status "SUCCESS" "Container is running"
else
    print_status "ERROR" "Container is not running"
    exit 1
fi

# Test 3: Checkpoint container
echo
echo "Test 3: Checkpointing container"
echo "-------------------------------"

print_status "INFO" "Creating checkpoint..."

if sudo $DOCKER_CR checkpoint "$TEST_CONTAINER" --output "$CHECKPOINT_DIR" --name test-checkpoint; then
    print_status "SUCCESS" "Checkpoint created successfully"
else
    print_status "ERROR" "Checkpoint creation failed"
    exit 1
fi

# Verify checkpoint files exist
CHECKPOINT_PATH="$CHECKPOINT_DIR/$TEST_CONTAINER/test-checkpoint"

if [ -d "$CHECKPOINT_PATH" ]; then
    print_status "SUCCESS" "Checkpoint directory created"
else
    print_status "ERROR" "Checkpoint directory not found"
    exit 1
fi

# Check for required files
REQUIRED_FILES=("container_metadata.json" "mount_mappings.json" "checkpoint_metadata.json" "images")

for file in "${REQUIRED_FILES[@]}"; do
    if [ -e "$CHECKPOINT_PATH/$file" ]; then
        print_status "SUCCESS" "Required file/directory found: $file"
    else
        print_status "ERROR" "Required file/directory missing: $file"
        exit 1
    fi
done

# Test 4: Inspect checkpoint
echo
echo "Test 4: Inspecting checkpoint"
echo "-----------------------------"

print_status "INFO" "Running checkpoint inspection..."

if $DOCKER_CR inspect "$CHECKPOINT_PATH" --summary; then
    print_status "SUCCESS" "Checkpoint inspection successful"
else
    print_status "ERROR" "Checkpoint inspection failed"
    exit 1
fi

# Test 5: Stop original container
echo
echo "Test 5: Stopping original container"
echo "-----------------------------------"

print_status "INFO" "Stopping and removing original container..."

if docker stop "$TEST_CONTAINER" && docker rm "$TEST_CONTAINER"; then
    print_status "SUCCESS" "Original container removed"
else
    print_status "ERROR" "Failed to remove original container"
    exit 1
fi

# Verify container is gone
if ! docker ps -a | grep -q "$TEST_CONTAINER"; then
    print_status "SUCCESS" "Container completely removed"
else
    print_status "ERROR" "Container still exists"
    exit 1
fi

# Test 6: Restore container
echo
echo "Test 6: Restoring container"
echo "---------------------------"

print_status "INFO" "Restoring container as: $RESTORED_CONTAINER"

if sudo $DOCKER_CR restore --from "$CHECKPOINT_PATH" --new-name "$RESTORED_CONTAINER"; then
    print_status "SUCCESS" "Container restore completed"
else
    print_status "ERROR" "Container restore failed"
    # Don't exit here, let's check what happened
fi

# Test 7: Verify restored container
echo
echo "Test 7: Verifying restored container"
echo "------------------------------------"

# Check if container exists
if docker ps -a | grep -q "$RESTORED_CONTAINER"; then
    print_status "SUCCESS" "Restored container exists"

    # Check if container is running
    if docker ps | grep -q "$RESTORED_CONTAINER"; then
        print_status "SUCCESS" "Restored container is running"

        # Get container info
        print_status "INFO" "Restored container details:"
        docker ps --filter "name=$RESTORED_CONTAINER" --format "table {{.Names}}\t{{.Status}}\t{{.Image}}"

    else
        print_status "ERROR" "Restored container exists but is not running"
        print_status "INFO" "Container status:"
        docker ps -a --filter "name=$RESTORED_CONTAINER" --format "table {{.Names}}\t{{.Status}}\t{{.Image}}"
    fi
else
    print_status "ERROR" "Restored container does not exist"
fi

# Test 8: Container functionality
echo
echo "Test 8: Testing container functionality"
echo "--------------------------------------"

if docker ps | grep -q "$RESTORED_CONTAINER"; then
    print_status "INFO" "Testing basic container commands..."

    # Test basic command execution
    if docker exec "$RESTORED_CONTAINER" echo "Hello from restored container"; then
        print_status "SUCCESS" "Container command execution works"
    else
        print_status "ERROR" "Container command execution failed"
    fi

    # Test file system access
    if docker exec "$RESTORED_CONTAINER" ls -la /; then
        print_status "SUCCESS" "Container filesystem access works"
    else
        print_status "ERROR" "Container filesystem access failed"
    fi

    # Test process list
    if docker exec "$RESTORED_CONTAINER" ps aux; then
        print_status "SUCCESS" "Container process listing works"
    else
        print_status "ERROR" "Container process listing failed"
    fi
else
    print_status "ERROR" "Cannot test functionality - container not running"
fi

# Test 9: Detailed inspection
echo
echo "Test 9: Detailed checkpoint analysis"
echo "------------------------------------"

print_status "INFO" "Running detailed checkpoint inspection..."

echo "Process tree:"
$DOCKER_CR inspect "$CHECKPOINT_PATH" --ps-tree || print_status "ERROR" "Process tree inspection failed"

echo
echo "Environment variables:"
$DOCKER_CR inspect "$CHECKPOINT_PATH" --env || print_status "ERROR" "Environment inspection failed"

echo
echo "Mount mappings:"
$DOCKER_CR inspect "$CHECKPOINT_PATH" --mounts || print_status "ERROR" "Mount inspection failed"

# Test Summary
echo
echo "=== Test Summary ==="
echo "===================="

# Count checkpoint files
CHECKPOINT_FILES=$(find "$CHECKPOINT_PATH/images" -name "*.img" 2>/dev/null | wc -l)
print_status "INFO" "Checkpoint contains $CHECKPOINT_FILES CRIU image files"

# Check logs
if [ -f "$CHECKPOINT_PATH/checkpoint.log" ]; then
    LOG_SIZE=$(wc -l < "$CHECKPOINT_PATH/checkpoint.log")
    print_status "INFO" "Checkpoint log contains $LOG_SIZE lines"

    # Check for errors in log
    if grep -qi error "$CHECKPOINT_PATH/checkpoint.log"; then
        print_status "ERROR" "Errors found in checkpoint log"
        echo "Recent errors:"
        grep -i error "$CHECKPOINT_PATH/checkpoint.log" | tail -5
    else
        print_status "SUCCESS" "No errors in checkpoint log"
    fi
fi

# Final status
if docker ps | grep -q "$RESTORED_CONTAINER"; then
    print_status "SUCCESS" "Basic test completed successfully!"
    print_status "INFO" "Container $RESTORED_CONTAINER is running and functional"
    echo
    echo "You can interact with the restored container using:"
    echo "  docker exec -it $RESTORED_CONTAINER sh"
    echo
    echo "To clean up manually:"
    echo "  docker stop $RESTORED_CONTAINER && docker rm $RESTORED_CONTAINER"
    echo "  rm -rf $CHECKPOINT_DIR"
    exit 0
else
    print_status "ERROR" "Basic test completed with issues"
    print_status "INFO" "Check the logs above for details"
    exit 1
fi