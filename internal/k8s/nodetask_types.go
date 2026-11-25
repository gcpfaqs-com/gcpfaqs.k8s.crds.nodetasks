package k8s

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// GroupVersion identifica el grupo API y la versión del recurso NodeTask.
var GroupVersion = schema.GroupVersion{Group: "gcpfaqs.k8s.crds.nodetasks", Version: "v1alpha1"}

// NodeTaskSpec define la especificación deseada del recurso NodeTask que el usuario escribe en el YAML (campo "spec").
// Se usa dentro de NodeTask para indicar sobre qué nodos actuar (NodeSelector), qué operación ejecutar (Operation), qué parámetros pasar (Options) y si ejecutar en modo simulación sin aplicar cambios (DryRun).
type NodeTaskSpec struct {
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
	Operation    string            `json:"operation"`
	Options      map[string]string `json:"options,omitempty"`
	DryRun       bool              `json:"dryRun,omitempty"`
}

// NodeTaskStatus representa el estado observado del recurso NodeTask en el clúster (campo "status").
// Se usa dentro de NodeTask para registrar la fase actual (Phase) y un mensaje descriptivo (Message) sobre el resultado o avance de la reconciliación.
type NodeTaskStatus struct {
	Phase   string `json:"phase,omitempty"`
	Message string `json:"message,omitempty"`
}

// NodeTask representa el recurso personalizado (Custom Resource) en código Go, combinando TypeMeta, ObjectMeta, Spec y Status.
// Se usa en main() al registrar el controlador con For(&NodeTask{}) y en NodeTaskReconciler.Reconcile() para leer el objeto que disparó el evento.
// Embeber metav1.TypeMeta aporta GetObjectKind() para cumplir con runtime.Object y client.Object.
type NodeTask struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              NodeTaskSpec   `json:"spec,omitempty"`
	Status            NodeTaskStatus `json:"status,omitempty"`
}

// NodeTaskList representa una colección de objetos NodeTask exigida por controller-runtime al registrar el tipo en el Scheme.
// Se usa internamente en el cliente y la caché de controller-runtime al listar o observar recursos NodeTask.
type NodeTaskList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NodeTask `json:"items"`
}

// DeepCopyObject cumple con la interfaz runtime.Object exigida por controller-runtime para copiar el recurso en memoria sin mutar el original.
// Copia de forma independiente los mapas de Spec para evitar compartir referencias mutables entre instancias.
func (in *NodeTask) DeepCopyObject() runtime.Object {
	out := *in
	if in.Spec.NodeSelector != nil {
		out.Spec.NodeSelector = make(map[string]string, len(in.Spec.NodeSelector))
		for k, v := range in.Spec.NodeSelector {
			out.Spec.NodeSelector[k] = v
		}
	}
	if in.Spec.Options != nil {
		out.Spec.Options = make(map[string]string, len(in.Spec.Options))
		for k, v := range in.Spec.Options {
			out.Spec.Options[k] = v
		}
	}
	return &out
}

// DeepCopyObject cumple con la interfaz runtime.Object para NodeTaskList copiando el slice de elementos.
// Garantiza que las copias en caché o en llamadas al cliente no alteren el slice original.
func (in *NodeTaskList) DeepCopyObject() runtime.Object {
	out := *in
	if in.Items != nil {
		out.Items = make([]NodeTask, len(in.Items))
		for i := range in.Items {
			copied := in.Items[i].DeepCopyObject().(*NodeTask)
			out.Items[i] = *copied
		}
	}
	return &out
}
