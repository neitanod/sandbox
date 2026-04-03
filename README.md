# sandbox

Docker container manager via JSON configs. Run untrusted code in isolated containers without maintaining separate Dockerfiles.

## Install

```bash
go build -o sandbox .
cp sandbox ~/.local/bin/
```

Requires Docker to be installed and running.

## Quick Start

```bash
# cd into a project you want to isolate
cd ~/repos/sketchy-repo

# Create and enter a sandbox (mounts current dir to /workspace)
sandbox myproject --build --description "Sketchy npm package from Reddit"

# Or create and enter, using a specific base image
sandbox myproject --build --image node:20 --description "Sketchy npm package from Reddit"

# Once created, enter the container anytime
sandbox myproject

# Or run a command inside without entering
sandbox myproject -- npm install

# Shell commands work too
sandbox myproject -- "cd src && ls -la"

# List all sandboxes
sandbox list
    NAME       IMAGE    STATUS   DESCRIPTION
    myproject  node:25  running  Sketchy npm package from Reddit

# Stop, remove, clean up
sandbox stop myproject
sandbox rm myproject            # Config file is kept, you can rebuild later with the same settings
sandbox rm myproject --forget   # Config file is also destroyed
```

### Customizing with setup commands

You can install packages inside the container by adding setup commands to the config. These run during `docker build` and become part of the container image.

```bash
# Add zsh to an existing sandbox
sandbox config myproject setup + "apk add --no-cache zsh"

# Rebuild to apply (generates a Dockerfile and builds it)
sandbox myproject --rebuild

# Or set it up before the first build
sandbox config myproject setup + "apk add --no-cache zsh curl git"
sandbox config myproject mounts + ~/repos/sketchy-repo:/workspace
sandbox myproject --build
```

## Usage

```
sandbox <name> [--build|--rebuild] [--ephemeral] [--exit] [--image <img>] [--config <path>] [--description "text"] [-- <cmd>]
sandbox list
sandbox stop <name>
sandbox rm <name> [--forget [-y]]
sandbox rm --orphans [-y]
sandbox config <name> [<key>] [<value>]
```

### Lifecycle

| Command | What it does |
|---------|-------------|
| `sandbox foo --build` | Create container (fails if exists). Enters shell after. |
| `sandbox foo --rebuild` | Destroy and recreate from scratch. Enters shell after. |
| `sandbox foo --build --exit` | Create without entering (for scripts). |
| `sandbox foo` | Start and enter existing container. |
| `sandbox foo -- <cmd>` | Run a command in the container. |
| `sandbox foo --ephemeral` | Disposable session (`docker run --rm`). |
| `sandbox stop foo` | Stop a running container. |
| `sandbox rm foo` | Remove container + custom image. Keep config. |
| `sandbox rm foo --forget` | Remove everything including config (asks confirmation). |
| `sandbox rm --orphans` | Clean up containers whose config was deleted. |

### Configuration

Configs live in `~/.config/sandbox/<name>.json`. Created automatically on first `--build` (mounts pwd to `/workspace`).

Manage from CLI:

```bash
sandbox config myapp                                # Show full config
sandbox config myapp image "node:20"                # Set image
sandbox config myapp mounts + /path:/workspace      # Add mount
sandbox config myapp ports + 3000:3000              # Add port
sandbox config myapp setup + "apk add git"          # Add setup command
sandbox config myapp security.drop_caps true        # Enable security option
sandbox config myapp mounts - /path:/workspace      # Remove mount
```

### JSON Structure

```json
{
  "description": "Sketchy npm package from Reddit",
  "image": "alpine:latest",
  "setup": [
    "apk add --no-cache nodejs npm"
  ],
  "mounts": [
    { "host": "/home/user/repos/sketchy", "container": "/workspace" }
  ],
  "ports": [
    { "host": 3000, "container": 3000 }
  ],
  "workdir": "/workspace",
  "network": true,
  "security": {
    "no_root": false,
    "drop_caps": false,
    "read_only_rootfs": false,
    "seccomp_default": false
  }
}
```

All fields optional except `mounts`. Defaults: Alpine image, network enabled, no security hardening.

When `setup` commands are present, a temporary Dockerfile is generated and built automatically.

### Security Options

| Option | Docker flag | Effect |
|--------|------------|--------|
| `no_root` | `--user 1000:1000` | Run as non-root user |
| `drop_caps` | `--cap-drop=ALL` | Drop all Linux capabilities |
| `read_only_rootfs` | `--read-only --tmpfs /tmp` | Read-only root filesystem |
| `seccomp_default` | default seccomp profile | Explicit default seccomp (blocks dangerous syscalls) |

### Container Identification

Containers are named `sandbox-<name>` and labeled with `sandbox.managed=true`. This means:

- They're easy to spot in `docker ps`
- `sandbox rm --orphans` only touches containers created by this tool
- Description and creation date travel with the container as Docker labels

## License

MIT

---

# sandbox (Castellano)

Gestor de contenedores Docker via configs JSON. Ejecuta codigo no confiable en contenedores aislados sin mantener Dockerfiles separados.

