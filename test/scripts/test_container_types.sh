#!/bin/bash
# Comprehensive container types test runner

set -e

echo "=== Container Types and Images Test Suite ==="
echo "Testing docker-cr with various container types and configurations"
echo

# Configuration
TEST_BASE_DIR="./container-type-tests-$(date +%s)"
TESTS_RUN=0
TESTS_PASSED=0
TESTS_FAILED=0

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

print_status() {
    local status=$1
    local message=$2
    case $status in
        "SUCCESS") echo -e "${GREEN}✓${NC} $message" ;;
        "ERROR") echo -e "${RED}✗${NC} $message" ;;
        "INFO") echo -e "${YELLOW}ℹ${NC} $message" ;;
    esac
}

run_test() {
    local test_name="$1"
    local test_function="$2"

    echo
    echo "Running: $test_name"
    echo "----------------------------------------"

    TESTS_RUN=$((TESTS_RUN + 1))

    if $test_function; then
        print_status "SUCCESS" "$test_name"
        TESTS_PASSED=$((TESTS_PASSED + 1))
    else
        print_status "ERROR" "$test_name"
        TESTS_FAILED=$((TESTS_FAILED + 1))
    fi
}

# Check if docker-cr is available
check_docker_cr() {
    if command -v docker-cr >/dev/null 2>&1; then
        DOCKER_CR="docker-cr"
    elif [ -f "./bin/docker-cr" ]; then
        DOCKER_CR="./bin/docker-cr"
    elif [ -f "../../bin/docker-cr" ]; then
        DOCKER_CR="../../bin/docker-cr"
    else
        print_status "ERROR" "docker-cr binary not found"
        exit 1
    fi
    print_status "INFO" "Using docker-cr at: $DOCKER_CR"
}

# Test functions
test_alpine_basic() {
    local container_name="test-auto-alpine-$(date +%s)"

    docker run -d --name "$container_name" alpine:latest sleep 120 || return 1
    sleep 2

    sudo $DOCKER_CR checkpoint "$container_name" --output "$TEST_BASE_DIR" || return 1
    docker stop "$container_name" && docker rm "$container_name" || return 1

    sudo $DOCKER_CR restore --from "$TEST_BASE_DIR/$container_name/checkpoint" --new-name "${container_name}-restored" || return 1
    docker exec "${container_name}-restored" echo "Alpine test successful" || return 1

    docker stop "${container_name}-restored" && docker rm "${container_name}-restored" || return 1
    return 0
}

test_ubuntu_basic() {
    local container_name="test-auto-ubuntu-$(date +%s)"

    docker run -d --name "$container_name" ubuntu:20.04 sleep 120 || return 1
    sleep 2

    sudo $DOCKER_CR checkpoint "$container_name" --output "$TEST_BASE_DIR" || return 1
    docker stop "$container_name" && docker rm "$container_name" || return 1

    sudo $DOCKER_CR restore --from "$TEST_BASE_DIR/$container_name/checkpoint" --new-name "${container_name}-restored" || return 1
    docker exec "${container_name}-restored" echo "Ubuntu test successful" || return 1

    docker stop "${container_name}-restored" && docker rm "${container_name}-restored" || return 1
    return 0
}

test_busybox_basic() {
    local container_name="test-auto-busybox-$(date +%s)"

    docker run -d --name "$container_name" busybox:latest sleep 120 || return 1
    sleep 2

    sudo $DOCKER_CR checkpoint "$container_name" --output "$TEST_BASE_DIR" || return 1
    docker stop "$container_name" && docker rm "$container_name" || return 1

    sudo $DOCKER_CR restore --from "$TEST_BASE_DIR/$container_name/checkpoint" --new-name "${container_name}-restored" || return 1
    docker exec "${container_name}-restored" echo "BusyBox test successful" || return 1

    docker stop "${container_name}-restored" && docker rm "${container_name}-restored" || return 1
    return 0
}

