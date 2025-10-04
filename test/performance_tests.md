# Performance Tests

This document outlines performance testing procedures for docker-cr to measure checkpoint/restore times, resource usage, and scalability.

## Performance Test Categories

### 1. Checkpoint Time Measurements

#### Test 1.1: Container Size Impact

**Objective**: Measure checkpoint time based on container size and complexity.

```bash
# Test 1: Minimal container (Alpine)
echo "=== Testing Minimal Container (Alpine) ==="
docker run -d --name perf-alpine-minimal alpine:latest sleep 300

echo "Checkpointing minimal container..."
time sudo docker-cr checkpoint perf-alpine-minimal --output ./perf-tests --name time-test

echo "Checkpoint size:"
du -sh ./perf-tests/perf-alpine-minimal/time-test/

# Test 2: Medium container (Ubuntu with packages)
echo "=== Testing Medium Container (Ubuntu + packages) ==="
docker run -d --name perf-ubuntu-medium ubuntu:20.04 sh -c 'apt update && apt install -y curl wget vim && sleep 300'

echo "Checkpointing medium container..."
time sudo docker-cr checkpoint perf-ubuntu-medium --output ./perf-tests --name time-test

echo "Checkpoint size:"
du -sh ./perf-tests/perf-ubuntu-medium/time-test/

# Test 3: Large container (Full development environment)
echo "=== Testing Large Container (Development environment) ==="
docker run -d --name perf-ubuntu-large ubuntu:20.04 sh -c '
    apt update &&
    apt install -y build-essential python3 python3-pip nodejs npm git vim curl wget htop &&
    pip3 install flask django requests &&
    npm install -g express lodash &&
    sleep 300'

echo "Checkpointing large container..."
time sudo docker-cr checkpoint perf-ubuntu-large --output ./perf-tests --name time-test

echo "Checkpoint size:"
du -sh ./perf-tests/perf-ubuntu-large/time-test/

# Cleanup
docker stop perf-alpine-minimal perf-ubuntu-medium perf-ubuntu-large 2>/dev/null || true
docker rm perf-alpine-minimal perf-ubuntu-medium perf-ubuntu-large 2>/dev/null || true
```

#### Test 1.2: Memory Usage Impact

**Objective**: Test checkpoint time with different memory usage patterns.

```bash
# Test 1: Low memory usage
echo "=== Testing Low Memory Usage ==="
docker run -d --name perf-mem-low alpine:latest sh -c 'dd if=/dev/zero of=/tmp/file bs=1M count=10 && sleep 300'

echo "Checkpointing low memory container..."
time sudo docker-cr checkpoint perf-mem-low --output ./perf-tests --name mem-test

# Test 2: Medium memory usage
echo "=== Testing Medium Memory Usage ==="
docker run -d --name perf-mem-medium alpine:latest sh -c 'dd if=/dev/zero of=/tmp/file bs=1M count=100 && sleep 300'

echo "Checkpointing medium memory container..."
time sudo docker-cr checkpoint perf-mem-medium --output ./perf-tests --name mem-test

# Test 3: High memory usage
echo "=== Testing High Memory Usage ==="
docker run -d --name perf-mem-high alpine:latest sh -c 'dd if=/dev/zero of=/tmp/file bs=1M count=500 && sleep 300'

echo "Checkpointing high memory container..."
time sudo docker-cr checkpoint perf-mem-high --output ./perf-tests --name mem-test

# Show checkpoint sizes
echo "Checkpoint sizes:"
du -sh ./perf-tests/perf-mem-*/mem-test/

# Cleanup
docker stop perf-mem-low perf-mem-medium perf-mem-high 2>/dev/null || true
docker rm perf-mem-low perf-mem-medium perf-mem-high 2>/dev/null || true
```

### 2. Restore Time Measurements

#### Test 2.1: Restore Performance

**Objective**: Measure restore times for different container types.

