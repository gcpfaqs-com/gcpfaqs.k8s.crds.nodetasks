// NodeTask CRD y controlador en Go para ejecutar operaciones idempotentes sobre nodos de Kubernetes mediante Ansible.
//
// Qué hace este CRD:
// Permite declarar en un recurso NodeTask qué operación ejecutar (spec.operation), sobre qué nodos actuar
// (spec.nodeSelector), qué parámetros usar (spec.options) y si simular sin aplicar cambios (spec.dryRun).
// El controlador observa los recursos NodeTask, busca los nodos coincidentes en el clúster, resuelve la operación
// mediante manejadores registrados en Go, ejecuta el comando con Ansible y escribe el resultado en los logs del pod.
package main

import (
	"os"

	"gcpfaqs.k8s.crds.nodetasks/internal/controller"
	"gcpfaqs.k8s.crds.nodetasks/internal/k8s"
	"gcpfaqs.k8s.crds.nodetasks/internal/operations"
	"gcpfaqs.k8s.crds.nodetasks/internal/runner"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
)

// main inicializa el esquema de Kubernetes, registra NodeTask y NodeTaskList, crea el Manager del controlador, registra los manejadores de operaciones y arranca el bucle de reconciliación.
// Inyecta las dependencias concretas (resolvedor de nodos, registro de operaciones y ejecutor de Ansible).
func main() {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	scheme.AddKnownTypes(k8s.GroupVersion, &k8s.NodeTask{}, &k8s.NodeTaskList{})
	metav1.AddToGroupVersion(scheme, k8s.GroupVersion)

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{Scheme: scheme})
	if err != nil {
		os.Exit(1)
	}

	reconciler := &controller.NodeTaskReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Resolver: k8s.NewKubeNodeResolver(mgr.GetClient()),
		Registry: operations.NewRegistry(
			&operations.CronServiceHandler{},
			&operations.SystemUpdateHandler{},
			&operations.NodeTestHandler{},
		),
		Runner: runner.NewAnsibleRunner("ubuntu", "/etc/ssh/id_rsa"),
	}

	if err := ctrl.NewControllerManagedBy(mgr).
		For(&k8s.NodeTask{}).
		Complete(reconciler); err != nil {
		os.Exit(1)
	}

	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		os.Exit(1)
	}
}
