package controller

import (
	"context"

	"gcpfaqs.k8s.crds.nodetasks/internal/k8s"
	"gcpfaqs.k8s.crds.nodetasks/internal/operations"
	"gcpfaqs.k8s.crds.nodetasks/internal/runner"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// NodeTaskReconciler coordina el ciclo de reconciliación de los recursos NodeTask mediante sus dependencias inyectadas (Cliente de Kubernetes, Scheme, Resolver, Registry y Runner).
// Se instancia en main(), se registra con controller-runtime para NodeTask y ejecuta Reconcile() ante cada evento del recurso.
// Agrupa solo las dependencias necesarias para orquestar el flujo sin implementar directamente las consultas de nodos, las reglas de cada operación ni la ejecución del comando externo.
type NodeTaskReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Resolver k8s.NodeResolver
	Registry *operations.Registry
	Runner   runner.Runner
}

// Reconcile atiende los eventos del recurso NodeTask: obtiene el objeto, valida la operación y sus opciones, resuelve nodos y ejecuta Ansible.
// Escribe registros estructurados en los logs del controlador para seguir cada etapa y devuelve error solo cuando conviene reintentar en fallos transitorios.
func (r *NodeTaskReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := log.FromContext(ctx)

	var task k8s.NodeTask
	if err := r.Get(ctx, req.NamespacedName, &task); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	l.Info("reconciling node task", "name", task.Name, "operation", task.Spec.Operation, "options", task.Spec.Options, "dryRun", task.Spec.DryRun)

	handler, err := r.Registry.Get(task.Spec.Operation)
	if err != nil {
		l.Error(err, "invalid operation")
		return ctrl.Result{}, nil
	}

	opCmd, err := handler.Build(task.Spec.Options)
	if err != nil {
		l.Error(err, "invalid operation options")
		return ctrl.Result{}, nil
	}

	hosts, err := r.Resolver.Resolve(ctx, task.Spec.NodeSelector)
	if err != nil {
		l.Error(err, "failed resolving target nodes")
		return ctrl.Result{}, err
	}
	if len(hosts) == 0 {
		l.Info("no matching nodes found", "selector", task.Spec.NodeSelector)
		return ctrl.Result{}, nil
	}

	l.Info("executing ansible operation", "hosts", hosts, "module", opCmd.Module, "args", opCmd.Args)

	output, err := r.Runner.Run(ctx, hosts, opCmd, task.Spec.DryRun)
	l.Info("ansible output", "output", output)
	if err != nil {
		l.Error(err, "ansible execution failed")
		return ctrl.Result{}, err
	}

	l.Info("node task completed", "name", task.Name, "operation", task.Spec.Operation)
	return ctrl.Result{}, nil
}
