# Container Types and Images Tests

This document provides comprehensive tests for different container types, images, and configurations to validate docker-cr functionality across various scenarios.

## Test Categories by Container Type

### 1. Operating System Base Images

#### Test 1.1: Alpine Linux Containers

**Objective**: Test with minimal Alpine Linux containers.

```bash
# Test 1: Basic Alpine
docker run -d --name test-alpine-basic alpine:latest sleep 300
sudo docker-cr checkpoint test-alpine-basic --output ./test-checkpoints
docker stop test-alpine-basic && docker rm test-alpine-basic
sudo docker-cr restore --from ./test-checkpoints/test-alpine-basic/checkpoint --new-name test-alpine-restored

# Test 2: Alpine with packages
docker run -d --name test-alpine-packages alpine:latest sh -c 'apk add --no-cache curl && sleep 300'
sudo docker-cr checkpoint test-alpine-packages --output ./test-checkpoints
docker stop test-alpine-packages && docker rm test-alpine-packages
sudo docker-cr restore --from ./test-checkpoints/test-alpine-packages/checkpoint --new-name test-alpine-packages-restored

# Test 3: Alpine with custom entrypoint
docker run -d --name test-alpine-custom alpine:latest sh -c 'echo "Custom startup" && sleep 300'
sudo docker-cr checkpoint test-alpine-custom --output ./test-checkpoints
docker stop test-alpine-custom && docker rm test-alpine-custom
sudo docker-cr restore --from ./test-checkpoints/test-alpine-custom/checkpoint --new-name test-alpine-custom-restored

# Cleanup
docker stop test-alpine-basic test-alpine-packages test-alpine-custom test-alpine-restored test-alpine-packages-restored test-alpine-custom-restored 2>/dev/null || true
docker rm test-alpine-basic test-alpine-packages test-alpine-custom test-alpine-restored test-alpine-packages-restored test-alpine-custom-restored 2>/dev/null || true
```

#### Test 1.2: Ubuntu Containers

**Objective**: Test with Ubuntu-based containers.

```bash
# Test 1: Ubuntu LTS
docker run -d --name test-ubuntu-lts ubuntu:20.04 sleep 300
sudo docker-cr checkpoint test-ubuntu-lts --output ./test-checkpoints
docker stop test-ubuntu-lts && docker rm test-ubuntu-lts
sudo docker-cr restore --from ./test-checkpoints/test-ubuntu-lts/checkpoint --new-name test-ubuntu-lts-restored

# Test 2: Ubuntu with updates
docker run -d --name test-ubuntu-updated ubuntu:20.04 sh -c 'apt update && apt install -y curl && sleep 300'
sudo docker-cr checkpoint test-ubuntu-updated --output ./test-checkpoints
docker stop test-ubuntu-updated && docker rm test-ubuntu-updated
sudo docker-cr restore --from ./test-checkpoints/test-ubuntu-updated/checkpoint --new-name test-ubuntu-updated-restored

# Test 3: Ubuntu latest
docker run -d --name test-ubuntu-latest ubuntu:latest sleep 300
sudo docker-cr checkpoint test-ubuntu-latest --output ./test-checkpoints
docker stop test-ubuntu-latest && docker rm test-ubuntu-latest
sudo docker-cr restore --from ./test-checkpoints/test-ubuntu-latest/checkpoint --new-name test-ubuntu-latest-restored

# Cleanup
docker stop test-ubuntu-lts test-ubuntu-updated test-ubuntu-latest test-ubuntu-lts-restored test-ubuntu-updated-restored test-ubuntu-latest-restored 2>/dev/null || true
docker rm test-ubuntu-lts test-ubuntu-updated test-ubuntu-latest test-ubuntu-lts-restored test-ubuntu-updated-restored test-ubuntu-latest-restored 2>/dev/null || true
```

#### Test 1.3: Debian Containers

**Objective**: Test with Debian-based containers.