## Instalacion

```bash
go build -o sandbox .
cp sandbox ~/.local/bin/
```

Requiere Docker instalado y corriendo.

## Inicio rapido

```bash
# Entra al directorio del proyecto que queres aislar
cd ~/repos/repo-sospechoso

# Crea y entra a un sandbox (monta el directorio actual en /workspace)
sandbox miproyecto --build

# Usa una imagen base especifica
sandbox miproyecto --build --image node:20

# Ejecuta un comando adentro
sandbox miproyecto -- npm install

# Comandos de shell tambien funcionan
sandbox miproyecto -- "cd src && ls -la"

# Lista todos los sandboxes
sandbox list

# Frenar, borrar, limpiar
sandbox stop miproyecto
sandbox rm miproyecto
```

### Personalizar con comandos de setup

Podes instalar paquetes dentro del contenedor agregando comandos de setup al config. Se ejecutan durante el `docker build` y quedan como parte de la imagen.

```bash
# Agregar zsh a un sandbox existente
sandbox config miproyecto setup + "apk add --no-cache zsh"

# Rebuild para que aplique (genera un Dockerfile y lo buildea)
sandbox miproyecto --rebuild

# O configurarlo antes del primer build
sandbox config miproyecto setup + "apk add --no-cache zsh curl git"
sandbox config miproyecto mounts + ~/repos/repo-sospechoso:/workspace
sandbox miproyecto --build
```

## Uso

```
sandbox <nombre> [--build|--rebuild] [--ephemeral] [--exit] [--image <img>] [--config <path>] [--description "texto"] [-- <cmd>]
sandbox list
sandbox stop <nombre>
sandbox rm <nombre> [--forget [-y]]
sandbox rm --orphans [-y]
sandbox config <nombre> [<key>] [<valor>]
```

### Ciclo de vida

| Comando | Que hace |
|---------|----------|
| `sandbox foo --build` | Crea el contenedor (falla si ya existe). Entra al shell. |
| `sandbox foo --rebuild` | Destruye y recrea desde cero. Entra al shell. |
| `sandbox foo --build --exit` | Crea sin entrar (para scripts). |
| `sandbox foo` | Arranca y entra al contenedor existente. |
| `sandbox foo -- <cmd>` | Ejecuta un comando en el contenedor. |
| `sandbox foo --ephemeral` | Sesion descartable (`docker run --rm`). |
| `sandbox stop foo` | Frena un contenedor corriendo. |
| `sandbox rm foo` | Borra contenedor + imagen custom. Mantiene config. |
| `sandbox rm foo --forget` | Borra todo incluido el config (pide confirmacion). |
| `sandbox rm --orphans` | Limpia contenedores cuyo config fue borrado. |

### Configuracion

Los configs viven en `~/.config/sandbox/<nombre>.json`. Se crean automaticamente en el primer `--build` (monta el pwd en `/workspace`).

Gestionar desde CLI:

```bash
sandbox config myapp                                # Mostrar config completo
sandbox config myapp image "node:20"                # Cambiar imagen
sandbox config myapp mounts + /path:/workspace      # Agregar mount
sandbox config myapp ports + 3000:3000              # Agregar puerto
sandbox config myapp setup + "apk add git"          # Agregar comando de setup
sandbox config myapp security.drop_caps true        # Habilitar opcion de seguridad
sandbox config myapp mounts - /path:/workspace      # Quitar mount
```

### Estructura del JSON

```json
{
  "description": "Paquete npm sospechoso de Reddit",
  "image": "alpine:latest",
  "setup": [
    "apk add --no-cache nodejs npm"
  ],
  "mounts": [
    { "host": "/home/user/repos/sospechoso", "container": "/workspace" }
  ],
  "ports": [
    { "host": 3000, "container": 3000 }
  ],
  "workdir": "/workspace",
  "network": true,
  "security": {
    "no_root": false,
    "drop_caps": false,
    "read_only_rootfs": false,
    "seccomp_default": false
  }
}
```

Todos los campos son opcionales excepto `mounts`. Defaults: imagen Alpine, red habilitada, sin hardening de seguridad.

Cuando hay comandos `setup`, se genera y buildea un Dockerfile temporal automaticamente.

### Opciones de seguridad

| Opcion | Flag Docker | Efecto |
|--------|------------|--------|
| `no_root` | `--user 1000:1000` | Ejecuta como usuario no-root |
| `drop_caps` | `--cap-drop=ALL` | Remueve todas las Linux capabilities |
| `read_only_rootfs` | `--read-only --tmpfs /tmp` | Filesystem root de solo lectura |
| `seccomp_default` | perfil seccomp default | Seccomp por defecto explicito (bloquea syscalls peligrosas) |

### Identificacion de contenedores

Los contenedores se llaman `sandbox-<nombre>` y llevan el label `sandbox.managed=true`. Esto significa:

- Son faciles de identificar en `docker ps`
- `sandbox rm --orphans` solo toca contenedores creados por esta herramienta
- La descripcion y fecha de creacion viajan con el contenedor como labels de Docker

## Licencia

MIT
