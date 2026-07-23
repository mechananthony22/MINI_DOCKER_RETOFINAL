package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"syscall"
)

// usage: mini-docker run [-v host:container] [-e KEY=VAL] <cmd> <args>
func main() {
	if len(os.Args) < 2 {
		fmt.Printf("Uso: %s [run|ui] ...\n", os.Args[0])
		os.Exit(1)
	}

	switch os.Args[1] {
	case "run":
		if len(os.Args) < 3 {
			fmt.Printf("Uso: %s run [-v host:container] [-e KEY=VAL] <comando> [argumentos...]\n", os.Args[0])
			os.Exit(1)
		}
		run()
	case "child":
		child()
	case "ui":
		startUI()
	default:
		panic("Comando no reconocido")
	}
}

func run() {
	// Parseamos argumentos para 'run'
	runCmd := flag.NewFlagSet("run", flag.ExitOnError)
	volume := runCmd.String("v", "", "Volumen a montar (host:container)")
	envVar := runCmd.String("e", "", "Variable de entorno (KEY=VALUE)")
	runCmd.Parse(os.Args[2:])

	args := runCmd.Args()
	if len(args) == 0 {
		fmt.Println("Error: Se requiere un comando para ejecutar dentro del contenedor")
		os.Exit(1)
	}

	fmt.Printf("Corriendo %v con PID %d\n", args, os.Getpid())

	// Pasamos todos los argumentos originales a 'child' para que también los parsee
	childArgs := append([]string{"child"}, os.Args[2:]...)
	cmd := exec.Command("/proc/self/exe", childArgs...)

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Agregamos la variable de entorno si se especificó
	cmd.Env = os.Environ()
	if *envVar != "" {
		cmd.Env = append(cmd.Env, *envVar)
	}

	// Pasamos el volumen como variable de entorno oculta para que 'child' lo monte
	if *volume != "" {
		cmd.Env = append(cmd.Env, "MINI_DOCKER_VOL="+*volume)
	}

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags:   syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS,
		Unshareflags: syscall.CLONE_NEWNS,
	}

	if err := cmd.Run(); err != nil {
		fmt.Printf("Error ejecutando el contenedor: %v\n", err)
		os.Exit(1)
	}
}

func child() {
	// Parseamos argumentos también en 'child' para ignorar las flags (-v, -e)
	// y quedarnos solo con el comando real a ejecutar
	childCmd := flag.NewFlagSet("child", flag.ExitOnError)
	childCmd.String("v", "", "")
	childCmd.String("e", "", "")
	childCmd.Parse(os.Args[2:])

	args := childCmd.Args()
	fmt.Printf("Corriendo en el contenedor %v con PID %d\n", args, os.Getpid())

	// Configurar Cgroups v2
	cgPath, err := setupCgroup()
	if err != nil {
		fmt.Printf("Advertencia: No se pudieron configurar cgroups v2 (¿Estás en un sistema con cgroups v1?). Continuando sin límites...\n")
	}
	defer cleanupCgroup(cgPath)

	if err := syscall.Sethostname([]byte("minidocker")); err != nil {
		fmt.Printf("Error cambiando hostname: %v\n", err)
		os.Exit(1)
	}

	// Extraemos el volumen oculto del entorno (si lo hay) para setupMounts
	volMount := os.Getenv("MINI_DOCKER_VOL")

	if err := setupMounts(volMount); err != nil {
		fmt.Printf("Error configurando montajes: %v\n", err)
		os.Exit(1)
	}

	// Ejecutar comando final
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	// El entorno ya fue heredado del proceso padre (incluyendo -e si se pasó)

	if err := cmd.Run(); err != nil {
		fmt.Printf("Error ejecutando comando final: %v\n", err)
		os.Exit(1)
	}
}

type ContainerInfo struct {
	PID     int    `json:"pid"`
	Command string `json:"command"`
	Status  string `json:"status"`
}

var (
	activeContainers = make(map[int]ContainerInfo)
	mutex            sync.Mutex
)

func startUI() {
	fmt.Println("Iniciando Mini-Docker UI en http://0.0.0.0:9000")

	// Servir archivos estáticos desde la carpeta 'ui'
	fs := http.FileServer(http.Dir("./ui"))
	http.Handle("/", fs)

	// API para obtener contenedores activos
	http.HandleFunc("/api/containers", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		mutex.Lock()
		defer mutex.Unlock()

		list := []ContainerInfo{}
		for _, c := range activeContainers {
			// Revisar si el proceso sigue vivo usando la señal 0
			if err := syscall.Kill(c.PID, 0); err != nil {
				c.Status = "Exited"
				activeContainers[c.PID] = c
			}
			list = append(list, c)
		}
		json.NewEncoder(w).Encode(list)
	})

	// API para iniciar un nuevo contenedor
	http.HandleFunc("/api/run", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			Cmd string `json:"cmd"`
			Env string `json:"env"`
			Vol string `json:"vol"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		args := []string{"run"}
		if req.Vol != "" {
			args = append(args, "-v", req.Vol)
		}
		if req.Env != "" {
			args = append(args, "-e", req.Env)
		}

		// Como recibimos el comando como un solo string (ej: "echo hola > archivo"),
		// la forma correcta de ejecutarlo es pasárselo al shell interno del contenedor.
		args = append(args, "/bin/sh", "-c", req.Cmd)

		// Ejecutar ./mini-docker run en segundo plano
		cmd := exec.Command("./mini-docker", args...)

		// Enviar la salida a un archivo o a la nada para evitar bloquear
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Start(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		pid := cmd.Process.Pid
		mutex.Lock()
		activeContainers[pid] = ContainerInfo{
			PID:     pid,
			Command: req.Cmd,
			Status:  "Running",
		}
		mutex.Unlock()

		// Como es segundo plano, no esperamos.
		go func() {
			cmd.Wait()
		}()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"pid": pid, "status": "started"})
	})

	// API para apagar el servidor UI
	http.HandleFunc("/api/shutdown", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "shutting down"})

		go func() {
			fmt.Println("\nApagando servidor UI...")
			os.Exit(0)
		}()
	})

	if err := http.ListenAndServe(":9000", nil); err != nil {
		fmt.Printf("Error iniciando servidor web: %v\n", err)
	}
}