```bash
# Test 1: Debian stable
docker run -d --name test-debian-stable debian:stable sleep 300
sudo docker-cr checkpoint test-debian-stable --output ./test-checkpoints
docker stop test-debian-stable && docker rm test-debian-stable
sudo docker-cr restore --from ./test-checkpoints/test-debian-stable/checkpoint --new-name test-debian-stable-restored

# Test 2: Debian slim
docker run -d --name test-debian-slim debian:stable-slim sleep 300
sudo docker-cr checkpoint test-debian-slim --output ./test-checkpoints
docker stop test-debian-slim && docker rm test-debian-slim
sudo docker-cr restore --from ./test-checkpoints/test-debian-slim/checkpoint --new-name test-debian-slim-restored

# Cleanup
docker stop test-debian-stable test-debian-slim test-debian-stable-restored test-debian-slim-restored 2>/dev/null || true
docker rm test-debian-stable test-debian-slim test-debian-stable-restored test-debian-slim-restored 2>/dev/null || true
```

#### Test 1.4: CentOS/RHEL Containers

**Objective**: Test with Red Hat family containers.

```bash
# Test 1: CentOS
docker run -d --name test-centos centos:7 sleep 300
sudo docker-cr checkpoint test-centos --output ./test-checkpoints
docker stop test-centos && docker rm test-centos
sudo docker-cr restore --from ./test-checkpoints/test-centos/checkpoint --new-name test-centos-restored

# Test 2: Rocky Linux (CentOS replacement)
docker run -d --name test-rocky rockylinux:8 sleep 300
sudo docker-cr checkpoint test-rocky --output ./test-checkpoints
docker stop test-rocky && docker rm test-rocky
sudo docker-cr restore --from ./test-checkpoints/test-rocky/checkpoint --new-name test-rocky-restored

# Cleanup
docker stop test-centos test-rocky test-centos-restored test-rocky-restored 2>/dev/null || true
docker rm test-centos test-rocky test-centos-restored test-rocky-restored 2>/dev/null || true
```

---

### 2. Application Runtime Images

#### Test 2.1: Web Servers

**Objective**: Test web server containers.

```bash
# Test 1: Nginx
docker run -d --name test-nginx -p 8080:80 nginx:alpine
curl -s http://localhost:8080 > /dev/null && echo "Nginx is responding"
sudo docker-cr checkpoint test-nginx --output ./test-checkpoints
docker stop test-nginx && docker rm test-nginx
sudo docker-cr restore --from ./test-checkpoints/test-nginx/checkpoint --new-name test-nginx-restored
sleep 2
curl -s http://localhost:8080 > /dev/null && echo "Restored Nginx is responding" || echo "Restored Nginx not responding"

# Test 2: Apache
docker run -d --name test-apache -p 8081:80 httpd:alpine
curl -s http://localhost:8081 > /dev/null && echo "Apache is responding"
sudo docker-cr checkpoint test-apache --output ./test-checkpoints
docker stop test-apache && docker rm test-apache
sudo docker-cr restore --from ./test-checkpoints/test-apache/checkpoint --new-name test-apache-restored
sleep 2
curl -s http://localhost:8081 > /dev/null && echo "Restored Apache is responding" || echo "Restored Apache not responding"

# Test 3: Nginx with custom config
docker run -d --name test-nginx-custom -p 8082:80 -v /tmp/nginx-config:/etc/nginx/conf.d nginx:alpine
sudo docker-cr checkpoint test-nginx-custom --output ./test-checkpoints
docker stop test-nginx-custom && docker rm test-nginx-custom
sudo docker-cr restore --from ./test-checkpoints/test-nginx-custom/checkpoint --new-name test-nginx-custom-restored --auto-fix-mounts=true

# Cleanup
docker stop test-nginx test-apache test-nginx-custom test-nginx-restored test-apache-restored test-nginx-custom-restored 2>/dev/null || true
docker rm test-nginx test-apache test-nginx-custom test-nginx-restored test-apache-restored test-nginx-custom-restored 2>/dev/null || true
```

#### Test 2.2: Database Containers

**Objective**: Test database containers (Note: These may be more complex due to data consistency).