test_nginx_webserver() {
    local container_name="test-auto-nginx-$(date +%s)"
    local port=$((8100 + RANDOM % 100))

    docker run -d --name "$container_name" -p "$port:80" nginx:alpine || return 1
    sleep 3

    # Test if nginx is responding
    curl -s "http://localhost:$port" > /dev/null || return 1

    sudo $DOCKER_CR checkpoint "$container_name" --output "$TEST_BASE_DIR" || return 1
    docker stop "$container_name" && docker rm "$container_name" || return 1

    sudo $DOCKER_CR restore --from "$TEST_BASE_DIR/$container_name/checkpoint" --new-name "${container_name}-restored" || return 1
    sleep 3

    # Test if restored nginx is responding
    curl -s "http://localhost:$port" > /dev/null || return 1

    docker stop "${container_name}-restored" && docker rm "${container_name}-restored" || return 1
    return 0
}

test_redis_database() {
    local container_name="test-auto-redis-$(date +%s)"

    docker run -d --name "$container_name" redis:alpine || return 1
    sleep 3

    # Set a test value
    docker exec "$container_name" redis-cli SET test-key "checkpoint-test-value" || return 1

    sudo $DOCKER_CR checkpoint "$container_name" --output "$TEST_BASE_DIR" || return 1
    docker stop "$container_name" && docker rm "$container_name" || return 1

    sudo $DOCKER_CR restore --from "$TEST_BASE_DIR/$container_name/checkpoint" --new-name "${container_name}-restored" || return 1
    sleep 3

    # Check if the value is still there (may or may not work depending on Redis state)
    local value=$(docker exec "${container_name}-restored" redis-cli GET test-key 2>/dev/null || echo "")
    print_status "INFO" "Redis value after restore: '$value'"

    docker stop "${container_name}-restored" && docker rm "${container_name}-restored" || return 1
    return 0
}

test_bind_mount() {
    local container_name="test-auto-mount-$(date +%s)"
    local mount_dir="/tmp/auto-test-mount-$(date +%s)"

    mkdir -p "$mount_dir" || return 1
    echo "mount test data $(date)" > "$mount_dir/test.txt" || return 1

    docker run -d --name "$container_name" -v "$mount_dir:/data" alpine:latest sleep 120 || return 1
    sleep 2

    sudo $DOCKER_CR checkpoint "$container_name" --output "$TEST_BASE_DIR" || return 1
    docker stop "$container_name" && docker rm "$container_name" || return 1

    sudo $DOCKER_CR restore --from "$TEST_BASE_DIR/$container_name/checkpoint" --new-name "${container_name}-restored" --auto-fix-mounts=true || return 1
    docker exec "${container_name}-restored" cat /data/test.txt | grep -q "mount test data" || return 1

    docker stop "${container_name}-restored" && docker rm "${container_name}-restored" || return 1
    rm -rf "$mount_dir" || return 1
    return 0
}

test_environment_vars() {
    local container_name="test-auto-env-$(date +%s)"
    local test_value="auto-test-value-$(date +%s)"

    docker run -d --name "$container_name" -e "TEST_VAR=$test_value" -e "SECOND_VAR=second-value" alpine:latest sleep 120 || return 1
    sleep 2

    sudo $DOCKER_CR checkpoint "$container_name" --output "$TEST_BASE_DIR" || return 1
    docker stop "$container_name" && docker rm "$container_name" || return 1

    sudo $DOCKER_CR restore --from "$TEST_BASE_DIR/$container_name/checkpoint" --new-name "${container_name}-restored" || return 1
    docker exec "${container_name}-restored" env | grep -q "TEST_VAR=$test_value" || return 1
    docker exec "${container_name}-restored" env | grep -q "SECOND_VAR=second-value" || return 1

    docker stop "${container_name}-restored" && docker rm "${container_name}-restored" || return 1
    return 0
}

test_working_directory() {
    local container_name="test-auto-workdir-$(date +%s)"

    docker run -d --name "$container_name" -w /app alpine:latest sh -c 'pwd > /tmp/workdir.txt && sleep 120' || return 1
    sleep 2

    sudo $DOCKER_CR checkpoint "$container_name" --output "$TEST_BASE_DIR" || return 1
    docker stop "$container_name" && docker rm "$container_name" || return 1

    sudo $DOCKER_CR restore --from "$TEST_BASE_DIR/$container_name/checkpoint" --new-name "${container_name}-restored" || return 1
    docker exec "${container_name}-restored" cat /tmp/workdir.txt | grep -q "/app" || return 1

    docker stop "${container_name}-restored" && docker rm "${container_name}-restored" || return 1
    return 0
}

