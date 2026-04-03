# sandbox - CLI para contenedores Docker configurables via JSON

## Resumen

Herramienta CLI en Go que gestiona contenedores Docker "descartables" para ejecutar codigo no confiable. La configuracion vive en archivos JSON (en vez de Dockerfiles dispersos), permitiendo crear, recrear y reutilizar contenedores con un solo comando.

## Uso

```
sandbox <nombre> [--build|--rebuild] [--ephemeral] [--config <path>] [--description "texto"] [-- <comando>]
sandbox list
sandbox stop <nombre>
sandbox rm <nombre> [--forget [-y]]
sandbox rm --orphans [-y]
sandbox config <nombre> <key> [<value>]
```

### Ejemplos

```bash
# Ciclo de vida basico
sandbox sketchy-repo --build                                # Crea el contenedor por primera vez
sandbox sketchy-repo --build --description "npm sospechoso" # Idem con descripcion custom
sandbox sketchy-repo                                        # Lo arranca y da shell interactiva
sandbox sketchy-repo -- npm start                           # Lo arranca y ejecuta un comando
sandbox sketchy-repo --rebuild                              # Lo destruye y recrea desde cero
sandbox sketchy-repo --rebuild --description "nueva desc"   # Rebuild con descripcion nueva
sandbox sketchy-repo --ephemeral                            # Sesion descartable (--rm)
sandbox sketchy-repo --ephemeral -- python app.py

# Gestion
sandbox list                                                # Lista sandboxes (configs + huerfanos)
sandbox stop sketchy-repo                                   # Frena el contenedor
sandbox rm sketchy-repo                                     # Borra contenedor + imagen custom
sandbox rm sketchy-repo --forget                            # Borra contenedor + imagen + JSON (pide confirmacion)
sandbox rm sketchy-repo --forget -y                         # Idem sin confirmacion
sandbox rm --orphans                                        # Limpia contenedores huerfanos (pide confirmacion)
sandbox rm --orphans -y                                     # Idem sin confirmacion

# Configuracion desde CLI
sandbox config myapp description "Proyecto sospechoso"      # Setea un campo simple
sandbox config myapp image "node:20"                        # Cambia la imagen
sandbox config myapp network false                          # Deshabilita red
sandbox config myapp security.drop_caps true                # Campo anidado
sandbox config myapp security.read_only_rootfs true         # Otro campo anidado
sandbox config myapp mounts + /home/sebas/repo:/workspace   # Agrega un mount (+ = append)
sandbox config myapp mounts - /home/sebas/repo:/workspace   # Quita un mount (- = remove)
sandbox config myapp ports + 3000:3000                      # Agrega un port mapping
sandbox config myapp ports - 3000:3000                      # Quita un port mapping
sandbox config myapp setup + "apk add git"                  # Agrega un comando de setup
sandbox config myapp setup - "apk add git"                  # Quita un comando de setup
sandbox config myapp description                            # Sin valor: muestra el valor actual
sandbox config myapp                                        # Sin key: muestra todo el JSON
```

## JSON de configuracion

### Ubicacion

Busca en `~/.config/sandbox/<nombre>.json` por defecto. Se puede overridear con `--config <path>`.

### Estructura

