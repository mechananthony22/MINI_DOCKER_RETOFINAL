# Bitácora de Depuración: Mini-Docker

Este documento sirve como registro de los problemas técnicos encontrados durante el desarrollo y las pruebas de bajo nivel interactuando con las primitivas de Linux, y las soluciones aplicadas.

## Entrada 1: Problemas de Propagación de Montajes
**Problema Encontrado:** Al ejecutar el contenedor y montar `/proc` virtual, el sistema host (fuera del contenedor) se vio afectado. El comando `ps` en el host dejó de funcionar correctamente.
**Diagnóstico:** El montaje `/` en el host utilizaba banderas de propagación compartida (`MS_SHARED`), de manera que todos los montajes dentro del contenedor se filtraban al host.
**Solución Aplicada:** Antes de manipular los montajes, llamamos a `syscall.Mount("", "/", "", syscall.MS_PRIVATE|syscall.MS_REC, "")` en el entorno aislado para hacer que todo el árbol sea privado y cortar la propagación de vuelta al host.

## Entrada 2: Fallo en Pivot Root (Invalid Argument)
**Problema Encontrado:** `syscall.PivotRoot(new_root, put_old)` retornaba `EINVAL` (Invalid Argument).
**Diagnóstico:** Según las especificaciones de `pivot_root(2)`, `new_root` y `put_old` no pueden estar en el mismo sistema de archivos que la raíz actual o padre. Además, `new_root` *debe* ser un punto de montaje en sí mismo.
**Solución Aplicada:** 
Hicimos un "bind mount" del directorio `new_root` sobre sí mismo:
`syscall.Mount(rootfsPath, rootfsPath, "bind", syscall.MS_BIND|syscall.MS_REC, "")`.
Esto engaña al kernel para que el directorio pase a ser su propio punto de montaje, cumpliendo el requerimiento de `pivot_root`.

## Entrada 3: Permisos denegados (Operation Not Permitted)
**Problema Encontrado:** Al correr la CLI e intentar utilizar `SysProcAttr` con las flags de Namespaces.
**Diagnóstico:** Linux requiere privilegios administrativos (la capability `CAP_SYS_ADMIN`) para crear la gran mayoría de namespaces y modificar montajes o cgroups.
**Solución Aplicada:** En la documentación especificamos que la herramienta debe ejecutarse con `sudo`.

---

*(Estudiante: Por favor, utiliza las siguientes secciones para registrar problemas adicionales que encuentres, por ejemplo, relacionados con el Cgroup V2 o la resolución DNS (resolv.conf) si expandes el contenedor).*

## Entrada 4: [Añadir Título]
**Problema Encontrado:**
**Diagnóstico:**
**Solución Aplicada:**
