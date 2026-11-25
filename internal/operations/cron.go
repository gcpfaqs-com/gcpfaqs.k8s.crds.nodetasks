package operations

import "fmt"

// CronServiceHandler implementa Handler para la operación "system.service.cron".
// Se registra en main() dentro de Registry y se invoca desde Reconcile() cuando task.Spec.Operation vale "system.service.cron".
// Existe como tipo propio para encapsular las acciones válidas del servicio cron sin modificar el resto del sistema al añadir nuevas operaciones.
type CronServiceHandler struct{}

// Name devuelve el identificador de operación que debe coincidir con spec.operation en el recurso NodeTask ("system.service.cron").
// Sirve como clave en el mapa de Registry para enrutar cada petición al manejador adecuado.
func (h *CronServiceHandler) Name() string {
	return "system.service.cron"
}

// Build valida options["action"] y construye el comando de Ansible para gestionar el servicio cron mediante el módulo ansible.builtin.service.
// Admite las acciones "restart", "reload", "start" y "stop", traduciéndolas a los estados "restarted", "reloaded", "started" y "stopped" con privilegios (Become: true).
func (h *CronServiceHandler) Build(options map[string]string) (Command, error) {
	action := options["action"]
	switch action {
	case "restart", "reload", "start", "stop":
		stateMap := map[string]string{
			"restart": "restarted",
			"reload":  "reloaded",
			"start":   "started",
			"stop":    "stopped",
		}
		return Command{
			Module: "ansible.builtin.service",
			Args:   fmt.Sprintf("name=cron state=%s", stateMap[action]),
			Become: true,
		}, nil
	default:
		return Command{}, fmt.Errorf("invalid action for %s: %s", h.Name(), action)
	}
}
