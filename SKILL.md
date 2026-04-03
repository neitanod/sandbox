---
name: sandbox-containers
description: Crear y gestionar contenedores Docker aislados para ejecutar codigo no
  confiable usando configs JSON. Usalo cuando necesites aislar un repo, ejecutar codigo
  sospechoso, o crear entornos descartables.
---

# sandbox - Contenedores Docker aislados via JSON

## Cuando usar este skill

- Necesitas ejecutar codigo de un repo que no conoces en un entorno aislado
- Queres crear un contenedor para desarrollo sin escribir un Dockerfile
- Necesitas aislar un proyecto con mounts, puertos y seguridad configurables
- Alguien pide "correr esto en un sandbox" o "aislar este proyecto"

## Requisitos

- Docker instalado y corriendo
- Binario `sandbox` en el PATH

## Instalacion

```bash
cd /home/sebas/robotin/apps/sandbox
go build -o sandbox .
ln -sf "$(pwd)/sandbox" ~/.local/bin/sandbox
```

Verificar: `sandbox --help`

## Conceptos clave

- Los configs viven en `~/.config/sandbox/<nombre>.json`
- Los contenedores se llaman `sandbox-<nombre>` en Docker
- Si no existe config, `sandbox build` crea uno minimo montando el pwd en `/workspace`
- Si hay comandos `setup` en el JSON, se genera un Dockerfile temporal y se buildea automaticamente

## Flujo tipico: aislar un repo

```bash
# 1. Entrar al directorio del repo
cd ~/repos/repo-sospechoso

# 2. Crear sandbox (monta pwd en /workspace automaticamente)
sandbox build miproyecto --description "Paquete npm sospechoso"

# 3. Ya estas adentro del shell del contenedor
# Salis con exit

# 4. Volver a entrar cuando quieras
sandbox miproyecto

# 5. Ejecutar un comando sin entrar
sandbox miproyecto -- npm install

# 6. Comandos con shell syntax funcionan
sandbox miproyecto -- "cd src && ls -la"
```

## Flujo tipico: sandbox con imagen especifica

```bash
# Node.js
sandbox build miproyecto --image node:20 --description "App Node"

# Python
sandbox build miproyecto --image python:3.12 --description "Script Python"

# Ubuntu
sandbox build miproyecto --image ubuntu:24.04 --description "Testing"
```

## Instalar paquetes dentro del contenedor

Los comandos de setup se ejecutan durante el `docker build` y quedan como parte de la imagen:

```bash
# Agregar paquetes al config
sandbox config miproyecto setup + "apk add --no-cache zsh curl git"

# Rebuild para que aplique
sandbox rebuild miproyecto
```

## Referencia rapida de comandos

### Ciclo de vida

```bash
sandbox build <nombre> [--exit] [--image <img>] [--description "texto"]
sandbox rebuild <nombre> [--exit] [--image <img>] [--description "texto"]
sandbox <nombre>                    # Entrar al contenedor
sandbox <nombre> -- <cmd>           # Ejecutar comando
sandbox <nombre> --ephemeral        # Sesion descartable (docker run --rm)
sandbox stop <nombre>               # Frenar contenedor
sandbox rm <nombre>                 # Borrar contenedor + imagen custom (mantiene config)
sandbox rm <nombre> --forget [-y]   # Borrar todo incluido config
sandbox rm --orphans [-y]           # Limpiar contenedores sin config
```

### Informacion y configuracion

```bash
sandbox list                        # Listar todos los sandboxes
sandbox config <nombre>             # Ver config completo
sandbox edit <nombre>               # Abrir config en $EDITOR
```

### Modificar config desde CLI

```bash
sandbox config <nombre> image "node:20"              # Cambiar imagen
sandbox config <nombre> mounts + /path:/workspace    # Agregar mount
sandbox config <nombre> mounts - /path:/workspace    # Quitar mount
sandbox config <nombre> ports + 3000:3000            # Agregar puerto
sandbox config <nombre> ports - 3000:3000            # Quitar puerto
sandbox config <nombre> setup + "apk add git"        # Agregar setup
sandbox config <nombre> setup - "apk add git"        # Quitar setup
sandbox config <nombre> network false                # Deshabilitar red
sandbox config <nombre> security.drop_caps true      # Activar seguridad
sandbox config <nombre> security.no_root true
sandbox config <nombre> security.read_only_rootfs true
sandbox config <nombre> security.seccomp_default true
```

## Estructura del JSON

```json
{
  "description": "Descripcion del sandbox",
  "image": "alpine:latest",
  "setup": [
    "apk add --no-cache nodejs npm"
  ],
  "mounts": [
    { "host": "/home/user/repos/proyecto", "container": "/workspace" }
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

Todos los campos opcionales excepto `mounts`. Defaults: Alpine, red habilitada, sin hardening.

## Opciones de seguridad

| Opcion | Flag Docker | Efecto |
|--------|------------|--------|
| `no_root` | `--user 1000:1000` | Ejecuta como usuario no-root |
| `drop_caps` | `--cap-drop=ALL` | Remueve todas las Linux capabilities |
| `read_only_rootfs` | `--read-only --tmpfs /tmp` | Filesystem root de solo lectura |
| `seccomp_default` | perfil seccomp default | Bloquea syscalls peligrosas |

## Flags importantes

| Flag | Donde | Efecto |
|------|-------|--------|
| `--exit` | build/rebuild | No entrar al shell despues de crear (util para scripts) |
| `--image` | build/rebuild | Especificar imagen base (override del JSON) |
| `--description` | build/rebuild | Descripcion del sandbox |
| `--config` | build/rebuild/run | Usar un JSON de config especifico |
| `--ephemeral` | run | Contenedor descartable (--rm) |
| `--forget` | rm | Borrar tambien el archivo de config |
| `-y` | rm --forget, rm --orphans | Saltar confirmacion |

## Ejemplo: crear sandbox para agente

```bash
# Crear sandbox para que un agente ejecute codigo aislado
cd /path/al/repo
sandbox build agente-task --image node:20 --exit --description "Tarea del agente"

# Ejecutar comandos desde afuera
sandbox agente-task -- npm install
sandbox agente-task -- npm test

# Limpiar cuando termine
sandbox rm agente-task --forget -y
```

## Troubleshooting

### "container already exists"
```bash
sandbox rebuild <nombre>   # Destruye y recrea
```

### "config not found"
```bash
sandbox build <nombre>     # Crea config minimo automaticamente
```

### Comando con shell syntax falla
```bash
# Usar comillas para que sandbox lo envuelva en sh -c
sandbox <nombre> -- "cd src && npm test"
```

### Ver estado de todos los sandboxes
```bash
sandbox list
```

## Codigo fuente

`/home/sebas/robotin/apps/sandbox/`

Repo: `https://github.com/neitanod/sandbox`
