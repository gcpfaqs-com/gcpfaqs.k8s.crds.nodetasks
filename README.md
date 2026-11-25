# `node-task` Operator (`gcpfaqs.k8s.crds.nodetasks`)

Operador de Kubernetes en Go basado en `controller-runtime` que observa recursos personalizados `NodeTask` (`ops.ejemplo.local/v1alpha1`), resuelve nodos objetivo por etiquetas (`nodeSelector`), construye comandos normalizados según la operación solicitada (`system.service.cron`, `system.update`, `node.test`) y los ejecuta de forma remota mediante `ansible` por SSH.

---

## 1. Arquitectura y estructura del proyecto

El proyecto separa la definición y resolución de recursos de Kubernetes, el despacho y construcción de operaciones, el transporte con Ansible y la reconciliación:

```text
node-task/
├── go.mod
├── cmd/
│   └── main.go
├── internal/
│   ├── controller/
│   │   └── reconciler.go
│   ├── k8s/
│   │   ├── nodetask_types.go
│   │   └── node_resolver.go
│   ├── operations/
│   │   ├── factory.go
│   │   ├── cron.go
│   │   ├── update.go
│   │   └── node_test.go
│   └── runner/
│       └── ansible_runner.go
└── manifests/
    ├── nodetask-crd.yaml
    ├── test-cron-dryrun.yaml
    ├── test-update-dryrun.yaml
    └── test-node-syslog.yaml

```

### Responsabilidad de cada paquete y fichero

* **`cmd/main.go`:** Registra `NodeTask` y `NodeTaskList` en el `runtime.Scheme`, crea el `Manager` de `controller-runtime`, instancia dependencias (`KubeNodeResolver`, `operations.Registry` con los manejadores registrados y `AnsibleRunner`) e inicia el bucle de reconciliación.
* **`internal/k8s/nodetask_types.go`:** Define `GroupVersion` (`ops.ejemplo.local/v1alpha1`), `NodeTaskSpec` (`nodeSelector`, `operation`, `options`, `dryRun`), `NodeTaskStatus` (`phase`, `message`), `NodeTask`, `NodeTaskList` y sus implementaciones de `DeepCopyObject()`.
* **`internal/k8s/node_resolver.go`:** Define `NodeResolver` e implementa `KubeNodeResolver`, que lista los `Node` filtrando por `client.MatchingLabels(selector)` y extrae la IP (`NodeInternalIP` o `NodeExternalIP`) de cada nodo.
* **`internal/operations/factory.go`:** Define `Command` (`Module`, `Args`, `Become`), la interfaz `Handler` (`Name()`, `Build(options)`) y `Registry` (mapa indexado por nombre de operación con `NewRegistry` y `Get`).
* **`internal/operations/cron.go`:** Implementa `CronServiceHandler` para la operación `system.service.cron`, validando `options["action"]` (`restart`, `reload`, `start`, `stop`) y generando la llamada a `ansible.builtin.service` con `Become: true`.
* **`internal/operations/update.go`:** Implementa `SystemUpdateHandler` para `system.update` usando `ansible.builtin.apt` (`upgrade=dist update_cache=yes`) con `Become: true`.
* **`internal/operations/node_test.go`:** Implementa `NodeTestHandler` para `node.test`. No admite opciones (`len(options) > 0` devuelve error) y ejecuta con `Become: true` el comando `echo "hello crd" > /var/log/syslog` mediante `ansible.builtin.shell`.
* **`internal/runner/ansible_runner.go`:** Define `Runner` e implementa `AnsibleRunner`. Crea un inventario temporal en disco, añade `--become` si `cmd.Become` es `true`, añade `--check` si `dryRun` es `true`, ejecuta `ansible` y borra el inventario temporal al terminar.
* **`internal/controller/reconciler.go`:** Implementa `NodeTaskReconciler.Reconcile()` para leer el `NodeTask`, obtener el `Handler` en `Registry`, construir `Command`, resolver nodos, ejecutar `AnsibleRunner.Run()` y registrar la salida en los logs.

---

## 2. Código de la operación `node.test` y registro en `main.go`

### `internal/operations/node_test.go`

```go
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

```

### Registro en `cmd/main.go`

```go
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

```

---

## 3. Requisitos previos en la máquina local y en el clúster MicroK8s

1. **Módulo Go:** Nombre de módulo `gcpfaqs.k8s.crds.nodetasks` en `go.mod` y dependencias descargadas.
2. **Binario `ansible` y clave SSH:** `ansible` instalado en el `PATH` de la máquina donde ejecutas el controlador, clave privada accesible (por ejemplo `/etc/ssh/id_rsa` con permisos `600`) y acceso SSH con el usuario configurado (`ubuntu`) hacia los nodos del clúster.
3. **Acceso de red a las IPs de los nodos:** `KubeNodeResolver` toma la primera dirección `NodeInternalIP` o `NodeExternalIP` de cada `Node`. La máquina que ejecuta `ansible` debe poder alcanzar por SSH esas IPs.
4. **Clúster MicroK8s remoto:** `KUBECONFIG` apuntando al clúster MicroK8s con `server:` configurado a la IP accesible de la máquina remota.

