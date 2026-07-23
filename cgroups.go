package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

const cgroupDir = "/sys/fs/cgroup/mini-docker"

func setupCgroup() (string, error) {
	// Usamos el PID actual (que será el 1 en el namespace del contenedor,
	// pero en el host es el PID real del hijo).
	// Espera, dentro del contenedor (después de CLONE_NEWPID) os.Getpid() devuelve 1.
	// Por tanto, necesitamos crear el cgroup DESDE el host o asegurarnos de usar
	// el PID real, pero Cgroups v2 permite que el propio proceso escriba su PID "virtual" si
	// los namespaces están bien configurados, aunque a veces es más seguro hacerlo desde `run` (el padre).
	// Para este taller, lo hacemos desde el `child` como simplificación, sabiendo que `/sys/fs/cgroup`
	// aún no está aislado o montamos cgroups antes.
	// Ojo: si ya hicimos un chroot, no veremos /sys/fs/cgroup.
	// Lo haremos ANTES del setupMounts() (que es cuando aislamos el filesystem).

	pid := os.Getpid() // Será 1 dentro del namespace, lo cual escribirá el PID 1 en el cgroup de este namespace?
	// En cgroups v2, al escribir en cgroup.procs un PID de un namespace, el kernel lo traduce al PID real del host si estamos en un cgroup namespace.
	// Pero si no usamos Cgroup Namespace, podría haber problemas al escribir el PID 1.
	// En linux, un proceso puede escribir "0" en cgroup.procs para referirse a sí mismo.
	
	cgPath := filepath.Join(cgroupDir, strconv.Itoa(pid))
	
	// Crear el directorio del cgroup
	if err := os.MkdirAll(cgPath, 0755); err != nil {
		return "", fmt.Errorf("error creando cgroup dir %s: %v", cgPath, err)
	}

	// Límite de Memoria: 50 MB
	memLimit := "52428800"
	if err := os.WriteFile(filepath.Join(cgPath, "memory.max"), []byte(memLimit), 0700); err != nil {
		return "", fmt.Errorf("error escribiendo memory.max: %v", err)
	}

	// Límite de CPU: 10% (10000 de cuota sobre 100000 de periodo)
	// cgroup v2 usa cpu.max "max period"
	cpuLimit := "10000 100000"
	if err := os.WriteFile(filepath.Join(cgPath, "cpu.max"), []byte(cpuLimit), 0700); err != nil {
		// Algunos entornos no tienen el controlador cpu habilitado en cgroups v2 por defecto, ignoramos si falla.
		fmt.Printf("Advertencia: No se pudo configurar CPU max (puede que el controlador no esté habilitado): %v\n", err)
	}

	// Añadir el proceso actual al cgroup
	// Escribir "0" añade el hilo/proceso actual al cgroup sin importar el namespace.
	if err := os.WriteFile(filepath.Join(cgPath, "cgroup.procs"), []byte("0"), 0700); err != nil {
		return "", fmt.Errorf("error escribiendo cgroup.procs: %v", err)
	}

	return cgPath, nil
}

func cleanupCgroup(cgPath string) {
	// Al terminar el contenedor, el directorio cgroup puede ser eliminado
	// una vez que no queden procesos.
	if cgPath != "" {
		if err := os.RemoveAll(cgPath); err != nil {
			fmt.Printf("Advertencia: No se pudo limpiar el cgroup %s: %v\n", cgPath, err)
		}
	}
}