```bash
# Test 1: Redis (simpler, in-memory)
docker run -d --name test-redis redis:alpine
docker exec test-redis redis-cli SET test-key "test-value"
REDIS_VALUE=$(docker exec test-redis redis-cli GET test-key)
echo "Redis value before checkpoint: $REDIS_VALUE"
sudo docker-cr checkpoint test-redis --output ./test-checkpoints
docker stop test-redis && docker rm test-redis
sudo docker-cr restore --from ./test-checkpoints/test-redis/checkpoint --new-name test-redis-restored
sleep 2
RESTORED_VALUE=$(docker exec test-redis-restored redis-cli GET test-key 2>/dev/null || echo "FAILED")
echo "Redis value after restore: $RESTORED_VALUE"

# Test 2: SQLite (file-based, simpler than networked DBs)
docker run -d --name test-sqlite -v /tmp/sqlite-data:/data alpine:latest sh -c 'apk add --no-cache sqlite && echo "CREATE TABLE test (id INTEGER, value TEXT); INSERT INTO test VALUES (1, \"checkpoint-test\");" | sqlite3 /data/test.db && sleep 300'
sudo docker-cr checkpoint test-sqlite --output ./test-checkpoints
docker stop test-sqlite && docker rm test-sqlite
sudo docker-cr restore --from ./test-checkpoints/test-sqlite/checkpoint --new-name test-sqlite-restored --auto-fix-mounts=true
sleep 2
SQLITE_RESULT=$(docker exec test-sqlite-restored sh -c 'sqlite3 /data/test.db "SELECT value FROM test WHERE id=1;"' 2>/dev/null || echo "FAILED")
echo "SQLite data after restore: $SQLITE_RESULT"

# Cleanup
docker stop test-redis test-sqlite test-redis-restored test-sqlite-restored 2>/dev/null || true
docker rm test-redis test-sqlite test-redis-restored test-sqlite-restored 2>/dev/null || true
rm -rf /tmp/sqlite-data
```

#### Test 2.3: Programming Language Runtimes

**Objective**: Test containers with different programming language runtimes.

```bash
# Test 1: Node.js
docker run -d --name test-nodejs node:alpine sh -c 'echo "console.log(\"Hello from Node.js\");" > app.js && node app.js && sleep 300'
sudo docker-cr checkpoint test-nodejs --output ./test-checkpoints
docker stop test-nodejs && docker rm test-nodejs
sudo docker-cr restore --from ./test-checkpoints/test-nodejs/checkpoint --new-name test-nodejs-restored

# Test 2: Python
docker run -d --name test-python python:alpine sh -c 'echo "print(\"Hello from Python\")" > app.py && python app.py && sleep 300'
sudo docker-cr checkpoint test-python --output ./test-checkpoints
docker stop test-python && docker rm test-python
sudo docker-cr restore --from ./test-checkpoints/test-python/checkpoint --new-name test-python-restored

# Test 3: Go runtime
docker run -d --name test-golang golang:alpine sh -c 'echo "package main; import \"fmt\"; func main() { fmt.Println(\"Hello from Go\") }" > main.go && go run main.go && sleep 300'
sudo docker-cr checkpoint test-golang --output ./test-checkpoints
docker stop test-golang && docker rm test-golang
sudo docker-cr restore --from ./test-checkpoints/test-golang/checkpoint --new-name test-golang-restored

# Test 4: Java
docker run -d --name test-java openjdk:8-alpine sh -c 'echo "public class Hello { public static void main(String[] args) { System.out.println(\"Hello from Java\"); } }" > Hello.java && javac Hello.java && java Hello && sleep 300'
sudo docker-cr checkpoint test-java --output ./test-checkpoints
docker stop test-java && docker rm test-java
sudo docker-cr restore --from ./test-checkpoints/test-java/checkpoint --new-name test-java-restored

# Cleanup
docker stop test-nodejs test-python test-golang test-java test-nodejs-restored test-python-restored test-golang-restored test-java-restored 2>/dev/null || true
docker rm test-nodejs test-python test-golang test-java test-nodejs-restored test-python-restored test-golang-restored test-java-restored 2>/dev/null || true
```

---

### 3. Containers with Different Configurations

#### Test 3.1: Environment Variables

**Objective**: Test containers with various environment variable configurations.

