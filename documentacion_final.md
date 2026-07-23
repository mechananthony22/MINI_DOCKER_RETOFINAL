# Documentación Final: Proyecto Mini-Docker

Este documento recopila de principio a fin el marco teórico, el proceso de desarrollo y los comandos utilizados para construir y probar nuestro propio runtime de contenedores en Go, diseñado para revelar qué hace Docker "por debajo del capó".

---

## 1. ¿Qué son los Contenedores y cómo funcionan realmente?

Es un error muy común pensar que un contenedor es una "máquina virtual ligera". **Un contenedor no es una máquina virtual, es simplemente un proceso normal de Linux**. 

Lo que lo hace especial es que el Kernel de Linux le "miente" a ese proceso, presentándole una vista completamente aislada del sistema. Para lograr esta magia, Linux utiliza dos tecnologías fundamentales que hemos implementado en este proyecto:

1. **Namespaces (El Aislamiento Visual):** Limitan lo que un proceso puede **ver**. 
   - *UTS Namespace*: Permite que el contenedor tenga su propio nombre de host (`hostname`), independiente del host físico.
   - *PID Namespace*: Le hace creer al proceso que es el primero en arrancar (asignándole el PID 1) y le oculta los procesos de la máquina real.
   - *Mount Namespace (MNT)*: Permite aislar el sistema de archivos para que el contenedor solo vea su propia raíz (RootFS).

2. **Cgroups (El Límite de Recursos):** Mientras los namespaces limitan lo que el proceso *ve*, los Control Groups limitan lo que el proceso *puede consumir*. Permiten ponerle un techo fijo de memoria RAM, uso de CPU o red a un grupo de procesos, matándolos (OOM Killer) si se exceden.

---

## 2. Cómo construimos Mini-Docker

El proyecto fue construido en lenguaje **Go**, ya que ofrece acceso nativo de bajo nivel a las llamadas del sistema de Linux a través del paquete `syscall`.

### Arquitectura del Código
El programa utiliza un "patrón de re-ejecución" (fork and exec):
1. **El padre (`main.go - run`)**: Toma la petición del usuario, configura las banderas de clonación (`SysProcAttr` con `CLONE_NEWUTS`, `CLONE_NEWPID`, `CLONE_NEWNS`) y lanza una copia de sí mismo pasándole el argumento secreto `child`.
2. **El hijo (`main.go - child`)**: Este proceso ya nace "aislado" por el kernel. Se encarga de cambiar el hostname, restringir la memoria y CPU (escribiendo en `/sys/fs/cgroup`), y finalmente usar `pivot_root` para cambiar el disco duro raíz a uno de Alpine Linux, antes de lanzar la terminal que el usuario pidió (`/bin/sh`).

---

## 3. Comandos Utilizados durante el Desarrollo

Dado que el código se programó en Windows, pero depende de características exclusivas de Linux, utilizamos un flujo de compilación cruzada y transferencia en red local.

### A. Compilar en Windows para Linux
En la terminal (CMD) de Windows, le indicamos al compilador de Go que nuestro objetivo era un sistema Linux:
```cmd
set GOOS=linux
go build -o mini-docker
```

### B. Transferir los binarios a la Máquina Virtual (Kali Linux)
Para evitar los problemas de red NAT en VirtualBox, levantamos un pequeño servidor web usando PHP en Windows:
```cmd
php -S 0.0.0.0:8000
```
Y desde la terminal de Kali Linux, descargamos el programa compilado y el script de preparación:
```bash
wget http://10.0.2.2:8000/mini-docker
wget http://10.0.2.2:8000/setup-rootfs.sh
```

---

## 4. Comandos para Ejecutar el Proyecto (En Linux)

Una vez que los archivos estaban en Kali Linux, preparamos el entorno e iniciamos el contenedor:

1. **Dar permisos de ejecución**:
   ```bash
   chmod +x mini-docker setup-rootfs.sh
   ```

2. **Preparar el sistema de archivos aislado (Rootfs)**:
   ```bash
   ./setup-rootfs.sh
   ```
   *Este comando descargó un sistema Alpine Linux virgen y lo extrajo en `/tmp/rootfs` para que nuestro contenedor pueda encerrarse allí.*

3. **Arrancar el contenedor**:
   ```bash
   ./mini-docker run /bin/sh
   ```
   *Inicia el proceso y abre una terminal sh (shell) dentro de la celda de aislamiento.*

---

## 5. Comprobación del Aislamiento (Comandos dentro del Contenedor)

Una vez que el prompt cambió a `/ #` (indicando que estábamos dentro del contenedor como superusuario), ejecutamos comandos para verificar que las paredes del contenedor fueran reales:

1. **Comprobar el Namespace UTS**:
   ```bash
   hostname
   ```
   *Salida:* `minidocker`. Demuestra que el nombre de la máquina fue aislado sin afectar al Kali Linux original.

2. **Comprobar el Namespace PID y MNT**:
   ```bash
   ps aux
   ```
   *Salida:* Mostró únicamente 3 procesos, en los que nuestro ejecutable principal se apropió del **PID 1**, seguido por el `/bin/sh`. Los cientos de procesos normales de Kali Linux eran completamente invisibles.

3. **Comprobar el RootFS (Mount Namespace)**:
   ```bash
   ls /
   ```
   *Mostró una estructura básica de carpetas de Alpine Linux. Los archivos del usuario en `/home/kali` no existían, demostrando que estábamos atrapados en la nueva raíz (`pivot_root`).*

Para finalizar la sesión y matar el contenedor, simplemente ejecutamos:
```bash
exit
```

---

## 6. Ejecutando Aplicaciones Gráficas (X11) en Mini-Docker

También es posible correr aplicaciones con interfaz visual dentro de tu contenedor y que la ventana se muestre en tu máquina anfitriona (Kali Linux).

Para lograr esto, necesitamos compartir el "socket" del servidor gráfico X11 y configurar la variable de entorno `DISPLAY`.

**Pasos a seguir (en Kali Linux):**

1. **Dar permisos al servidor gráfico local**: 
   Antes de arrancar el contenedor, debes permitir que conexiones locales dibujen en tu pantalla.
   ```bash
   xhost +local:
   ```

2. **Levantar el contenedor con volúmenes de video**:
   Inicia el contenedor pasando el directorio `/tmp/.X11-unix` y la variable `DISPLAY`:
   ```bash
   sudo ./mini-docker run -v /tmp/.X11-unix:/tmp/.X11-unix -e DISPLAY=$DISPLAY /bin/sh
   ```

3. **Instalar y correr una aplicación gráfica**:
   Una vez dentro del contenedor (el prompt `/ #`), ya que hemos actualizado `setup-rootfs.sh` para copiar `/etc/resolv.conf`, tendrás internet. Puedes usar el gestor de paquetes de Alpine para instalar un reloj gráfico y ejecutarlo:
   ```bash
   apk add xclock
   xclock
   ```

La ventana de `xclock` debería aparecer en tu escritorio de Kali, ¡pero el proceso está corriendo totalmente aislado dentro del contenedor!
