package k8s

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NodeResolver define el contrato para obtener las direcciones IP de los nodos que cumplen un selector de etiquetas.
type NodeResolver interface {
	Resolve(ctx context.Context, selector map[string]string) ([]string, error)
}

// KubeNodeResolver es la implementación concreta de NodeResolver que consulta los nodos reales del clúster de Kubernetes.
// Se instancia en main() y se inyecta en NodeTaskReconciler para obtener las direcciones IP de los nodos que cumplen el nodeSelector.
// Existe como struct separado para aislar las consultas a la API de Kubernetes de la lógica de reconciliación y de ejecución de Ansible.
type KubeNodeResolver struct {
	client client.Reader
}

// NewKubeNodeResolver construye un resolvedor de nodos de Kubernetes inyectando un lector de cliente (client.Reader).
// Permite desacoplar el reconciliador de cómo se obtienen los nodos y facilita pruebas unitarias sustituyendo esta dependencia.
func NewKubeNodeResolver(c client.Reader) *KubeNodeResolver {
	return &KubeNodeResolver{client: c}
}

// Resolve busca los nodos del clúster que coinciden con las etiquetas del selector y extrae su dirección IP para que Ansible pueda conectarse.
// Recorre las direcciones de cada Node y prioriza NodeInternalIP y, si no existe, NodeExternalIP.
func (r *KubeNodeResolver) Resolve(ctx context.Context, selector map[string]string) ([]string, error) {
	var nodes corev1.NodeList
	if err := r.client.List(ctx, &nodes, client.MatchingLabels(selector)); err != nil {
		return nil, err
	}
	var targets []string
	for _, node := range nodes.Items {
		for _, addr := range node.Status.Addresses {
			if addr.Type == corev1.NodeInternalIP || addr.Type == corev1.NodeExternalIP {
				targets = append(targets, addr.Address)
				break
			}
		}
	}
	return targets, nil
}
