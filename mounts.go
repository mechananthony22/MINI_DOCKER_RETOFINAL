package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

func setupMounts(volumeFlag string) error {
	// 1. Evitar que los montajes se propaguen al host
	if err := syscall.Mount("", "/", "", syscall.MS_PRIVATE|syscall.MS_REC, ""); err != nil {
		return fmt.Errorf("error montando / como privado: %v", err)
	}

	rootfsPath := "/tmp/rootfs"

	if _, err := os.Stat(rootfsPath); os.IsNotExist(err) {
		return fmt.Errorf("no se encontró rootfs en %s", rootfsPath)
	}

	// Montaje del volumen (antes del pivot root)
	// volumeFlag viene con el formato "ruta_host:ruta_contenedor"
	if volumeFlag != "" {
		parts := strings.Split(volumeFlag, ":")
		if len(parts) == 2 {
			hostPath := parts[0]
			contPath := filepath.Join(rootfsPath, parts[1])
			
			// Asegurar que el destino dentro del rootfs existe
			os.MkdirAll(contPath, 0755)
			
			fmt.Printf("Montando volumen %s en %s\n", hostPath, parts[1])
			if err := syscall.Mount(hostPath, contPath, "bind", syscall.MS_BIND|syscall.MS_REC, ""); err != nil {
				fmt.Printf("Advertencia: No se pudo montar el volumen %s: %v\n", volumeFlag, err)
			}
		} else {
			fmt.Println("Advertencia: Formato de volumen incorrecto. Usar /ruta/host:/ruta/contenedor")
		}
	}

	// 2. Montar el rootfs sobre sí mismo
	if err := syscall.Mount(rootfsPath, rootfsPath, "bind", syscall.MS_BIND|syscall.MS_REC, ""); err != nil {
		return fmt.Errorf("error haciendo bind mount del rootfs: %v", err)
	}

	// 3. Crear directorio para el old_root
	oldRoot := filepath.Join(rootfsPath, ".oldroot")
	if err := os.MkdirAll(oldRoot, 0700); err != nil {
		return fmt.Errorf("error creando oldroot: %v", err)
	}

	// 4. Pivot Root
	if err := syscall.PivotRoot(rootfsPath, oldRoot); err != nil {
		return fmt.Errorf("error en pivot_root: %v", err)
	}

	// 5. Cambiar el directorio de trabajo al nuevo root
	if err := syscall.Chdir("/"); err != nil {
		return fmt.Errorf("error cambiando al nuevo /: %v", err)
	}

	// 6. Montar el sistema virtual /proc
	if err := syscall.Mount("proc", "/proc", "proc", 0, ""); err != nil {
		return fmt.Errorf("error montando /proc: %v", err)
	}

	// 7. Desmontar el old root y borrar el directorio
	if err := syscall.Unmount("/.oldroot", syscall.MNT_DETACH); err != nil {
		fmt.Printf("Advertencia: No se pudo desmontar oldroot: %v\n", err)
	} else {
		os.Remove("/.oldroot")
	}

	return nil
}
