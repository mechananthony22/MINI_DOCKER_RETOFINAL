# Aislamiento en Mini-Docker: Namespaces y Cgroups

Este documento explica los mecanismos del kernel de Linux utilizados en Mini-Docker para crear la ilusión de una máquina independiente (aislamiento) y restringir el uso de recursos.

## Namespaces de Linux

Los namespaces limitan lo que un proceso puede **ver**. Linux provee varios namespaces, cada uno aislando un recurso global particular del sistema operativo.

### 1. UTS Namespace (UNIX Timesharing System)
**Qué aísla:** El hostname (nombre de la máquina) y el domain name.
**Implementación en Mini-Docker:** Se activa pasando la bandera `CLONE_NEWUTS` a `SysProcAttr.Cloneflags`. Dentro del proceso hijo, llamamos a `syscall.Sethostname` para asignar el nombre `"minidocker"`. Debido a este namespace, el cambio solo afecta al entorno del contenedor, mientras que el host mantiene su hostname original.

### 2. PID Namespace (Process ID)
**Qué aísla:** El árbol de identificadores de procesos.
**Implementación en Mini-Docker:** Utilizando `CLONE_NEWPID`, el proceso hijo creado se convierte en el PID 1 (el proceso "init") dentro del nuevo namespace. Cualquier otro proceso lanzado dentro de este namespace será hijo de él. Los procesos dentro del contenedor no pueden ver (ni enviar señales a) procesos fuera del contenedor, ofreciendo una capa fuerte de aislamiento de procesos.

### 3. Mount Namespace (MNT)
**Qué aísla:** Los puntos de montaje del sistema de archivos. Permite que un proceso tenga su propia vista de la jerarquía del sistema de archivos (mount tree).
**Implementación en Mini-Docker:** Usando `CLONE_NEWNS`.
Para que el proceso no afecte al host, en `mounts.go`:
1. Hacemos que el montaje raíz `/` sea **privado** (`MS_PRIVATE`), lo que impide que los montajes subsiguientes dentro del contenedor se "propaguen" (se vean) en el host.
2. Utilizamos **`pivot_root`** para intercambiar la raíz del sistema de archivos con un directorio pre-configurado (`/tmp/rootfs`).
3. Montamos un **`/proc` virtual**: Como el sistema de archivos cambió y tenemos un PID namespace propio, los comandos como `ps` o `top` (que leen de `/proc`) necesitan un filesystem `proc` que coincida con el nuevo PID namespace. Por ello, se monta explícitamente `proc` en `/proc`.

## Control Groups (Cgroups v2)

Mientras los namespaces limitan lo que el proceso *ve*, los cgroups limitan los recursos que el proceso *puede utilizar* (CPU, memoria, I/O de disco, red, etc.).

Mini-Docker asume y utiliza la jerarquía de **Cgroups v2**, la cual expone una única jerarquía unificada en `/sys/fs/cgroup`.

**Implementación en Mini-Docker:**
- **Creación:** Se crea un directorio específico para el contenedor: `/sys/fs/cgroup/mini-docker-<pid>`. En cgroups v2, crear un directorio automáticamente crea las interfaces para los controladores habilitados.
- **Límite de Memoria:** Escribimos el valor `52428800` (50 MB en bytes) en el archivo `memory.max`. Si el contenedor excede esta memoria, el kernel lanzará el proceso OOM (Out Of Memory) killer y matará el proceso causante dentro del contenedor.
- **Límite de CPU:** Escribimos la cuota de tiempo `10000 100000` (10% del tiempo de CPU) en `cpu.max`. Esto restringe la cantidad máxima de CPU que pueden consumir los procesos del contenedor de forma conjunta en cada periodo del scheduler del kernel.
- **Asignación de procesos:** El contenedor se añade a este cgroup escribiendo el valor `0` (que representa al proceso que hace la llamada) en el archivo `cgroup.procs`.

*Nota: La correcta aplicación de las políticas de cgroup requiere privilegios de administrador (`root`).*
