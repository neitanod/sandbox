package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

func main() {
	args := os.Args[1:]

	if len(args) == 0 {
		printUsage()
		os.Exit(1)
	}

	switch args[0] {
	case "build":
		if len(args) < 2 {
			fatal("usage: sandbox build <name> [--exit] [--image <img>] [--config <path>] [--description \"text\"]")
		}
		cmdBuild(args[1], args[2:], false)
	case "rebuild":
		if len(args) < 2 {
			fatal("usage: sandbox rebuild <name> [--exit] [--image <img>] [--config <path>] [--description \"text\"]")
		}
		cmdBuild(args[1], args[2:], true)
	case "list":
		cmdList()
	case "stop":
		if len(args) < 2 {
			fatal("usage: sandbox stop <name>")
		}
		cmdStop(args[1])
	case "rm":
		cmdRm(args[1:])
	case "config":
		if len(args) < 2 {
			fatal("usage: sandbox config <name> [<key>] [<value>]")
		}
		cmdConfig(args[1:])
	case "edit":
		if len(args) < 2 {
			fatal("usage: sandbox edit <name>")
		}
		cmdEdit(args[1])
	case "help", "--help", "-h":
		printUsage()
	default:
		// Default: sandbox <name> [--ephemeral] [--config <path>] [-- cmd...]
		cmdRun(args)
	}
}

// parseBuildFlags parses flags common to build and rebuild subcommands
type buildFlags struct {
	exitAfter   bool
	configPath  string
	description string
	image       string
}

func parseBuildFlags(args []string) buildFlags {
	var f buildFlags
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--exit":
			f.exitAfter = true
		case "--config":
			if i+1 >= len(args) {
				fatal("--config requires a path")
			}
			i++
			f.configPath = args[i]
		case "--description":
			if i+1 >= len(args) {
				fatal("--description requires a value")
			}
			i++
			f.description = args[i]
		case "--image":
			if i+1 >= len(args) {
				fatal("--image requires a value")
			}
			i++
			f.image = args[i]
		default:
			fatal("unknown flag: %s", args[i])
		}
	}
	return f
}

func cmdBuild(name string, args []string, rebuild bool) {
	f := parseBuildFlags(args)

	cfg, err := LoadConfig(name, f.configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// Create a minimal config with pwd mounted at /workspace
			cfg = NewDefaultConfig()
			pwd, pwdErr := os.Getwd()
			if pwdErr != nil {
				fatal("cannot get working directory: %v", pwdErr)
			}
			cfg.Mounts = []MountConfig{{Host: pwd, Container: "/workspace"}}
			cfg.Workdir = "/workspace"
			if f.image != "" {
				cfg.Image = f.image
			}
			if f.description != "" {
				cfg.Description = f.description
			}
			if saveErr := SaveConfig(name, cfg); saveErr != nil {
				fatal("cannot create config for %q: %v", name, saveErr)
			}
			fmt.Printf("created minimal config at %s (mounting %s -> /workspace)\n", DefaultConfigPath(name), pwd)
		} else {
			fatal("cannot load config for %q: %v", name, err)
		}
	}

	// CLI overrides (for existing configs)
	if f.image != "" {
		cfg.Image = f.image
	}
	if f.description != "" {
		cfg.Description = f.description
	}

	containerName := ContainerName(name)

	if rebuild {
		doRebuild(name, containerName, cfg)
	} else {
		doBuild(name, containerName, cfg)
	}

	if !f.exitAfter {
		doRun(containerName, cfg, nil)
	}
}

func cmdRun(args []string) {
	name := args[0]
	rest := args[1:]

	var (
		ephemeral  bool
		configPath string
		userCmd    []string
	)

	for i := 0; i < len(rest); i++ {
		switch rest[i] {
		case "--ephemeral":
			ephemeral = true
		case "--config":
			if i+1 >= len(rest) {
				fatal("--config requires a path")
			}
			i++
			configPath = rest[i]
		case "--":
			userCmd = rest[i+1:]
			i = len(rest) // break loop
		default:
			fatal("unknown flag: %s", rest[i])
		}
	}

	cfg, err := LoadConfig(name, configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if ephemeral {
				cfg = NewDefaultConfig()
			} else {
				fatal("config not found for %q. Create it with: sandbox build %s", name, name)
			}
		} else {
			fatal("cannot load config for %q: %v", name, err)
		}
	}

	containerName := ContainerName(name)

	if ephemeral {
		doEphemeral(name, containerName, cfg, userCmd)
		return
	}

	// Default: run/exec
	doExec(containerName, cfg, userCmd)
}

func doBuild(name, containerName string, cfg *SandboxConfig) {
	if containerExists(containerName) {
		fatal("container %q already exists, use 'sandbox rebuild %s' to recreate", containerName, name)
	}

	imageName := buildImageIfNeeded(name, cfg)
	createContainer(containerName, imageName, cfg)
	fmt.Printf("sandbox %q created successfully\n", name)
}

