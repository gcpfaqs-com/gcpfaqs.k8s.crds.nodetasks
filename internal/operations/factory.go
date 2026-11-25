package operations

import "fmt"

// Command encapsula la instrucción normalizada que debe ejecutar Ansible: qué módulo usar (Module), qué argumentos pasar (Args) y si requiere privilegios elevados (Become).
// Se crea dentro de cada Handler.Build() y se pasa a Runner.Run() para ejecutar el comando sin que el runner conozca las reglas de cada operación.
type Command struct {
	Module string
	Args   string
	Become bool
}

// Handler define el contrato que debe cumplir cada operación soportada por el CRD.
type Handler interface {
	Name() string
	Build(options map[string]string) (Command, error)
}

// Registry almacena y busca los manejadores de operaciones (Handler) mediante un mapa indexado por nombre.
// Se crea en main() con los manejadores disponibles y se inyecta en NodeTaskReconciler para resolver qué lógica corresponde a task.Spec.Operation.
// Existe como struct propio para centralizar el despacho de operaciones y evitar grandes bloques switch dentro del reconciliador.
type Registry struct {
	handlers map[string]Handler
}

// NewRegistry inicializa el registro de operaciones a partir de una lista variable de implementaciones de Handler.
// Indexa cada manejador por su nombre (h.Name()) en un mapa para que el reconciliador pueda resolver operaciones en tiempo constante.
func NewRegistry(handlers ...Handler) *Registry {
	reg := &Registry{handlers: make(map[string]Handler, len(handlers))}
	for _, h := range handlers {
		reg.handlers[h.Name()] = h
	}
	return reg
}

// Get busca y devuelve el Handler registrado para el nombre de operación solicitado.
// Si la operación no existe en el registro, devuelve un error explícito indicando que la operación no está soportada.
func (r *Registry) Get(name string) (Handler, error) {
	h, ok := r.handlers[name]
	if !ok {
		return nil, fmt.Errorf("unsupported operation: %s", name)
	}
	return h, nil
}
