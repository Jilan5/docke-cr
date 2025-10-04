#!/bin/bash
# deploy_docker_cr.sh - Script to setup and run the Docker-CR checkpoint project on EC2

set -e

echo "=== EC2 Docker-CR Checkpoint Setup Script ==="
echo

# Function to check if a command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Install Docker if not present
install_docker() {
    if ! command_exists docker; then
        echo "Installing Docker..."
        sudo apt-get update
        sudo apt-get install -y \
            ca-certificates \
            curl \
            gnupg \
            lsb-release

        curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor -o /usr/share/keyrings/docker-archive-keyring.gpg

        echo \
          "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/docker-archive-keyring.gpg] https://download.docker.com/linux/ubuntu \
          $(lsb_release -cs) stable" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null

        sudo apt-get update
        sudo apt-get install -y docker-ce docker-ce-cli containerd.io
        sudo usermod -aG docker $USER
        echo "Docker installed successfully!"
        echo "Note: Please logout and login again for docker group changes to take effect"
    else
        echo "Docker is already installed"
        echo "Ensuring user is in docker group..."
        sudo usermod -aG docker $USER
        echo "Note: Please logout and login again for docker group changes to take effect"
    fi
}

# Install CRIU
install_criu() {
    if ! command_exists criu; then
        echo "Installing CRIU..."
        sudo apt-get update
        sudo apt-get install -y criu

        # Set capabilities for CRIU
        sudo setcap cap_sys_admin,cap_sys_ptrace,cap_sys_chroot+ep $(which criu)

        # Verify installation
        criu --version
        echo "CRIU installed successfully!"
    else
        echo "CRIU is already installed"
        criu --version
    fi
}

# Install Go if not present
install_go() {
    if ! command_exists go; then
        echo "Installing Go..."
        GO_VERSION="1.21.5"
        wget -q "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz"
        sudo rm -rf /usr/local/go
        sudo tar -C /usr/local -xzf "go${GO_VERSION}.linux-amd64.tar.gz"
        rm "go${GO_VERSION}.linux-amd64.tar.gz"

        # Add Go to PATH
        export PATH=$PATH:/usr/local/go/bin
        echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
        export PATH=$PATH:/usr/local/go/bin

        go version
        echo "Go installed successfully!"
    else
        echo "Go is already installed"
        go version
    fi
}

# Install development tools
install_dev_tools() {
    echo "Installing development tools..."
    sudo apt-get install -y \
        make \
        gcc \
        libc6-dev \
        git \
        vim \
        htop \
        tree
    echo "Development tools installed!"
}

# Enable Docker experimental features for checkpoint support
enable_docker_experimental() {
    echo "Enabling Docker experimental features..."
    sudo mkdir -p /etc/docker

    # Create or update Docker daemon configuration
    sudo tee /etc/docker/daemon.json > /dev/null <<EOF
{
    "experimental": true,
    "live-restore": true,
    "storage-driver": "overlay2"
}
EOF

    # Restart Docker to apply changes
    sudo systemctl restart docker
    echo "Docker experimental features enabled!"
}

# Setup Go environment
setup_go_environment() {
    echo "Setting up Go environment..."

    # Set Go environment variables
    export GOPATH=$HOME/go
    export PATH=$PATH:/usr/local/go/bin:$GOPATH/bin

    # Add to bashrc if not already there
    if ! grep -q "GOPATH" ~/.bashrc; then
        echo 'export GOPATH=$HOME/go' >> ~/.bashrc
        echo 'export PATH=$PATH:/usr/local/go/bin:$GOPATH/bin' >> ~/.bashrc
    fi

    # Create GOPATH directory
    mkdir -p $GOPATH/{bin,src,pkg}

    echo "Go environment setup complete!"
}

# Build the docker-cr application
build_application() {
    echo "Building the Docker-CR checkpoint application..."

    # Get current directory (should be docker-cr project root)
    PROJECT_DIR=$(pwd)
    echo "Working in directory: $PROJECT_DIR"

    # Verify we're in the right directory
    if [[ ! -f "go.mod" ]] || [[ ! -f "Makefile" ]]; then
        echo "Error: This doesn't appear to be the docker-cr project directory"
        echo "Please run this script from the docker-cr project root"
        exit 1
    fi

    # Download dependencies and create go.sum
    echo "Downloading Go dependencies..."
    go mod tidy
    go mod download

    # Check dependencies
    echo "Checking CRIU support..."
    make check-deps || echo "Warning: Some dependencies may not be available"

    # Build the application using Makefile
    echo "Building docker-cr binary..."
    make build

    # Verify binary was created
    if [[ -f "bin/docker-cr" ]]; then
        echo "Application built successfully!"
        echo "Binary location: $(pwd)/bin/docker-cr"

        # Make sure it's executable
        chmod +x bin/docker-cr

        # Show version
        ./bin/docker-cr version
    else
        echo "Error: Build failed - binary not found"
        exit 1
    fi
}