func doRebuild(name, containerName string, cfg *SandboxConfig) {
	// Remove existing container
	if containerExists(containerName) {
		dockerRmForce(containerName)
	}

	// Remove custom image if exists
	customImage := CustomImageName(name)
	if imageExists(customImage) {
		dockerRmi(customImage)
	}

	imageName := buildImageIfNeeded(name, cfg)
	createContainer(containerName, imageName, cfg)
	fmt.Printf("sandbox %q rebuilt successfully\n", name)
}

func doEphemeral(name, containerName string, cfg *SandboxConfig, userCmd []string) {
	if cfg == nil {
		fatal("cannot run ephemeral without config")
	}
	runEphemeral(name, cfg, userCmd)
}

func doRun(containerName string, cfg *SandboxConfig, userCmd []string) {
	if !containerExists(containerName) {
		return // container just created, not started yet — start + exec
	}
	doExec(containerName, cfg, userCmd)
}

func doExec(containerName string, cfg *SandboxConfig, userCmd []string) {
	if !containerExists(containerName) {
		name := SandboxNameFromContainer(containerName)
		fatal("container %q does not exist, use 'sandbox build %s' to create it", containerName, name)
	}

	state := containerState(containerName)
	if state != "running" {
		dockerStart(containerName)
	}

	if len(userCmd) > 0 {
		dockerExec(containerName, cfg, userCmd)
	} else {
		dockerExecInteractive(containerName, cfg)
	}
}

func cmdStop(name string) {
	containerName := ContainerName(name)
	if !containerExists(containerName) {
		fatal("container %q does not exist", containerName)
	}
	state := containerState(containerName)
	if state != "running" {
		fatal("container %q is not running (state: %s)", containerName, state)
	}
	dockerStop(containerName)
	fmt.Printf("sandbox %q stopped\n", name)
}

func cmdRm(args []string) {
	if len(args) == 0 {
		fatal("usage: sandbox rm <name> [--forget [-y]] | sandbox rm --orphans [-y]")
	}

	// Check for --orphans
	if args[0] == "--orphans" {
		skipConfirm := len(args) > 1 && args[1] == "-y"
		removeOrphans(skipConfirm)
		return
	}

	name := args[0]
	rest := args[1:]

	var forget, skipConfirm bool
	for _, a := range rest {
		switch a {
		case "--forget":
			forget = true
		case "-y":
			skipConfirm = true
		default:
			fatal("unknown flag: %s", a)
		}
	}

	containerName := ContainerName(name)

	if forget {
		if !skipConfirm {
			fmt.Print("You are about to destroy a container and also its configuration file, you'll lose it forever.\nAre you sure? (use -y to skip this confirmation) y/N: ")
			var answer string
			fmt.Scanln(&answer)
			if strings.ToLower(answer) != "y" {
				fmt.Println("Cancelled.")
				return
			}
		}
	}

	// Remove container
	if containerExists(containerName) {
		dockerRmForce(containerName)
		fmt.Printf("container %q removed\n", containerName)
	} else {
		fmt.Printf("container %q does not exist (skipping)\n", containerName)
	}

	// Remove custom image
	customImage := CustomImageName(name)
	if imageExists(customImage) {
		dockerRmi(customImage)
		fmt.Printf("image %q removed\n", customImage)
	}

	// Remove config file if --forget
	if forget {
		configFile := DefaultConfigPath(name)
		if err := os.Remove(configFile); err != nil && !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "warning: could not remove config %s: %v\n", configFile, err)
		} else if err == nil {
			fmt.Printf("config %q removed\n", configFile)
		}
	}

	fmt.Printf("sandbox %q removed\n", name)
}

func printUsage() {
	fmt.Println(`sandbox - Docker container manager via JSON configs

Usage:
  sandbox <name> [--ephemeral] [--config <path>] [-- <cmd>]
  sandbox build <name> [--exit] [--image <img>] [--config <path>] [--description "text"]
  sandbox rebuild <name> [--exit] [--image <img>] [--config <path>] [--description "text"]
  sandbox list
  sandbox stop <name>
  sandbox rm <name> [--forget [-y]]
  sandbox rm --orphans [-y]
  sandbox config <name> [<key>] [<value>]
  sandbox edit <name>

Subcommands:
  build <name>    Create a new container (fails if already exists)
  rebuild <name>  Destroy and recreate from scratch
  list            List all configured sandboxes and their status
  stop <name>     Stop a running container
  rm <name>       Remove container and custom image
  config <name>   View or modify JSON configuration
  edit <name>     Open JSON config in $EDITOR

Flags (build/rebuild):
  --exit          Don't enter the container after building (for scripts)
  --image <img>   Use a specific base image (overrides JSON)
  --description   Set description (overrides JSON)
  --config <path> Use a specific JSON config file

Flags (run):
  --ephemeral     Run a disposable container (--rm)
  --config <path> Use a specific JSON config file`)
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "sandbox: "+format+"\n", args...)
	os.Exit(1)
}
