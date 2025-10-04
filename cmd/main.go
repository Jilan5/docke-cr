package main

import (
	"os"

	"docker-cr/pkg/checkpoint"
	"docker-cr/pkg/docker"
	"docker-cr/pkg/inspect"
	"docker-cr/pkg/restore"
	"docker-cr/pkg/utils"
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	logger    *logrus.Logger
	logLevel  string
	verbose   bool
)

func main() {
	// Initialize logger
	logger = logrus.New()
	logger.SetOutput(os.Stdout)

	var rootCmd = &cobra.Command{
		Use:   "docker-cr",
		Short: "Docker Checkpoint/Restore Tool",
		Long: `A simple Docker checkpoint and restore tool using Go-CRIU.
Supports checkpointing running containers and restoring them with proper mount namespace handling.`,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			setupLogging()
		},
	}

	// Global flags
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "Log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")

	// Add commands
	rootCmd.AddCommand(newCheckpointCommand())
	rootCmd.AddCommand(newRestoreCommand())
	rootCmd.AddCommand(newInspectCommand())
	rootCmd.AddCommand(newVersionCommand())

	if err := rootCmd.Execute(); err != nil {
		logger.Fatal(err)
	}
}

func setupLogging() {
	switch logLevel {
	case "debug":
		logger.SetLevel(logrus.DebugLevel)
	case "info":
		logger.SetLevel(logrus.InfoLevel)
	case "warn":
		logger.SetLevel(logrus.WarnLevel)
	case "error":
		logger.SetLevel(logrus.ErrorLevel)
	default:
		logger.SetLevel(logrus.InfoLevel)
	}

	if verbose {
		logger.SetLevel(logrus.DebugLevel)
	}

	// Set formatter
	logger.SetFormatter(&logrus.TextFormatter{
		DisableColors: false,
		FullTimestamp: true,
	})
}

func newCheckpointCommand() *cobra.Command {
	var (
		outputDir      string
		checkpointName string
		leaveRunning   bool
		tcpEstablished bool
		fileLocks      bool
		preDump        bool
		manageCgroups  bool
		shell          bool
	)

	cmd := &cobra.Command{
		Use:   "checkpoint <container-name>",
		Short: "Checkpoint a running container",
		Long:  `Create a checkpoint of a running Docker container using CRIU.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			containerName := args[0]

			// Initialize managers
			dockerManager, err := docker.NewManager(logger)
			if err != nil {
				return fmt.Errorf("failed to initialize Docker manager: %w", err)
			}
			defer dockerManager.Close()

			checkpointManager := checkpoint.NewManager(dockerManager, logger)

			// Check CRIU support
			if err := checkpointManager.CheckCRIUSupport(); err != nil {
				return fmt.Errorf("CRIU support check failed: %w", err)
			}

			// Prepare checkpoint config
			config := checkpoint.CheckpointConfig{
				OutputDir:       outputDir,
				CheckpointName:  checkpointName,
				LeaveRunning:    leaveRunning,
				TcpEstablished:  tcpEstablished,
				FileLocks:       fileLocks,
				PreDump:         preDump,
				LogLevel:        4, // Debug level
				ManageCgroups:   manageCgroups,
				Shell:           shell,
			}

			// Perform checkpoint
			logger.Infof("Starting checkpoint of container: %s", containerName)
			if err := checkpointManager.Checkpoint(containerName, config); err != nil {
				return fmt.Errorf("checkpoint failed: %w", err)
			}

			// Show checkpoint files
			checkpointDir := fmt.Sprintf("%s/%s/%s", outputDir, containerName, checkpointName)
			files, err := checkpointManager.ListCheckpointFiles(checkpointDir)
			if err == nil {
				fmt.Printf("\nCheckpoint files created:\n")
				for _, file := range files {
					fmt.Printf("  %s\n", file)
				}
			}

			fmt.Printf("\nCheckpoint completed successfully!\n")
			fmt.Printf("Location: %s\n", checkpointDir)

			return nil
		},
	}

	cmd.Flags().StringVarP(&outputDir, "output", "o", "/tmp/docker-checkpoints", "Output directory for checkpoints")
	cmd.Flags().StringVarP(&checkpointName, "name", "n", "checkpoint", "Name for the checkpoint")
	cmd.Flags().BoolVar(&leaveRunning, "leave-running", true, "Leave container running after checkpoint")
	cmd.Flags().BoolVar(&tcpEstablished, "tcp", true, "Checkpoint established TCP connections")
	cmd.Flags().BoolVar(&fileLocks, "file-locks", true, "Checkpoint file locks")
	cmd.Flags().BoolVar(&preDump, "pre-dump", false, "Perform pre-dump for optimization")
	cmd.Flags().BoolVar(&manageCgroups, "manage-cgroups", true, "Manage cgroups during checkpoint")
	cmd.Flags().BoolVar(&shell, "shell", true, "Checkpoint as shell job")

	return cmd
}

func newRestoreCommand() *cobra.Command {
	var (
		checkpointDir    string
		archivePath      string
		newContainerName string
		manageCgroups    bool
		tcpEstablished   bool
		restoreSibling   bool
		shell            bool
		validateEnv      bool
		autoFixMounts    bool
		skipMounts       []string
	)

	cmd := &cobra.Command{
		Use:   "restore",
		Short: "Restore a container from checkpoint",
		Long:  `Restore a Docker container from a previously created checkpoint.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Initialize managers
			dockerManager, err := docker.NewManager(logger)
			if err != nil {
				return fmt.Errorf("failed to initialize Docker manager: %w", err)
			}
			defer dockerManager.Close()

			checkpointManager := checkpoint.NewManager(dockerManager, logger)
			restoreManager := restore.NewManager(dockerManager, checkpointManager, logger)

			var restoreConfig restore.RestoreConfig

			if archivePath != "" {
				// Restore from archive
				if newContainerName == "" {
					return fmt.Errorf("--new-name is required when restoring from archive")
				}

				restoreConfig = restore.RestoreConfig{
					CheckpointDir:    archivePath, // Will be handled as archive
					NewContainerName: newContainerName,
					LogLevel:         4,
					ManageCgroups:    manageCgroups,
					TcpEstablished:   tcpEstablished,
					RestoreSibling:   restoreSibling,
					Shell:            shell,
					ValidateEnv:      validateEnv,
					AutoFixMounts:    autoFixMounts,
					SkipMounts:       skipMounts,
				}

				return restoreManager.RestoreFromArchive(archivePath, newContainerName, restoreConfig)
			}

			if checkpointDir == "" {
				return fmt.Errorf("either --from or --archive must be specified")
			}

			// Get default restore config if not provided
			if newContainerName == "" {
				defaultConfig, err := restoreManager.GetRestoreOptions(checkpointDir)
				if err != nil {
					return fmt.Errorf("failed to get default restore options: %w", err)
				}
				newContainerName = defaultConfig.NewContainerName
			}

			restoreConfig = restore.RestoreConfig{
				CheckpointDir:    checkpointDir,
				NewContainerName: newContainerName,
				LogLevel:         4,
				ManageCgroups:    manageCgroups,
				TcpEstablished:   tcpEstablished,
				RestoreSibling:   restoreSibling,
				Shell:            shell,
				ValidateEnv:      validateEnv,
				AutoFixMounts:    autoFixMounts,
				SkipMounts:       skipMounts,
			}

			// Perform restore
			logger.Infof("Starting restore from: %s", checkpointDir)
			if err := restoreManager.Restore(restoreConfig); err != nil {
				return fmt.Errorf("restore failed: %w", err)
			}

			fmt.Printf("\nRestore completed successfully!\n")
			fmt.Printf("New container name: %s\n", newContainerName)

			return nil
		},
	}

	cmd.Flags().StringVar(&checkpointDir, "from", "", "Checkpoint directory to restore from")
	cmd.Flags().StringVar(&archivePath, "archive", "", "Checkpoint archive to restore from")
	cmd.Flags().StringVar(&newContainerName, "new-name", "", "Name for the restored container")
	cmd.Flags().BoolVar(&manageCgroups, "manage-cgroups", false, "Manage cgroups during restore")
	cmd.Flags().BoolVar(&tcpEstablished, "tcp", false, "Restore established TCP connections")
	cmd.Flags().BoolVar(&restoreSibling, "restore-sibling", false, "Restore as sibling process")
	cmd.Flags().BoolVar(&shell, "shell", true, "Restore as shell job")
	cmd.Flags().BoolVar(&validateEnv, "validate-env", true, "Validate restore environment")
	cmd.Flags().BoolVar(&autoFixMounts, "auto-fix-mounts", true, "Automatically create missing mount sources")
	cmd.Flags().StringSliceVar(&skipMounts, "skip-mounts", []string{}, "Mount paths to skip during restore")

	return cmd
}