```bash
# Setup: Create checkpoints for testing
docker run -d --name restore-test-1 alpine:latest sleep 60
docker run -d --name restore-test-2 nginx:alpine
docker run -d --name restore-test-3 -v /tmp/restore-test:/data alpine:latest sleep 60

mkdir -p /tmp/restore-test
echo "test data" > /tmp/restore-test/file.txt

sudo docker-cr checkpoint restore-test-1 --output ./perf-tests --name restore-perf
sudo docker-cr checkpoint restore-test-2 --output ./perf-tests --name restore-perf
sudo docker-cr checkpoint restore-test-3 --output ./perf-tests --name restore-perf

docker stop restore-test-1 restore-test-2 restore-test-3
docker rm restore-test-1 restore-test-2 restore-test-3

# Test restore times
echo "=== Restore Performance Tests ==="

echo "Restoring simple Alpine container..."
time sudo docker-cr restore --from ./perf-tests/restore-test-1/restore-perf --new-name restore-test-1-new

echo "Restoring Nginx container..."
time sudo docker-cr restore --from ./perf-tests/restore-test-2/restore-perf --new-name restore-test-2-new

echo "Restoring container with bind mount..."
time sudo docker-cr restore --from ./perf-tests/restore-test-3/restore-perf --new-name restore-test-3-new --auto-fix-mounts=true

# Verify restored containers
docker ps | grep restore-test-.*-new

# Cleanup
docker stop restore-test-1-new restore-test-2-new restore-test-3-new 2>/dev/null || true
docker rm restore-test-1-new restore-test-2-new restore-test-3-new 2>/dev/null || true
rm -rf /tmp/restore-test
```

### 3. Resource Usage Monitoring

#### Test 3.1: CPU and Memory Usage

**Objective**: Monitor resource usage during checkpoint/restore operations.

```bash
# Create monitoring script
cat > monitor_resources.sh << 'EOF'
#!/bin/bash
LOG_FILE="$1"
PROCESS_NAME="$2"
INTERVAL="$3"

echo "timestamp,cpu_percent,memory_mb,processes" > "$LOG_FILE"

while true; do
    TIMESTAMP=$(date '+%Y-%m-%d %H:%M:%S')

    # Get CPU and memory usage for docker-cr processes
    STATS=$(ps aux | grep "$PROCESS_NAME" | grep -v grep | awk '{cpu+=$3; mem+=$6} END {print cpu","mem/1024}')

    if [ -n "$STATS" ]; then
        PROCESS_COUNT=$(ps aux | grep "$PROCESS_NAME" | grep -v grep | wc -l)
        echo "$TIMESTAMP,$STATS,$PROCESS_COUNT" >> "$LOG_FILE"
    fi

    sleep "$INTERVAL"
done
EOF

chmod +x monitor_resources.sh

# Test resource usage during checkpoint
echo "=== Resource Usage During Checkpoint ==="

# Start a large container
docker run -d --name resource-test ubuntu:20.04 sh -c 'apt update && apt install -y build-essential && sleep 600'

# Start monitoring
./monitor_resources.sh checkpoint_resources.csv "docker-cr" 1 &
MONITOR_PID=$!

# Perform checkpoint
sudo docker-cr checkpoint resource-test --output ./perf-tests --name resource-test

# Stop monitoring
kill $MONITOR_PID 2>/dev/null || true

echo "Resource usage log:"
cat checkpoint_resources.csv

# Test resource usage during restore
echo "=== Resource Usage During Restore ==="
docker stop resource-test && docker rm resource-test

./monitor_resources.sh restore_resources.csv "docker-cr" 1 &
MONITOR_PID=$!

sudo docker-cr restore --from ./perf-tests/resource-test/resource-test --new-name resource-test-restored

kill $MONITOR_PID 2>/dev/null || true

echo "Restore resource usage log:"
cat restore_resources.csv

# Cleanup
docker stop resource-test-restored 2>/dev/null || true
docker rm resource-test-restored 2>/dev/null || true
rm -f monitor_resources.sh checkpoint_resources.csv restore_resources.csv
```

### 4. Scalability Tests

#### Test 4.1: Multiple Containers

**Objective**: Test performance with multiple containers.