# Install the application system-wide
install_application() {
    echo "Installing docker-cr to system..."

    if [[ -f "bin/docker-cr" ]]; then
        sudo cp bin/docker-cr /usr/local/bin/
        sudo chmod +x /usr/local/bin/docker-cr
        echo "docker-cr installed to /usr/local/bin/"

        # Verify installation
        docker-cr version
    else
        echo "Error: Binary not found. Please build first."
        exit 1
    fi
}

# Create test environment
setup_test_environment() {
    echo "Setting up test environment..."

    # Create test directories
    mkdir -p ~/docker-cr-tests/{checkpoints,logs}

    # Create a simple test script
    cat > ~/docker-cr-tests/test_basic.sh << 'EOF'
#!/bin/bash
# Basic test script for docker-cr

set -e

echo "=== Docker-CR Basic Test ==="

# Test container name
TEST_CONTAINER="docker-cr-test-$(date +%s)"
CHECKPOINT_DIR="./checkpoints"

echo "1. Creating test container: $TEST_CONTAINER"
docker run -d --name "$TEST_CONTAINER" alpine:latest sleep 300

echo "2. Waiting for container to be ready..."
sleep 2

echo "3. Checkpointing container..."
sudo docker-cr checkpoint "$TEST_CONTAINER" --output "$CHECKPOINT_DIR" --name test-checkpoint

echo "4. Inspecting checkpoint..."
docker-cr inspect "$CHECKPOINT_DIR/$TEST_CONTAINER/test-checkpoint" --summary

echo "5. Stopping original container..."
docker stop "$TEST_CONTAINER" && docker rm "$TEST_CONTAINER"

echo "6. Restoring container..."
sudo docker-cr restore --from "$CHECKPOINT_DIR/$TEST_CONTAINER/test-checkpoint" --new-name "$TEST_CONTAINER-restored"

echo "7. Verifying restored container..."
docker ps | grep "$TEST_CONTAINER-restored" || echo "Container not running"

echo "8. Cleaning up..."
docker stop "$TEST_CONTAINER-restored" && docker rm "$TEST_CONTAINER-restored" || true
rm -rf "$CHECKPOINT_DIR"

echo "=== Test Complete ==="
EOF

    chmod +x ~/docker-cr-tests/test_basic.sh

    echo "Test environment created in ~/docker-cr-tests/"
}

# Run initial tests
run_initial_tests() {
    echo "Running initial tests..."

    # Test 1: Version check
    echo "Testing version command..."
    ./bin/docker-cr version

    # Test 2: Help command
    echo "Testing help command..."
    ./bin/docker-cr --help

    # Test 3: CRIU check
    echo "Testing CRIU support..."
    make check-deps || echo "Warning: CRIU check failed"

    echo "Initial tests completed!"
}

# Main execution
main() {
    echo "Starting EC2 setup for Docker-CR checkpoint project..."
    echo "This script will install Docker, CRIU, Go, and build the docker-cr tool"
    echo

    # Update system packages
    echo "Updating system packages..."
    sudo apt-get update

    # Install prerequisites
    sudo apt-get install -y wget curl git build-essential

    # Install required software
    install_docker
    install_criu
    install_go
    install_dev_tools
    setup_go_environment
    enable_docker_experimental

    # Build and install the application
    echo
    echo "Building and installing the docker-cr application..."
    build_application

    # Ask user if they want to install system-wide
    echo
    read -p "Do you want to install docker-cr system-wide? (y/n): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        install_application
    fi

    # Setup test environment
    setup_test_environment

    # Run initial tests
    run_initial_tests

    echo
    echo "=== Setup Complete ==="
    echo
    echo "IMPORTANT: Please logout and login again (or run 'newgrp docker') to activate Docker group membership"
    echo
    echo "Docker-CR tool is ready to use!"
    echo
    echo "Local binary: $(pwd)/bin/docker-cr"
    if command_exists docker-cr; then
        echo "System binary: /usr/local/bin/docker-cr"
    fi
    echo
    echo "Usage examples:"
    echo "  # Checkpoint a container"
    echo "  sudo docker-cr checkpoint <container-name> --output ./checkpoints"
    echo
    echo "  # Restore from checkpoint"
    echo "  sudo docker-cr restore --from ./checkpoints/<container>/<checkpoint> --new-name restored"
    echo
    echo "  # Inspect checkpoint"
    echo "  docker-cr inspect ./checkpoints/<container>/<checkpoint> --all"
    echo
    echo "Available make targets:"
    echo "  make build          # Build the binary"
    echo "  make test           # Run tests"
    echo "  make demo           # Run complete demo"
    echo "  make quick-test     # Quick functionality test"
    echo "  make help           # Show all available targets"
    echo
    echo "Test scripts:"
    echo "  ~/docker-cr-tests/test_basic.sh    # Basic functionality test"
    echo
    echo "Quick start test:"
    echo "  cd ~/docker-cr-tests && ./test_basic.sh"
    echo
}

# Run main function
main