```json
{
  "description": "Repo npm sospechoso que encontre en Reddit",
  "image": "alpine:latest",
  "setup": [
    "apk add --no-cache nodejs npm git",
    "npm install -g typescript"
  ],
  "mounts": [
    { "host": "/home/sebas/repos/sketchy-repo", "container": "/workspace" }
  ],
  "ports": [
    { "host": 3000, "container": 3000 },
    { "host": 8080, "container": 80 }
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

### Campos

Todos opcionales excepto `mounts`.

| Campo | Default | Descripcion |
|-------|---------|-------------|
| `description` | `""` | Descripcion del sandbox. Se graba como label de Docker al crear. |
| `image` | `"alpine:latest"` | Imagen base del contenedor |
| `setup` | `[]` | Comandos a ejecutar en el build (genera Dockerfile temporal) |
| `mounts` | *requerido* | Carpetas del host a montar en el contenedor |
| `ports` | `[]` | Puertos del host a puentear al contenedor |
| `workdir` | Primer mount container path, o `/` | Directorio de trabajo al entrar |
| `network` | `true` | Acceso a red. `false` para aislar |
| `security` | todo `false` | Opciones de hardening (ver seccion Seguridad) |

## Labels de Docker

Al crear un contenedor, sandbox agrega labels para identificacion y metadata:

```
--label sandbox.managed=true
--label sandbox.created-at=2026-04-03
--label sandbox.description=Repo npm sospechoso que encontre en Reddit
```

- `sandbox.managed=true`: identifica contenedores creados por esta herramienta. Es la fuente de verdad para operaciones como `--orphans`.
- `sandbox.created-at`: fecha de creacion (YYYY-MM-DD).
- `sandbox.description`: descripcion del sandbox. Viene del JSON por defecto, pero `--description` en la linea de comandos tiene prioridad.

## Naming convention

- Contenedores: `sandbox-<nombre>` (ej: `sandbox-sketchy-repo`)
- Imagenes custom: `sandbox-<nombre>:latest` (solo cuando hay `setup`)

El prefijo `sandbox-` permite identificar visualmente los contenedores en `docker ps`.

## Ciclo de vida

### `--build` (primera vez)

1. Verifica que NO exista un contenedor con ese nombre. Si existe, error: `"ya existe, usa --rebuild"`.
2. Si hay `setup`: genera Dockerfile temporal, ejecuta `docker build`, imagen resultante: `sandbox-<nombre>:latest`.
3. Si no hay `setup`: usa la imagen base directamente.
4. Ejecuta `docker create` con volumes, ports, security flags y labels. Contenedor creado.

### `--rebuild` (recrear desde cero)

1. Si existe el contenedor: `docker rm -f`.
2. Si existe imagen custom `sandbox-<nombre>`: `docker rmi`.
3. Mismo flujo que `--build`.

### Sin flags (ejecutar)

1. Verifica que el contenedor exista. Si no, error: `"no existe, usa --build"`.
2. Si el contenedor esta detenido: `docker start`.
3. Si se pasa `-- <cmd>`: `docker exec` con ese comando.
4. Si no se pasa comando: `docker exec` con shell interactiva (`/bin/sh`).

### `--ephemeral`

1. Ejecuta `docker run --rm` directamente (no crea contenedor persistente).
2. Aplica los mismos labels (para consistencia en logs de Docker).
3. Si se pasa `-- <cmd>`: ejecuta ese comando.
4. Si no: shell interactiva.

### `--description "texto"`

Se puede usar con `--build` o `--rebuild`. Tiene prioridad sobre el campo `description` del JSON. La descripcion se graba como label `sandbox.description` en el contenedor. Solo se puede cambiar con `--rebuild`.

### `sandbox list`

Cruza dos fuentes de informacion:

- Los `.json` de `~/.config/sandbox/` (configuraciones)
- Los contenedores Docker con label `sandbox.managed=true`

Esto permite detectar:

- Configuraciones sin contenedor creado (`not created`)
- Contenedores huerfanos: existen en Docker con label `sandbox.managed=true` pero no tienen JSON en `~/.config/sandbox/` (se marcan como `orphaned`)

Para cada sandbox muestra: nombre, imagen, estado, descripcion (leida del label de Docker, o del JSON si no hay contenedor).

Ejemplo de salida:

```
NAME             IMAGE            STATUS        DESCRIPTION
sketchy-repo     alpine:latest    running       Repo npm sospechoso de Reddit
ml-experiment    python:3.11      stopped       Experimento con PyTorch
viejo-proyecto   node:20          orphaned      Proyecto que ya no uso
otro-repo        node:20          not created   Para probar el CLI de un desconocido
```

### `sandbox stop <nombre>`

Ejecuta `docker stop sandbox-<nombre>`. Error si el contenedor no existe o ya esta detenido.

### `sandbox rm <nombre>`

Borra el contenedor (`docker rm -f`) y la imagen custom si existe (`docker rmi sandbox-<nombre>:latest`). NO borra el JSON de configuracion.

### `sandbox rm <nombre> --forget`

Borra contenedor + imagen custom + archivo JSON de configuracion. Pide confirmacion:

```
You are about to destroy a container and also its configuration file, you'll lose it forever.
Are you sure? (use -y to skip this confirmation) y/N:
```

Default "N". Solo procede si el usuario escribe "y". Con `-y` se salta la confirmacion.

### `sandbox rm --orphans`

Busca contenedores Docker con label `sandbox.managed=true` que no tengan un `.json` correspondiente en `~/.config/sandbox/`. Lista los que va a borrar y pide confirmacion. Con `-y` se salta la confirmacion.

### `sandbox config <nombre> [<key>] [<value>]`

Crea o modifica el JSON de configuracion desde la linea de comandos. Opera sobre `~/.config/sandbox/<nombre>.json`.

**Modos de uso:**

- `sandbox config myapp` — Sin key: muestra el JSON completo (pretty-printed).
- `sandbox config myapp description` — Con key, sin valor: muestra el valor actual de ese campo.
- `sandbox config myapp description "texto"` — Con key y valor: setea el campo.

**Keys con dot-notation para campos anidados:**

Las keys soportan dot-notation para acceder a campos anidados del JSON:

- `security.drop_caps` → `{"security": {"drop_caps": ...}}`
- `security.read_only_rootfs` → `{"security": {"read_only_rootfs": ...}}`

**Operadores `+` y `-` para arrays (mounts, ports, setup):**

Los campos que son arrays (`mounts`, `ports`, `setup`) usan `+` para agregar y `-` para quitar:

- `sandbox config myapp mounts + /home/sebas/repo:/workspace` — Agrega un mount.
- `sandbox config myapp mounts - /home/sebas/repo:/workspace` — Quita un mount.
- `sandbox config myapp ports + 3000:3000` — Agrega un port mapping.
- `sandbox config myapp ports - 3000:3000` — Quita un port mapping.
- `sandbox config myapp setup + "apk add git"` — Agrega un comando de setup.
- `sandbox config myapp setup - "apk add git"` — Quita un comando de setup.

**Formato compacto para mounts y ports en CLI:**

Para no tener que escribir JSON, los mounts y ports se expresan con `:` en la linea de comandos:

- Mounts: `<host_path>:<container_path>` → se traduce a `{"host": "...", "container": "..."}`
- Ports: `<host_port>:<container_port>` → se traduce a `{"host": N, "container": N}`

**Inferencia de tipos:**

Los valores se parsean automaticamente:

- `true`/`false` → booleano
- Numeros → numero
- Todo lo demas → string

**Creacion automatica:**

Si el archivo `~/.config/sandbox/<nombre>.json` no existe, `config` lo crea con el campo especificado (y defaults para el resto). Esto permite crear un sandbox completamente desde CLI:

```bash
sandbox config myapp description "Proyecto sospechoso"
sandbox config myapp image "node:20"
sandbox config myapp mounts + /home/sebas/repos/myapp:/workspace
sandbox config myapp ports + 8080:80
sandbox config myapp security.drop_caps true
sandbox myapp --build
```

## Dockerfile temporal

Cuando el JSON tiene `setup`, se genera un Dockerfile en `/tmp/sandbox-<nombre>/Dockerfile`:

```dockerfile
FROM alpine:latest
RUN apk add --no-cache nodejs npm git
RUN npm install -g typescript
```

Se buildea con `docker build -t sandbox-<nombre>:latest /tmp/sandbox-<nombre>/` y se limpia el temporal.

## Opciones de seguridad

Configurables en `security` del JSON. Todas deshabilitadas por defecto.

| Opcion | Flag Docker | Que hace |
|--------|-------------|----------|
| `no_root` | `--user 1000:1000` | Ejecuta como usuario no-root (UID 1000). Los mounts se acceden con ese usuario. |
| `drop_caps` | `--cap-drop=ALL` | Remueve todas las Linux capabilities. El proceso no puede montar filesystems, cambiar owners, usar raw sockets, etc. |
| `read_only_rootfs` | `--read-only --tmpfs /tmp:rw,noexec,nosuid` | El filesystem root es read-only. Solo `/tmp` y los mounts explicitados son escribibles. Previene que codigo malicioso modifique binarios del sistema. |
| `seccomp_default` | `--security-opt seccomp=unconfined` removido (usa default) | Aplica el perfil seccomp por defecto de Docker. Bloquea syscalls peligrosas como `ptrace`, `mount`, `kexec_load`, etc. Docker ya lo aplica por defecto, pero esta opcion lo hace explicito y previene que se deshabilite. |

### Combinaciones recomendadas

- **Minimo para codigo no confiable**: `drop_caps: true`
- **Paranoico**: todos en `true`
- **Desarrollo normal**: todos en `false` (default)

## Estructura del proyecto

```
sandbox/
├── main.go           # CLI parsing, entry point
├── config.go         # Lectura y validacion del JSON (struct, defaults, load/save)
├── configcmd.go      # Comando "sandbox config": get/set/append/remove desde CLI
├── docker.go         # Interaccion con Docker CLI (build, create, start, exec, rm, stop)
├── dockerfile.go     # Generacion del Dockerfile temporal
├── labels.go         # Gestion de labels de Docker (managed, created-at, description)
├── list.go           # Comando list: cruza JSONs con contenedores Docker
├── security.go       # Traduccion de opciones de seguridad a flags Docker
└── doc/
    └── specs.md      # Este archivo
```

Sin dependencias externas. Solo stdlib de Go: `os/exec` para Docker, `encoding/json` para config, `flag` o similar para CLI args.

## Enfoque tecnico

- **Wrapper sobre Docker CLI**: no usa el Docker SDK, ejecuta `docker` directamente via `os/exec`. Mas simple, mas facil de debuggear (se puede ver el comando exacto que ejecuta).
- **Dockerfile temporal para setup**: si hay comandos `setup`, genera Dockerfile temporal, buildea, y limpia. Si no hay `setup`, usa la imagen base directamente sin build.
- **Naming convention**: contenedores se llaman `sandbox-<nombre>`, imagenes custom `sandbox-<nombre>:latest`.
- **Labels como metadata**: toda la metadata (managed, fecha, descripcion) se graba como labels de Docker para que viaje con el contenedor.
- **Doble identificacion**: prefijo `sandbox-` (visibilidad en `docker ps`) + label `sandbox.managed=true` (seguridad para operaciones destructivas).
