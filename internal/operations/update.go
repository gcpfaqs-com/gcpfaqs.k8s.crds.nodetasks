package operations

// SystemUpdateHandler implementa Handler para la operación "system.update".
// Se registra en main() dentro de Registry y se invoca desde Reconcile() cuando task.Spec.Operation vale "system.update".
// Está separado de CronServiceHandler para mantener cada operación aislada y facilitar añadir o modificar comportamientos de actualización sin tocar otros manejadores.
type SystemUpdateHandler struct{}

// Name devuelve el identificador de operación "system.update" para actualizar paquetes del sistema en los nodos objetivo.
// Permite declarar actualizaciones del sistema operativo desde spec.operation sin escribir playbooks YAML manuales.
func (h *SystemUpdateHandler) Name() string {
	return "system.update"
}

// Build construye el comando de Ansible para actualizar la caché y realizar una actualización de paquetes con ansible.builtin.apt y elevación de privilegios.
// En esta versión mínima ejecuta upgrade=dist y update_cache=yes con Become: true.
func (h *SystemUpdateHandler) Build(options map[string]string) (Command, error) {
	return Command{
		Module: "ansible.builtin.apt",
		Args:   "upgrade=dist update_cache=yes",
		Become: true,
	}, nil
}