```bash
# Test checkpointing multiple containers
echo "=== Multiple Container Checkpoint Test ==="

CONTAINER_COUNT=5
CHECKPOINT_TIMES=()

echo "Creating $CONTAINER_COUNT test containers..."
for i in $(seq 1 $CONTAINER_COUNT); do
    docker run -d --name "scale-test-$i" alpine:latest sleep 300
done

echo "Checkpointing containers individually..."
for i in $(seq 1 $CONTAINER_COUNT); do
    echo "Checkpointing container $i..."
    START_TIME=$(date +%s.%N)
    sudo docker-cr checkpoint "scale-test-$i" --output ./perf-tests --name "scale-checkpoint"
    END_TIME=$(date +%s.%N)

    DURATION=$(echo "$END_TIME - $START_TIME" | bc -l)
    CHECKPOINT_TIMES+=("$DURATION")
    echo "Container $i checkpoint time: ${DURATION}s"
done

# Calculate average checkpoint time
TOTAL_TIME=0
for time in "${CHECKPOINT_TIMES[@]}"; do
    TOTAL_TIME=$(echo "$TOTAL_TIME + $time" | bc -l)
done
AVERAGE_TIME=$(echo "scale=3; $TOTAL_TIME / $CONTAINER_COUNT" | bc -l)

echo "Average checkpoint time: ${AVERAGE_TIME}s"

# Test parallel restore
echo "Stopping all containers..."
for i in $(seq 1 $CONTAINER_COUNT); do
    docker stop "scale-test-$i" && docker rm "scale-test-$i"
done

echo "Restoring containers in parallel..."
START_TIME=$(date +%s.%N)

for i in $(seq 1 $CONTAINER_COUNT); do
    sudo docker-cr restore --from "./perf-tests/scale-test-$i/scale-checkpoint" --new-name "scale-test-$i-restored" &
done

wait  # Wait for all background processes

END_TIME=$(date +%s.%N)
PARALLEL_DURATION=$(echo "$END_TIME - $START_TIME" | bc -l)

echo "Parallel restore time for $CONTAINER_COUNT containers: ${PARALLEL_DURATION}s"

# Verify all containers are running
RUNNING_COUNT=$(docker ps | grep "scale-test-.*-restored" | wc -l)
echo "Successfully restored containers: $RUNNING_COUNT/$CONTAINER_COUNT"

# Cleanup
for i in $(seq 1 $CONTAINER_COUNT); do
    docker stop "scale-test-$i-restored" 2>/dev/null || true
    docker rm "scale-test-$i-restored" 2>/dev/null || true
done
```

#### Test 4.2: Large Number of Files

**Objective**: Test performance with containers having many files.

```bash
echo "=== Large File Count Performance Test ==="

# Test 1: Container with many small files
echo "Creating container with many small files..."
docker run -d --name perf-many-files alpine:latest sh -c '
    mkdir -p /test-files
    for i in $(seq 1 1000); do
        echo "File $i content" > "/test-files/file_$i.txt"
    done
    sleep 300'

echo "Checkpointing container with many files..."
time sudo docker-cr checkpoint perf-many-files --output ./perf-tests --name many-files-test

echo "Checkpoint size:"
du -sh ./perf-tests/perf-many-files/many-files-test/

# Test 2: Container with fewer large files
echo "Creating container with large files..."
docker run -d --name perf-large-files alpine:latest sh -c '
    mkdir -p /test-files
    for i in $(seq 1 10); do
        dd if=/dev/zero of="/test-files/large_file_$i.dat" bs=1M count=10
    done
    sleep 300'

echo "Checkpointing container with large files..."
time sudo docker-cr checkpoint perf-large-files --output ./perf-tests --name large-files-test

echo "Checkpoint size:"
du -sh ./perf-tests/perf-large-files/large-files-test/

# Compare checkpoint sizes and times
echo "=== Comparison ==="
echo "Many small files:"
du -sh ./perf-tests/perf-many-files/many-files-test/

echo "Fewer large files:"
du -sh ./perf-tests/perf-large-files/large-files-test/

# Cleanup
docker stop perf-many-files perf-large-files 2>/dev/null || true
docker rm perf-many-files perf-large-files 2>/dev/null || true
```

### 5. Network Performance Tests

#### Test 5.1: Network-Active Containers

**Objective**: Test checkpoint/restore performance with network activity.

```bash
echo "=== Network Performance Tests ==="

# Test 1: Web server under load
echo "Testing web server checkpoint under load..."

# Start nginx
docker run -d --name perf-nginx -p 8200:80 nginx:alpine

# Generate some load (background)
for i in {1..10}; do
    curl -s http://localhost:8200 > /dev/null &
done

sleep 2

# Checkpoint under load
echo "Checkpointing nginx under load..."
time sudo docker-cr checkpoint perf-nginx --output ./perf-tests --name load-test

# Stop load
killall curl 2>/dev/null || true

# Test restore
docker stop perf-nginx && docker rm perf-nginx

echo "Restoring nginx..."
time sudo docker-cr restore --from ./perf-tests/perf-nginx/load-test --new-name perf-nginx-restored

# Verify nginx is working
sleep 2
curl -s http://localhost:8200 > /dev/null && echo "Nginx responding after restore" || echo "Nginx not responding"

# Cleanup
docker stop perf-nginx-restored 2>/dev/null || true
docker rm perf-nginx-restored 2>/dev/null || true
```

