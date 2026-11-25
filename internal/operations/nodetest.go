package operations

import "fmt"

// NodeTestHandler implementa Handler para la operación "node.test".
// Se registra en main() dentro de Registry y se invoca desde Reconcile() cuando task.Spec.Operation vale "node.test".
// No admite opciones en spec.options para mantener la prueba fija y predecible.
type NodeTestHandler struct{}

// Name devuelve el identificador de operación "node.test" para comprobar conectividad y escritura en /var/log/syslog.
func (h *NodeTestHandler) Name() string {
	return "node.test"
}

// Build valida que no se envíen opciones y construye el comando de Ansible con privilegios para escribir en /var/log/syslog.
func (h *NodeTestHandler) Build(options map[string]string) (Command, error) {
	if len(options) > 0 {
		return Command{}, fmt.Errorf("operation %s does not accept options", h.Name())
	}

	return Command{
		Module: "ansible.builtin.shell",
		Args:   `/bin/sh -c 'echo "hello crd" > /var/log/syslog'`,
		Become: true,
	}, nil
}