```bash
# Test 1: Single environment variable
docker run -d --name test-env-single -e TEST_VAR="checkpoint-test" alpine:latest sh -c 'echo "TEST_VAR=$TEST_VAR" && sleep 300'
docker exec test-env-single env | grep TEST_VAR
sudo docker-cr checkpoint test-env-single --output ./test-checkpoints
docker stop test-env-single && docker rm test-env-single
sudo docker-cr restore --from ./test-checkpoints/test-env-single/checkpoint --new-name test-env-single-restored
docker exec test-env-single-restored env | grep TEST_VAR

# Test 2: Multiple environment variables
docker run -d --name test-env-multiple \
    -e VAR1="value1" \
    -e VAR2="value2" \
    -e VAR3="value with spaces" \
    alpine:latest sh -c 'env | grep VAR && sleep 300'

sudo docker-cr checkpoint test-env-multiple --output ./test-checkpoints
docker stop test-env-multiple && docker rm test-env-multiple
sudo docker-cr restore --from ./test-checkpoints/test-env-multiple/checkpoint --new-name test-env-multiple-restored
docker exec test-env-multiple-restored env | grep VAR

# Test 3: Environment file
echo "FILE_VAR1=from_file1" > /tmp/test-env-file
echo "FILE_VAR2=from_file2" >> /tmp/test-env-file
docker run -d --name test-env-file --env-file /tmp/test-env-file alpine:latest sh -c 'env | grep FILE_VAR && sleep 300'
sudo docker-cr checkpoint test-env-file --output ./test-checkpoints
docker stop test-env-file && docker rm test-env-file
sudo docker-cr restore --from ./test-checkpoints/test-env-file/checkpoint --new-name test-env-file-restored
docker exec test-env-file-restored env | grep FILE_VAR

# Cleanup
docker stop test-env-single test-env-multiple test-env-file test-env-single-restored test-env-multiple-restored test-env-file-restored 2>/dev/null || true
docker rm test-env-single test-env-multiple test-env-file test-env-single-restored test-env-multiple-restored test-env-file-restored 2>/dev/null || true
rm -f /tmp/test-env-file
```

#### Test 3.2: Working Directory and User

**Objective**: Test containers with different working directories and user configurations.

```bash
# Test 1: Custom working directory
docker run -d --name test-workdir -w /app alpine:latest sh -c 'pwd > /tmp/workdir.txt && sleep 300'
docker exec test-workdir cat /tmp/workdir.txt
sudo docker-cr checkpoint test-workdir --output ./test-checkpoints
docker stop test-workdir && docker rm test-workdir
sudo docker-cr restore --from ./test-checkpoints/test-workdir/checkpoint --new-name test-workdir-restored
docker exec test-workdir-restored cat /tmp/workdir.txt

# Test 2: Custom user (if supported)
docker run -d --name test-user --user 1000:1000 alpine:latest sh -c 'id > /tmp/user.txt && sleep 300'
docker exec test-user cat /tmp/user.txt
sudo docker-cr checkpoint test-user --output ./test-checkpoints
docker stop test-user && docker rm test-user
sudo docker-cr restore --from ./test-checkpoints/test-user/checkpoint --new-name test-user-restored
docker exec test-user-restored cat /tmp/user.txt

# Cleanup
docker stop test-workdir test-user test-workdir-restored test-user-restored 2>/dev/null || true
docker rm test-workdir test-user test-workdir-restored test-user-restored 2>/dev/null || true
```

#### Test 3.3: Resource Limits

**Objective**: Test containers with resource constraints.

```bash
# Test 1: Memory limit
docker run -d --name test-memory-limit --memory=128m alpine:latest sh -c 'cat /proc/meminfo | head -5 && sleep 300'
sudo docker-cr checkpoint test-memory-limit --output ./test-checkpoints
docker stop test-memory-limit && docker rm test-memory-limit
sudo docker-cr restore --from ./test-checkpoints/test-memory-limit/checkpoint --new-name test-memory-limit-restored

# Test 2: CPU limit
docker run -d --name test-cpu-limit --cpus=0.5 alpine:latest sh -c 'cat /proc/cpuinfo | grep processor && sleep 300'
sudo docker-cr checkpoint test-cpu-limit --output ./test-checkpoints
docker stop test-cpu-limit && docker rm test-cpu-limit
sudo docker-cr restore --from ./test-checkpoints/test-cpu-limit/checkpoint --new-name test-cpu-limit-restored

# Test 3: Combined limits
docker run -d --name test-combined-limits --memory=256m --cpus=0.5 alpine:latest sleep 300
sudo docker-cr checkpoint test-combined-limits --output ./test-checkpoints
docker stop test-combined-limits && docker rm test-combined-limits
sudo docker-cr restore --from ./test-checkpoints/test-combined-limits/checkpoint --new-name test-combined-limits-restored

# Cleanup
docker stop test-memory-limit test-cpu-limit test-combined-limits test-memory-limit-restored test-cpu-limit-restored test-combined-limits-restored 2>/dev/null || true
docker rm test-memory-limit test-cpu-limit test-combined-limits test-memory-limit-restored test-cpu-limit-restored test-combined-limits-restored 2>/dev/null || true
```