---

## 4. Poner en marcha paso a paso

### Paso 1: inicializar módulo, descargar dependencias y compilar

```bash
cd ~/development/gcpfaqs-k8s-crds/node-task
go mod init gcpfaqs.k8s.crds.nodetasks
go mod tidy
go build ./...

```

### Paso 2: verificar `ansible`, clave SSH y conectividad con el nodo

```bash
ansible --version
ls -la /etc/ssh/id_rsa
chmod 600 /etc/ssh/id_rsa
ssh -i /etc/ssh/id_rsa ubuntu@<IP_DEL_NODO>

```

### Paso 3: configurar `KUBECONFIG` hacia MicroK8s

En la máquina remota de MicroK8s:

```bash
microk8s config > ~/microk8s-config.yaml

```

Copia el archivo a tu máquina local (por ejemplo `~/.kube/microk8s.yaml`), verifica que `server:` apunta a la IP de la máquina remota y exporta la variable:

```bash
export KUBECONFIG=~/.kube/microk8s.yaml
kubectl get nodes --show-labels

```

### Paso 4: crear y aplicar el CRD `NodeTask`

Crea `manifests/nodetask-crd.yaml`:

```yaml
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: nodetasks.ops.ejemplo.local
spec:
  group: ops.ejemplo.local
  names:
    kind: NodeTask
    listKind: NodeTaskList
    plural: nodetasks
    singular: nodetask
  scope: Namespaced
  versions:
    - name: v1alpha1
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object
          properties:
            spec:
              type: object
              required:
                - operation
              properties:
                nodeSelector:
                  type: object
                  additionalProperties:
                    type: string
                operation:
                  type: string
                options:
                  type: object
                  additionalProperties:
                    type: string
                dryRun:
                  type: boolean
            status:
              type: object
              properties:
                phase:
                  type: string
                message:
                  type: string

```

Aplica el CRD y comprueba que existe:

```bash
kubectl apply -f manifests/nodetask-crd.yaml
kubectl get crd nodetasks.ops.ejemplo.local

```

### Paso 5: etiquetar un nodo de prueba

```bash
kubectl label node <nombre-del-nodo> task-test=true

```

### Paso 6: arrancar el controlador con `go run`

Con `KUBECONFIG` exportado:

```bash
go run ./cmd/main.go

```

Mantén esta terminal abierta para ver los logs estructurados (`reconciling node task`, `executing ansible operation`, `ansible output`, `node task completed`).

---

## 5. Ejemplos y pruebas de `NodeTask`

Empieza siempre con `dryRun: true` para validar selección de nodos, SSH y construcción de comandos sin modificar el sistema.

### Ejemplo 1: `node.test` en modo `dryRun: true` y luego ejecución real (`dryRun: false`)

Crea `manifests/test-node-syslog.yaml`:

```yaml
apiVersion: ops.ejemplo.local/v1alpha1
kind: NodeTask
metadata:
  name: test-node-syslog
  namespace: default
spec:
  nodeSelector:
    task-test: "true"
  operation: node.test
  dryRun: true

```

Aplica y revisa los logs:

```bash
kubectl apply -f manifests/test-node-syslog.yaml

```

Para ejecutarlo realmente en el nodo, cambia `dryRun: false` en `manifests/test-node-syslog.yaml` y vuelve a aplicarlo:

```bash
kubectl apply -f manifests/test-node-syslog.yaml

```

*(Nota: `>` sobrescribe `/var/log/syslog` cuando `dryRun: false`; usa `>>` si prefieres anexar al final. En sistemas con `systemd-journald` sin `rsyslog`, revisa si el fichero existe o consulta `ansible output` en los logs del controlador.)*

### Ejemplo 2: `system.service.cron` con opción `action`

Crea `manifests/test-cron-dryrun.yaml`:

```yaml
apiVersion: ops.ejemplo.local/v1alpha1
kind: NodeTask
metadata:
  name: test-cron-dryrun
  namespace: default
spec:
  nodeSelector:
    task-test: "true"
  operation: system.service.cron
  options:
    action: restart
  dryRun: true

```

Aplícalo con:

```bash
kubectl apply -f manifests/test-cron-dryrun.yaml

```

### Ejemplo 3: `system.update` en modo simulación

Crea `manifests/test-update-dryrun.yaml`:

```yaml
apiVersion: ops.ejemplo.local/v1alpha1
kind: NodeTask
metadata:
  name: test-update-dryrun
  namespace: default
spec:
  nodeSelector:
    task-test: "true"
  operation: system.update
  dryRun: true

```

Aplícalo con:

```bash
kubectl apply -f manifests/test-update-dryrun.yaml

```

---

## 6. Limpieza tras las pruebas

```bash
kubectl delete nodetask test-node-syslog test-cron-dryrun test-update-dryrun --ignore-not-found
kubectl label node <nombre-del-nodo> task-test-

```

Detén el proceso `go run ./cmd/main.go` con `Ctrl+C`.