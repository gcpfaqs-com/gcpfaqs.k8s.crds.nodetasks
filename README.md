## 1. Cluster nodo master

Ejecuta estos pasos en el nodo master (por ejemplo, `prometheus` en `192.168.1.200`):

1. **Instala MicroK8s:**
```bash
sudo snap install microk8s --classic

```


*Verificación:* `microk8s status --wait-ready` muestra `microk8s is running`.
2. **Añade el usuario `markitos` al grupo `microk8s` y exporta su `kubeconfig`:**
```bash
sudo usermod -aG microk8s markitos
mkdir -p /home/markitos/.kube
sudo microk8s config > /home/markitos/.kube/config
sudo chown -R markitos:markitos /home/markitos/.kube
chmod 600 /home/markitos/.kube/config

```


*Verificación:* Cierra y abre sesión con `markitos` (o ejecuta `newgrp microk8s`) y comprueba `microk8s.kubectl get nodes` con estado `Ready`.
3. **Configura `KUBECONFIG` y el alias `kubectl` para las sesiones:**
```bash
echo 'export KUBECONFIG=/home/markitos/.kube/config' | sudo tee -a /etc/bash.bashrc
echo 'alias kubectl="microk8s.kubectl"' | sudo tee -a /etc/bash.bashrc
source /etc/bash.bashrc

```


*Verificación:* `kubectl get nodes` responde sin errores.
4. **Genera la clave SSH en `/etc/ssh/` del host donde correrá el operador y Ansible:**
```bash
sudo ssh-keygen -t ed25519 -C "ansible-gcp-lab" -f /etc/ssh/ansible_gcp_lab
sudo chown markitos:markitos /etc/ssh/ansible_gcp_lab /etc/ssh/ansible_gcp_lab.pub
sudo chmod 600 /etc/ssh/ansible_gcp_lab
sudo chmod 644 /etc/ssh/ansible_gcp_lab.pub

```


*Verificación:* `ls -la /etc/ssh/ansible_gcp_lab*` muestra `-rw-------` (`600`) en la clave privada y `-rw-r--r--` (`644`) en la pública, ambas con propietario `markitos:markitos`.

---

## 2. Cluster nodos worker

En cada nodo worker (y también en el master si el master va a ser objetivo de `NodeTask`), el operador conecta por SSH con el usuario `markitos` y usa `sudo` sin contraseña (`NOPASSWD`) para ejecutar tareas con `become: true`.

1. **Crea el usuario `markitos` y prepara su directorio `.ssh` en el nodo worker:**
```bash
sudo useradd -m -s /bin/bash markitos
sudo mkdir -p /home/markitos/.ssh
sudo chown -R markitos:markitos /home/markitos/.ssh
sudo chmod 700 /home/markitos/.ssh

```


*Verificación:* `id markitos` y `ls -ld /home/markitos/.ssh` confirman el usuario y los permisos `drwx------` (`700`).
2. **Configura `NOPASSWD` en `sudoers` para `markitos` en el nodo worker:**
```bash
echo "markitos ALL=(ALL) NOPASSWD:ALL" | sudo tee /etc/sudoers.d/markitos
sudo chmod 0440 /etc/sudoers.d/markitos
sudo visudo -cf /etc/sudoers.d/markitos

```


*Verificación:* `sudo -n true` como `markitos` sale con código `0` sin pedir contraseña.
3. **Copia la clave pública `/etc/ssh/ansible_gcp_lab.pub` desde el host del operador al worker:**
```bash
ssh-copy-id -i /etc/ssh/ansible_gcp_lab.pub markitos@<IP_WORKER>

```


*Verificación:* Desde el host donde está `/etc/ssh/ansible_gcp_lab`, ejecuta:
```bash
ssh -i /etc/ssh/ansible_gcp_lab markitos@<IP_WORKER> 'sudo -n id'

```


Debe devolver `uid=0(root)` sin pedir contraseña.
4. **Une el nodo worker al clúster MicroK8s (si va a formar parte del clúster de Kubernetes):**
En el master, genera el comando de unión:
```bash
microk8s add-node

```