---

### 4. Volume and Mount Tests

#### Test 4.1: Bind Mounts

**Objective**: Test containers with bind mounts.

```bash
# Setup test directories
mkdir -p /tmp/bind-test/{data,config,logs}
echo "Application data" > /tmp/bind-test/data/app.txt
echo "config=production" > /tmp/bind-test/config/app.conf
echo "Application started" > /tmp/bind-test/logs/app.log

# Test 1: Single bind mount
docker run -d --name test-bind-single -v /tmp/bind-test/data:/app/data alpine:latest sh -c 'ls -la /app/data && sleep 300'
sudo docker-cr checkpoint test-bind-single --output ./test-checkpoints
docker stop test-bind-single && docker rm test-bind-single
sudo docker-cr restore --from ./test-checkpoints/test-bind-single/checkpoint --new-name test-bind-single-restored --auto-fix-mounts=true

# Test 2: Multiple bind mounts
docker run -d --name test-bind-multiple \
    -v /tmp/bind-test/data:/app/data \
    -v /tmp/bind-test/config:/app/config:ro \
    -v /tmp/bind-test/logs:/app/logs \
    alpine:latest sh -c 'find /app -type f && sleep 300'

sudo docker-cr checkpoint test-bind-multiple --output ./test-checkpoints
docker stop test-bind-multiple && docker rm test-bind-multiple
sudo docker-cr restore --from ./test-checkpoints/test-bind-multiple/checkpoint --new-name test-bind-multiple-restored --auto-fix-mounts=true

# Test 3: Nested bind mounts
mkdir -p /tmp/bind-test/nested/deep/path
echo "Deep data" > /tmp/bind-test/nested/deep/path/deep.txt
docker run -d --name test-bind-nested -v /tmp/bind-test/nested:/app/nested alpine:latest sh -c 'find /app/nested && sleep 300'
sudo docker-cr checkpoint test-bind-nested --output ./test-checkpoints
docker stop test-bind-nested && docker rm test-bind-nested
sudo docker-cr restore --from ./test-checkpoints/test-bind-nested/checkpoint --new-name test-bind-nested-restored --auto-fix-mounts=true

# Cleanup
docker stop test-bind-single test-bind-multiple test-bind-nested test-bind-single-restored test-bind-multiple-restored test-bind-nested-restored 2>/dev/null || true
docker rm test-bind-single test-bind-multiple test-bind-nested test-bind-single-restored test-bind-multiple-restored test-bind-nested-restored 2>/dev/null || true
rm -rf /tmp/bind-test
```

#### Test 4.2: Named Volumes

**Objective**: Test containers with Docker named volumes.

```bash
# Test 1: Single named volume
docker volume create test-volume-1
docker run -d --name test-volume-single -v test-volume-1:/data alpine:latest sh -c 'echo "Volume data" > /data/volume.txt && sleep 300'
sudo docker-cr checkpoint test-volume-single --output ./test-checkpoints
docker stop test-volume-single && docker rm test-volume-single
sudo docker-cr restore --from ./test-checkpoints/test-volume-single/checkpoint --new-name test-volume-single-restored --auto-fix-mounts=true

# Test 2: Multiple named volumes
docker volume create test-volume-2
docker volume create test-volume-3
docker run -d --name test-volume-multiple \
    -v test-volume-2:/app/data \
    -v test-volume-3:/app/config \
    alpine:latest sh -c 'echo "data" > /app/data/data.txt && echo "config" > /app/config/config.txt && sleep 300'

sudo docker-cr checkpoint test-volume-multiple --output ./test-checkpoints
docker stop test-volume-multiple && docker rm test-volume-multiple
sudo docker-cr restore --from ./test-checkpoints/test-volume-multiple/checkpoint --new-name test-volume-multiple-restored --auto-fix-mounts=true

# Cleanup
docker stop test-volume-single test-volume-multiple test-volume-single-restored test-volume-multiple-restored 2>/dev/null || true
docker rm test-volume-single test-volume-multiple test-volume-single-restored test-volume-multiple-restored 2>/dev/null || true
docker volume rm test-volume-1 test-volume-2 test-volume-3 2>/dev/null || true
```