func newInspectCommand() *cobra.Command {
	var (
		outputFormat    string
		showProcessTree bool
		showEnvironment bool
		showFiles       bool
		showSockets     bool
		showMounts      bool
		showAll         bool
		summary         bool
	)

	cmd := &cobra.Command{
		Use:   "inspect <checkpoint-dir>",
		Short: "Inspect a checkpoint",
		Long:  `Analyze and display information about a checkpoint (like checkpointctl).`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			checkpointDir := args[0]

			if !utils.DirExists(checkpointDir) {
				return fmt.Errorf("checkpoint directory does not exist: %s", checkpointDir)
			}

			viewer := inspect.NewViewer(logger)

			if summary {
				output, err := viewer.GetSummary(checkpointDir)
				if err != nil {
					return fmt.Errorf("failed to get checkpoint summary: %w", err)
				}
				fmt.Print(output)
				return nil
			}

			options := inspect.ViewOptions{
				ShowProcessTree: showProcessTree,
				ShowEnvironment: showEnvironment,
				ShowFiles:       showFiles,
				ShowSockets:     showSockets,
				ShowMounts:      showMounts,
				ShowAll:         showAll,
				OutputFormat:    outputFormat,
				Verbose:         verbose,
			}

			output, err := viewer.ShowCheckpoint(checkpointDir, options)
			if err != nil {
				return fmt.Errorf("failed to inspect checkpoint: %w", err)
			}

			fmt.Print(output)
			return nil
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "format", "f", "text", "Output format (text, json, tree)")
	cmd.Flags().BoolVar(&showProcessTree, "ps-tree", false, "Show process tree")
	cmd.Flags().BoolVar(&showEnvironment, "env", false, "Show environment variables")
	cmd.Flags().BoolVar(&showFiles, "files", false, "Show file descriptors")
	cmd.Flags().BoolVar(&showSockets, "sockets", false, "Show socket information")
	cmd.Flags().BoolVar(&showMounts, "mounts", false, "Show mount mappings")
	cmd.Flags().BoolVar(&showAll, "all", false, "Show all information")
	cmd.Flags().BoolVar(&summary, "summary", false, "Show brief summary")

	return cmd
}

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("docker-cr version 1.0.0")
			fmt.Println("A simple Docker checkpoint/restore tool using Go-CRIU")
			fmt.Println("Built with love for container migration and forensic analysis")
		},
	}
}