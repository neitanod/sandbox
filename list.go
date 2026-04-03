package main

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
)

func cmdList() {
	// Get configs from filesystem
	configNames, err := ListConfigNames()
	if err != nil {
		fatal("cannot list configs: %v", err)
	}

	// Get managed containers from Docker
	containers, err := ListManagedContainers()
	if err != nil {
		fatal("cannot list containers: %v", err)
	}

	// Build a map of container info by sandbox name
	containerMap := make(map[string]ManagedContainerInfo)
	for _, c := range containers {
		containerMap[c.SandboxName] = c
	}

	// Track which container names we've already printed (to find orphans)
	printed := make(map[string]bool)

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tIMAGE\tSTATUS\tDESCRIPTION")

	// First: configs (with or without container)
	for _, name := range configNames {
		printed[name] = true

		if c, ok := containerMap[name]; ok {
			// Config + container exist
			state := normalizeState(c.State)
			desc := c.Description
			if desc == "" {
				// Fallback to JSON description
				cfg, err := LoadConfig(name, "")
				if err == nil {
					desc = cfg.Description
				}
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", name, c.Image, state, desc)
		} else {
			// Config exists, no container
			cfg, err := LoadConfig(name, "")
			image := "?"
			desc := ""
			if err == nil {
				image = cfg.Image
				desc = cfg.Description
			}
			fmt.Fprintf(w, "%s\t%s\tnot created\t%s\n", name, image, desc)
		}
	}

	// Second: orphaned containers (no config)
	for _, c := range containers {
		if printed[c.SandboxName] {
			continue
		}
		desc := c.Description
		if desc == "" {
			desc = "(no config file)"
		}
		fmt.Fprintf(w, "%s\t%s\torphaned\t%s\n", c.SandboxName, c.Image, desc)
	}

	w.Flush()
}

// removeOrphans finds and removes managed containers without a config file
func removeOrphans(skipConfirm bool) {
	configNames, err := ListConfigNames()
	if err != nil {
		fatal("cannot list configs: %v", err)
	}

	containers, err := ListManagedContainers()
	if err != nil {
		fatal("cannot list containers: %v", err)
	}

	configSet := make(map[string]bool)
	for _, name := range configNames {
		configSet[name] = true
	}

	var orphans []ManagedContainerInfo
	for _, c := range containers {
		if !configSet[c.SandboxName] {
			orphans = append(orphans, c)
		}
	}

	if len(orphans) == 0 {
		fmt.Println("No orphaned containers found.")
		return
	}

	fmt.Printf("Found %d orphaned container(s):\n", len(orphans))
	for _, o := range orphans {
		desc := o.Description
		if desc != "" {
			desc = " (" + desc + ")"
		}
		fmt.Printf("  - %s [%s]%s\n", o.ContainerName, o.State, desc)
	}

	if !skipConfirm {
		fmt.Print("\nRemove all orphaned containers? (use -y to skip this confirmation) y/N: ")
		var answer string
		fmt.Scanln(&answer)
		if strings.ToLower(answer) != "y" {
			fmt.Println("Cancelled.")
			return
		}
	}

	for _, o := range orphans {
		dockerRmForce(o.ContainerName)
		fmt.Printf("removed %s\n", o.ContainerName)

		// Also try to remove custom image
		customImage := CustomImageName(o.SandboxName)
		if imageExists(customImage) {
			dockerRmi(customImage)
		}
	}

	fmt.Printf("%d orphaned container(s) removed.\n", len(orphans))
}

func normalizeState(state string) string {
	switch {
	case strings.Contains(state, "running"):
		return "running"
	case strings.Contains(state, "exited"):
		return "stopped"
	case strings.Contains(state, "created"):
		return "created"
	default:
		return state
	}
}
