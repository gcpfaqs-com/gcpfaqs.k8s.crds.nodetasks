package runner

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"gcpfaqs.k8s.crds.nodetasks/internal/operations"
)

// Runner define la interfaz para ejecutar una operación normalizada sobre una lista de hosts.
type Runner interface {
	Run(ctx context.Context, hosts []string, cmd operations.Command, dryRun bool) (string, error)
}

// AnsibleRunner implementa Runner ejecutando el CLI de Ansible contra los hosts resueltos usando una cuenta SSH y una clave privada.
// Se instancia en main() y se inyecta en NodeTaskReconciler para ejecutar el operations.Command sobre la lista de nodos objetivo.
// Está separado del reconciliador para encapsular la creación del inventario temporal, los argumentos del binario ansible y la captura de salida.
type AnsibleRunner struct {
	sshUser string
	sshKey  string
}

// NewAnsibleRunner crea un ejecutor de Ansible configurado con el usuario SSH y la ruta a la clave privada que usará para conectarse a los nodos.
// Centraliza los parámetros de conexión para no mezclar detalles de transporte con la lógica de reconciliación de Kubernetes.
func NewAnsibleRunner(sshUser, sshKey string) *AnsibleRunner {
	return &AnsibleRunner{sshUser: sshUser, sshKey: sshKey}
}

// Run genera un inventario temporal con los hosts objetivo, ejecuta el binario "ansible" con el módulo y argumentos indicados y devuelve la salida combinada.
// Añade "--become" si cmd.Become es verdadero y "--check" si dryRun está activo, eliminando siempre el archivo temporal al finalizar.
func (r *AnsibleRunner) Run(ctx context.Context, hosts []string, cmd operations.Command, dryRun bool) (string, error) {
	invFile, err := os.CreateTemp("", "ansible-inv-*")
	if err != nil {
		return "", err
	}
	invPath := invFile.Name()
	defer os.Remove(invPath)

	var content strings.Builder
	content.WriteString("[targets]\n")
	for _, h := range hosts {
		fmt.Fprintf(&content, "%s ansible_user=%s ansible_ssh_private_key_file=%s\n", h, r.sshUser, r.sshKey)
	}
	if _, err := invFile.WriteString(content.String()); err != nil {
		invFile.Close()
		return "", err
	}
	if err := invFile.Close(); err != nil {
		return "", err
	}

	args := []string{
		"targets",
		"-i", invPath,
		"-m", cmd.Module,
		"-a", cmd.Args,
	}
	if cmd.Become {
		args = append(args, "--become")
	}
	if dryRun {
		args = append(args, "--check")
	}

	c := exec.CommandContext(ctx, "ansible", args...)
	var out bytes.Buffer
	c.Stdout = &out
	c.Stderr = &out
	err = c.Run()
	return out.String(), err
}
