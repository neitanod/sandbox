package main

// SecurityFlags returns Docker CLI flags for the given security config
func SecurityFlags(sec SecurityConfig) []string {
	var flags []string

	if sec.NoRoot {
		flags = append(flags, "--user", "1000:1000")
	}

	if sec.DropCaps {
		flags = append(flags, "--cap-drop=ALL")
	}

	if sec.ReadOnlyRootfs {
		flags = append(flags, "--read-only")
		flags = append(flags, "--tmpfs", "/tmp:rw,noexec,nosuid")
	}

	// Docker applies seccomp by default. When seccomp_default is true,
	// we make it explicit. When false, we don't change anything (Docker
	// still applies its default profile unless someone overrides it).
	if sec.SeccompDefault {
		flags = append(flags, "--security-opt", "seccomp=default")
	}

	return flags
}