### 6. Performance Test Automation

#### Test 6.1: Automated Performance Suite

Create `test/scripts/performance_test.sh`:

```bash
#!/bin/bash
# Automated performance test suite

set -e

echo "=== Docker-CR Performance Test Suite ==="
echo

# Configuration
PERF_TEST_DIR="./performance-results-$(date +%s)"
REPORT_FILE="$PERF_TEST_DIR/performance_report.txt"

# Ensure bc is available for calculations
if ! command -v bc >/dev/null 2>&1; then
    echo "Installing bc for calculations..."
    sudo apt-get update && sudo apt-get install -y bc
fi

mkdir -p "$PERF_TEST_DIR"

# Report functions
init_report() {
    cat > "$REPORT_FILE" << EOF
Docker-CR Performance Test Report
=================================
Date: $(date)
System: $(uname -a)
Docker Version: $(docker --version)
CRIU Version: $(criu --version 2>/dev/null || echo "Unknown")

EOF
}

add_to_report() {
    echo "$1" >> "$REPORT_FILE"
}

# Performance test functions
test_checkpoint_time() {
    local container_name="$1"
    local test_name="$2"

    local start_time=$(date +%s.%N)
    sudo docker-cr checkpoint "$container_name" --output "$PERF_TEST_DIR" --name "perf-test" > /dev/null 2>&1
    local end_time=$(date +%s.%N)

    local duration=$(echo "$end_time - $start_time" | bc -l)
    echo "$duration"
}

test_restore_time() {
    local container_name="$1"
    local new_name="$2"

    local start_time=$(date +%s.%N)
    sudo docker-cr restore --from "$PERF_TEST_DIR/$container_name/perf-test" --new-name "$new_name" > /dev/null 2>&1
    local end_time=$(date +%s.%N)

    local duration=$(echo "$end_time - $start_time" | bc -l)
    echo "$duration"
}

# Cleanup function
cleanup() {
    echo "Cleaning up performance test containers..."
    docker stop $(docker ps -q --filter "name=perf-") 2>/dev/null || true
    docker rm $(docker ps -aq --filter "name=perf-") 2>/dev/null || true
    rm -rf /tmp/perf-test-*
}

trap cleanup EXIT

# Initialize report
init_report

# Test 1: Basic container sizes
add_to_report "1. Checkpoint Times by Container Size"
add_to_report "===================================="

echo "Testing checkpoint times for different container sizes..."

# Small container (Alpine)
docker run -d --name perf-small alpine:latest sleep 300
small_time=$(test_checkpoint_time "perf-small" "Small Container")
add_to_report "Small Container (Alpine): ${small_time}s"
docker stop perf-small && docker rm perf-small

# Medium container (Ubuntu)
docker run -d --name perf-medium ubuntu:20.04 sleep 300
medium_time=$(test_checkpoint_time "perf-medium" "Medium Container")
add_to_report "Medium Container (Ubuntu): ${medium_time}s"
docker stop perf-medium && docker rm perf-medium

# Large container (Ubuntu + packages)
docker run -d --name perf-large ubuntu:20.04 sh -c 'apt update && apt install -y curl wget vim && sleep 300'
large_time=$(test_checkpoint_time "perf-large" "Large Container")
add_to_report "Large Container (Ubuntu + packages): ${large_time}s"
docker stop perf-large && docker rm perf-large

add_to_report ""

# Test 2: Restore times
add_to_report "2. Restore Times"
add_to_report "==============="

echo "Testing restore times..."

# Create test containers for restore
docker run -d --name perf-restore-test alpine:latest sleep 60
sudo docker-cr checkpoint perf-restore-test --output "$PERF_TEST_DIR" --name "restore-test" > /dev/null 2>&1
docker stop perf-restore-test && docker rm perf-restore-test

restore_time=$(test_restore_time "perf-restore-test" "perf-restore-test-new")
add_to_report "Basic Restore (Alpine): ${restore_time}s"
docker stop perf-restore-test-new && docker rm perf-restore-test-new

add_to_report ""

# Test 3: Resource usage summary
add_to_report "3. Checkpoint Sizes"
add_to_report "=================="

# Calculate checkpoint sizes
for checkpoint_dir in "$PERF_TEST_DIR"/*/perf-test; do
    if [ -d "$checkpoint_dir" ]; then
        container_name=$(basename $(dirname "$checkpoint_dir"))
        size=$(du -sh "$checkpoint_dir" | cut -f1)
        add_to_report "$container_name: $size"
    fi
done

add_to_report ""

# Test 4: Performance summary
add_to_report "4. Performance Summary"
add_to_report "====================="

# Calculate averages if we have enough data
if [ -n "$small_time" ] && [ -n "$medium_time" ] && [ -n "$large_time" ]; then
    avg_time=$(echo "scale=3; ($small_time + $medium_time + $large_time) / 3" | bc -l)
    add_to_report "Average Checkpoint Time: ${avg_time}s"
fi

add_to_report ""
add_to_report "Performance test completed at $(date)"

# Display results
echo
echo "Performance test completed!"
echo "Results saved to: $REPORT_FILE"
echo
echo "Summary:"
cat "$REPORT_FILE"

# Generate CSV for analysis
CSV_FILE="$PERF_TEST_DIR/performance_data.csv"
cat > "$CSV_FILE" << EOF
container_type,checkpoint_time,checkpoint_size
small,$small_time,$(du -sb "$PERF_TEST_DIR/perf-small/perf-test" 2>/dev/null | cut -f1 || echo 0)
medium,$medium_time,$(du -sb "$PERF_TEST_DIR/perf-medium/perf-test" 2>/dev/null | cut -f1 || echo 0)
large,$large_time,$(du -sb "$PERF_TEST_DIR/perf-large/perf-test" 2>/dev/null | cut -f1 || echo 0)
EOF

echo "CSV data saved to: $CSV_FILE"
```