En el worker (tras instalar MicroK8s con `sudo snap install microk8s --classic`), ejecuta el comando `microk8s join ...` que devolvió el master.
*Verificación:* En el master, `kubectl get nodes` muestra el worker en estado `Ready`.
5. **Etiqueta los nodos que quieres que coincidan con `spec.nodeSelector`:**
```bash
kubectl label node <NOMBRE_NODO> task-test=true

```


*Verificación:* `kubectl get nodes --show-labels` muestra `task-test=true` en los nodos seleccionados.

---

## 3. Instalación del CRD

Asegúrate de que el `GroupVersion` en Go (`gcpfaqs.k8s.crds.nodetasks` y `v1alpha1`) coincide con `metadata.name`, `spec.group` y `versions[0].name` del CRD (`manifests/nodetask-crd.yaml`):

```yaml
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: nodetasks.gcpfaqs.k8s.crds.nodetasks
spec:
  group: gcpfaqs.k8s.crds.nodetasks
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

Aplica el CRD:

```bash
kubectl apply -f manifests/nodetask-crd.yaml

```

*Verificación:* `kubectl get crd nodetasks.gcpfaqs.k8s.crds.nodetasks` devuelve el CRD registrado.

---

## 4. Creación del recurso de test

Crea el manifiesto `etc/test-node-syslog.yaml` empezando con `dryRun: true` para validar la selección de nodos y la ejecución de Ansible antes de escribir en `/var/log/syslog`:

```yaml
apiVersion: gcpfaqs.k8s.crds.nodetasks/v1alpha1
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

Aplica el recurso:

```bash
kubectl apply -f etc/test-node-syslog.yaml

```

*Verificación:* `kubectl get nodetask test-node-syslog` muestra el recurso creado en el namespace `default`.

---

## 5. Instalación y ejecución del operador

1. **Configura en `cmd/main.go` el logger y el `AnsibleRunner` con `/etc/ssh/ansible_gcp_lab`:**
```go
import (
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func main() {
	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	// ...
	Runner: runner.NewAnsibleRunner("markitos", "/etc/ssh/ansible_gcp_lab"),
	// ...
}

```


*Verificación:* Al arrancar el operador, se muestran los logs de `controller-runtime` y del reconciliador.
2. **Ejecuta el operador desde el directorio del proyecto:**
```bash
cd /home/markitos/development/gcpfaqs-k8s-crds/nodetasks
go run cmd/main.go

```


*Verificación:* Con `dryRun: true`, en los logs ves `reconciling node task`, `executing ansible operation`, `SKIPPED` por `--check` y `node task completed`.
3. **Pasa a ejecución real cambiando `dryRun: false` y reaplicando el manifiesto:**
Edita `etc/test-node-syslog.yaml` con `dryRun: false` y ejecuta:
```bash
kubectl apply -f etc/test-node-syslog.yaml

```


*Verificación:* En los logs del operador ves la ejecución completada y en el nodo destino compruebas `/var/log/syslog` con `tail -n 5 /var/log/syslog` (recuerda que `>` sobrescribe el archivo; usa `>>` en `internal/operations/node_test.go` si prefieres anexar).

---

## Detalles extra útiles para el laboratorio

* **Limpiar o repetir pruebas:** Si quieres volver a disparar una reconciliación limpia o cambiar de etiquetas, puedes borrar y recrear el objeto con `kubectl delete -f etc/test-node-syslog.yaml && kubectl apply -f etc/test-node-syslog.yaml`, o cambiar `spec.nodeSelector` y la etiqueta del nodo con `kubectl label node <NOMBRE_NODO> task-test=false --overwrite`.
* **Evitar `spec.nodeSelector` vacío:** No dejes `nodeSelector: {}` en operaciones que escriben archivos o modifican sistemas, ya que seleccionaría todos los nodos del clúster.
* **Desplegar como deployment dentro del clúster (opcional):** Cuando quieras dejarlo corriendo como pod en vez de `go run cmd/main.go`, construye la imagen con un tag de versión (`v*`), súbela a tu registry y monta `/etc/ssh/ansible_gcp_lab` mediante un Secret o volumen en el Pod del controlador con RBAC para `nodes` (list/get/watch) y `nodetasks` (get/list/watch/update/patch).