#### Test 4.3: tmpfs Mounts

**Objective**: Test containers with tmpfs mounts.

```bash
# Test 1: Single tmpfs mount
docker run -d --name test-tmpfs-single --tmpfs /tmp:size=100m alpine:latest sh -c 'echo "tmpfs data" > /tmp/tmpfs.txt && sleep 300'
sudo docker-cr checkpoint test-tmpfs-single --output ./test-checkpoints
docker stop test-tmpfs-single && docker rm test-tmpfs-single
sudo docker-cr restore --from ./test-checkpoints/test-tmpfs-single/checkpoint --new-name test-tmpfs-single-restored

# Test 2: Multiple tmpfs mounts
docker run -d --name test-tmpfs-multiple \
    --tmpfs /tmp:size=50m \
    --tmpfs /cache:size=50m \
    alpine:latest sh -c 'echo "tmp data" > /tmp/tmp.txt && echo "cache data" > /cache/cache.txt && sleep 300'

sudo docker-cr checkpoint test-tmpfs-multiple --output ./test-checkpoints
docker stop test-tmpfs-multiple && docker rm test-tmpfs-multiple
sudo docker-cr restore --from ./test-checkpoints/test-tmpfs-multiple/checkpoint --new-name test-tmpfs-multiple-restored

# Note: tmpfs data will likely be lost during checkpoint/restore (this is expected behavior)

# Cleanup
docker stop test-tmpfs-single test-tmpfs-multiple test-tmpfs-single-restored test-tmpfs-multiple-restored 2>/dev/null || true
docker rm test-tmpfs-single test-tmpfs-multiple test-tmpfs-single-restored test-tmpfs-multiple-restored 2>/dev/null || true
```

---

### 5. Network Configuration Tests

#### Test 5.1: Port Mapping

**Objective**: Test containers with port mappings.

```bash
# Test 1: Single port mapping
docker run -d --name test-port-single -p 8090:80 nginx:alpine
curl -s http://localhost:8090 > /dev/null && echo "Port 8090 responding before checkpoint"
sudo docker-cr checkpoint test-port-single --output ./test-checkpoints
docker stop test-port-single && docker rm test-port-single
sudo docker-cr restore --from ./test-checkpoints/test-port-single/checkpoint --new-name test-port-single-restored
sleep 2
curl -s http://localhost:8090 > /dev/null && echo "Port 8090 responding after restore" || echo "Port 8090 not responding after restore"

# Test 2: Multiple port mappings
docker run -d --name test-port-multiple -p 8091:80 -p 8092:443 nginx:alpine
curl -s http://localhost:8091 > /dev/null && echo "Port 8091 responding"
sudo docker-cr checkpoint test-port-multiple --output ./test-checkpoints
docker stop test-port-multiple && docker rm test-port-multiple
sudo docker-cr restore --from ./test-checkpoints/test-port-multiple/checkpoint --new-name test-port-multiple-restored

# Test 3: UDP port mapping
docker run -d --name test-port-udp -p 8093:53/udp alpine:latest sh -c 'nc -l -u -p 53 & sleep 300'
sudo docker-cr checkpoint test-port-udp --output ./test-checkpoints
docker stop test-port-udp && docker rm test-port-udp
sudo docker-cr restore --from ./test-checkpoints/test-port-udp/checkpoint --new-name test-port-udp-restored

# Cleanup
docker stop test-port-single test-port-multiple test-port-udp test-port-single-restored test-port-multiple-restored test-port-udp-restored 2>/dev/null || true
docker rm test-port-single test-port-multiple test-port-udp test-port-single-restored test-port-multiple-restored test-port-udp-restored 2>/dev/null || true
```