### 7. Performance Benchmarking

#### Benchmark Comparison Script

Create `test/scripts/benchmark_comparison.sh`:

```bash
#!/bin/bash
# Compare docker-cr performance with Docker's native checkpoint (if available)

echo "=== Performance Benchmark Comparison ==="

# Test container
docker run -d --name benchmark-test alpine:latest sleep 300

# Test docker-cr
echo "Testing docker-cr performance..."
time sudo docker-cr checkpoint benchmark-test --output ./benchmark-tests --name docker-cr-test

# Test Docker native checkpoint (experimental feature)
echo "Testing Docker native checkpoint (if available)..."
if docker checkpoint create benchmark-test docker-native-test 2>/dev/null; then
    echo "Docker native checkpoint succeeded"
else
    echo "Docker native checkpoint not available or failed"
fi

# Compare checkpoint sizes
echo "Checkpoint size comparison:"
echo "docker-cr: $(du -sh ./benchmark-tests/benchmark-test/docker-cr-test/ | cut -f1)"

if [ -d "/var/lib/docker/containers/$(docker inspect -f '{{.Id}}' benchmark-test)/checkpoints/docker-native-test" ]; then
    echo "docker native: $(du -sh /var/lib/docker/containers/$(docker inspect -f '{{.Id}}' benchmark-test)/checkpoints/docker-native-test | cut -f1)"
fi

# Cleanup
docker stop benchmark-test
docker rm benchmark-test
rm -rf ./benchmark-tests
```

## Performance Analysis

### Key Metrics to Monitor

1. **Checkpoint Time**: Time to create checkpoint
2. **Restore Time**: Time to restore from checkpoint
3. **Checkpoint Size**: Storage space used
4. **Memory Usage**: Peak memory during operations
5. **CPU Usage**: CPU utilization during operations
6. **I/O Operations**: Disk read/write activity

### Expected Performance Ranges

- **Small containers (Alpine)**: 1-5 seconds checkpoint time
- **Medium containers (Ubuntu)**: 5-15 seconds checkpoint time
- **Large containers**: 15-60 seconds checkpoint time
- **Checkpoint sizes**: 10MB - 1GB depending on container content
- **Memory overhead**: 50-200MB during operations

### Performance Optimization Tips

1. **Use minimal base images** when possible
2. **Avoid unnecessary file creation** in containers
3. **Consider pre-dump** for faster incremental checkpoints
4. **Use SSD storage** for checkpoint data
5. **Ensure sufficient RAM** for large container checkpoints
6. **Monitor network activity** during checkpoint of network-active containers