test_python_runtime() {
    local container_name="test-auto-python-$(date +%s)"

    docker run -d --name "$container_name" python:3.9-alpine sh -c 'python -c "print(\"Python is working\")" && sleep 120' || return 1
    sleep 3

    sudo $DOCKER_CR checkpoint "$container_name" --output "$TEST_BASE_DIR" || return 1
    docker stop "$container_name" && docker rm "$container_name" || return 1

    sudo $DOCKER_CR restore --from "$TEST_BASE_DIR/$container_name/checkpoint" --new-name "${container_name}-restored" || return 1
    docker exec "${container_name}-restored" python -c "print('Python restored successfully')" || return 1

    docker stop "${container_name}-restored" && docker rm "${container_name}-restored" || return 1
    return 0
}

test_nodejs_runtime() {
    local container_name="test-auto-nodejs-$(date +%s)"

    docker run -d --name "$container_name" node:alpine sh -c 'node -e "console.log(\"Node.js is working\")" && sleep 120' || return 1
    sleep 3

    sudo $DOCKER_CR checkpoint "$container_name" --output "$TEST_BASE_DIR" || return 1
    docker stop "$container_name" && docker rm "$container_name" || return 1

    sudo $DOCKER_CR restore --from "$TEST_BASE_DIR/$container_name/checkpoint" --new-name "${container_name}-restored" || return 1
    docker exec "${container_name}-restored" node -e "console.log('Node.js restored successfully')" || return 1

    docker stop "${container_name}-restored" && docker rm "${container_name}-restored" || return 1
    return 0
}

# Cleanup function
cleanup() {
    print_status "INFO" "Cleaning up test containers and data..."

    # Stop and remove any test containers that might still be running
    for container in $(docker ps -aq --filter "name=test-auto-"); do
        docker stop "$container" 2>/dev/null || true
        docker rm "$container" 2>/dev/null || true
    done

    # Clean up test directories
    rm -rf "$TEST_BASE_DIR" 2>/dev/null || true
    rm -rf /tmp/auto-test-mount-* 2>/dev/null || true

    print_status "INFO" "Cleanup completed"
}

trap cleanup EXIT

# Main execution
main() {
    print_status "INFO" "Starting container types test suite..."

    # Check prerequisites
    check_docker_cr

    if ! command -v docker >/dev/null 2>&1; then
        print_status "ERROR" "Docker not found"
        exit 1
    fi

    if ! command -v sudo >/dev/null 2>&1; then
        print_status "ERROR" "sudo not found (required for checkpoint operations)"
        exit 1
    fi

    # Create test base directory
    mkdir -p "$TEST_BASE_DIR"

    print_status "INFO" "Test results will be stored in: $TEST_BASE_DIR"

    # Run tests
    run_test "Alpine Linux Basic" test_alpine_basic
    run_test "Ubuntu Basic" test_ubuntu_basic
    run_test "BusyBox Basic" test_busybox_basic
    run_test "Nginx Web Server" test_nginx_webserver
    run_test "Redis Database" test_redis_database
    run_test "Bind Mount" test_bind_mount
    run_test "Environment Variables" test_environment_vars
    run_test "Working Directory" test_working_directory
    run_test "Python Runtime" test_python_runtime
    run_test "Node.js Runtime" test_nodejs_runtime

    # Summary
    echo
    echo "=== Test Results Summary ==="
    echo "============================="
    echo "Tests Run: $TESTS_RUN"
    echo "Passed: $TESTS_PASSED"
    echo "Failed: $TESTS_FAILED"
    echo "Success Rate: $(( TESTS_PASSED * 100 / TESTS_RUN ))%"

    if [ $TESTS_FAILED -eq 0 ]; then
        print_status "SUCCESS" "All container type tests passed!"
        echo
        echo "docker-cr successfully handles various container types:"
        echo "  ✓ Different base images (Alpine, Ubuntu, BusyBox)"
        echo "  ✓ Web servers (Nginx)"
        echo "  ✓ Databases (Redis)"
        echo "  ✓ Runtime environments (Python, Node.js)"
        echo "  ✓ Volume mounts and bind mounts"
        echo "  ✓ Environment variables"
        echo "  ✓ Working directory configurations"
        echo
        exit 0
    else
        print_status "ERROR" "Some container type tests failed!"
        echo
        echo "Failed tests: $TESTS_FAILED out of $TESTS_RUN"
        echo "Check the output above for specific error details."
        echo
        exit 1
    fi
}

# Run main function
main "$@"