#### Test 5.2: Custom Networks

**Objective**: Test containers with custom networks.

```bash
# Setup custom network
docker network create test-network

# Test 1: Container in custom network
docker run -d --name test-custom-network --network test-network alpine:latest sleep 300
docker exec test-custom-network ip addr show
sudo docker-cr checkpoint test-custom-network --output ./test-checkpoints
docker stop test-custom-network && docker rm test-custom-network
sudo docker-cr restore --from ./test-checkpoints/test-custom-network/checkpoint --new-name test-custom-network-restored

# Test 2: Container with network alias
docker run -d --name test-network-alias --network test-network --network-alias myapp alpine:latest sleep 300
sudo docker-cr checkpoint test-network-alias --output ./test-checkpoints
docker stop test-network-alias && docker rm test-network-alias
sudo docker-cr restore --from ./test-checkpoints/test-network-alias/checkpoint --new-name test-network-alias-restored

# Cleanup
docker stop test-custom-network test-network-alias test-custom-network-restored test-network-alias-restored 2>/dev/null || true
docker rm test-custom-network test-network-alias test-custom-network-restored test-network-alias-restored 2>/dev/null || true
docker network rm test-network 2>/dev/null || true
```

---

### 6. Long-Running Process Tests

#### Test 6.1: Continuous Processes

**Objective**: Test containers running continuous processes.

```bash
# Test 1: Log generating process
docker run -d --name test-continuous-log alpine:latest sh -c 'while true; do echo "$(date): Still running"; sleep 5; done'
sleep 10
docker logs test-continuous-log | tail -3
sudo docker-cr checkpoint test-continuous-log --output ./test-checkpoints
docker stop test-continuous-log && docker rm test-continuous-log
sudo docker-cr restore --from ./test-checkpoints/test-continuous-log/checkpoint --new-name test-continuous-log-restored
sleep 10
echo "Logs after restore:"
docker logs test-continuous-log-restored | tail -3

# Test 2: Counter process
docker run -d --name test-counter alpine:latest sh -c 'counter=0; while true; do echo "Counter: $counter"; counter=$((counter+1)); sleep 2; done'
sleep 8
docker logs test-counter | tail -2
sudo docker-cr checkpoint test-counter --output ./test-checkpoints
docker stop test-counter && docker rm test-counter
sudo docker-cr restore --from ./test-checkpoints/test-counter/checkpoint --new-name test-counter-restored
sleep 6
echo "Counter after restore:"
docker logs test-counter-restored | tail -2

# Test 3: File writing process
docker run -d --name test-file-writer -v /tmp/file-writer-data:/data alpine:latest sh -c 'counter=0; while true; do echo "Entry $counter: $(date)" >> /data/log.txt; counter=$((counter+1)); sleep 3; done'
mkdir -p /tmp/file-writer-data
sleep 10
tail -3 /tmp/file-writer-data/log.txt
sudo docker-cr checkpoint test-file-writer --output ./test-checkpoints
docker stop test-file-writer && docker rm test-file-writer
sudo docker-cr restore --from ./test-checkpoints/test-file-writer/checkpoint --new-name test-file-writer-restored --auto-fix-mounts=true
sleep 10
echo "File content after restore:"
tail -3 /tmp/file-writer-data/log.txt

# Cleanup
docker stop test-continuous-log test-counter test-file-writer test-continuous-log-restored test-counter-restored test-file-writer-restored 2>/dev/null || true
docker rm test-continuous-log test-counter test-file-writer test-continuous-log-restored test-counter-restored test-file-writer-restored 2>/dev/null || true
rm -rf /tmp/file-writer-data
```

---

### 7. Automated Test Runner Script

Create `test/scripts/test_container_types.sh`:

```bash
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

# Test functions
test_alpine_basic() {
    docker run -d --name test-auto-alpine alpine:latest sleep 60
    sudo docker-cr checkpoint test-auto-alpine --output "$TEST_BASE_DIR"
    docker stop test-auto-alpine && docker rm test-auto-alpine
    sudo docker-cr restore --from "$TEST_BASE_DIR/test-auto-alpine/checkpoint" --new-name test-auto-alpine-restored
    docker exec test-auto-alpine-restored echo "Alpine test successful"
    docker stop test-auto-alpine-restored && docker rm test-auto-alpine-restored
}

test_ubuntu_basic() {
    docker run -d --name test-auto-ubuntu ubuntu:20.04 sleep 60
    sudo docker-cr checkpoint test-auto-ubuntu --output "$TEST_BASE_DIR"
    docker stop test-auto-ubuntu && docker rm test-auto-ubuntu
    sudo docker-cr restore --from "$TEST_BASE_DIR/test-auto-ubuntu/checkpoint" --new-name test-auto-ubuntu-restored
    docker exec test-auto-ubuntu-restored echo "Ubuntu test successful"
    docker stop test-auto-ubuntu-restored && docker rm test-auto-ubuntu-restored
}

test_nginx_webserver() {
    docker run -d --name test-auto-nginx -p 8095:80 nginx:alpine
    sleep 2
    sudo docker-cr checkpoint test-auto-nginx --output "$TEST_BASE_DIR"
    docker stop test-auto-nginx && docker rm test-auto-nginx
    sudo docker-cr restore --from "$TEST_BASE_DIR/test-auto-nginx/checkpoint" --new-name test-auto-nginx-restored
    sleep 2
    curl -s http://localhost:8095 > /dev/null
    docker stop test-auto-nginx-restored && docker rm test-auto-nginx-restored
}

test_bind_mount() {
    mkdir -p /tmp/auto-test-mount
    echo "mount test data" > /tmp/auto-test-mount/test.txt
    docker run -d --name test-auto-mount -v /tmp/auto-test-mount:/data alpine:latest sleep 60
    sudo docker-cr checkpoint test-auto-mount --output "$TEST_BASE_DIR"
    docker stop test-auto-mount && docker rm test-auto-mount
    sudo docker-cr restore --from "$TEST_BASE_DIR/test-auto-mount/checkpoint" --new-name test-auto-mount-restored --auto-fix-mounts=true
    docker exec test-auto-mount-restored cat /data/test.txt | grep -q "mount test data"
    docker stop test-auto-mount-restored && docker rm test-auto-mount-restored
    rm -rf /tmp/auto-test-mount
}

test_environment_vars() {
    docker run -d --name test-auto-env -e TEST_VAR="auto-test-value" alpine:latest sleep 60
    sudo docker-cr checkpoint test-auto-env --output "$TEST_BASE_DIR"
    docker stop test-auto-env && docker rm test-auto-env
    sudo docker-cr restore --from "$TEST_BASE_DIR/test-auto-env/checkpoint" --new-name test-auto-env-restored
    docker exec test-auto-env-restored env | grep -q "TEST_VAR=auto-test-value"
    docker stop test-auto-env-restored && docker rm test-auto-env-restored
}

# Cleanup function
cleanup() {
    print_status "INFO" "Cleaning up test containers and data..."
    docker stop $(docker ps -q --filter "name=test-auto-") 2>/dev/null || true
    docker rm $(docker ps -aq --filter "name=test-auto-") 2>/dev/null || true
    rm -rf "$TEST_BASE_DIR"
}

trap cleanup EXIT

# Run tests
print_status "INFO" "Starting container types test suite..."

run_test "Alpine Linux Basic" test_alpine_basic
run_test "Ubuntu Basic" test_ubuntu_basic
run_test "Nginx Web Server" test_nginx_webserver
run_test "Bind Mount" test_bind_mount
run_test "Environment Variables" test_environment_vars

# Summary
echo
echo "=== Test Results Summary ==="
echo "Tests Run: $TESTS_RUN"
echo "Passed: $TESTS_PASSED"
echo "Failed: $TESTS_FAILED"

if [ $TESTS_FAILED -eq 0 ]; then
    print_status "SUCCESS" "All container type tests passed!"
    exit 0
else
    print_status "ERROR" "Some container type tests failed!"
    exit 1
fi
```

This comprehensive test suite covers various container types, images, and configurations to ensure docker-cr works across different scenarios. Each test validates both checkpoint creation and successful restoration while testing specific features or configurations.