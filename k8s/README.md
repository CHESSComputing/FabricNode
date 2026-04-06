# Fabric Node Helm Chart

This Helm chart deploys a **CHESS Federated Knowledge Fabric Node** into a Kubernetes cluster.
It includes four core services:

* **catalog** – dataset discovery and VoID/PROF/SHACL endpoints
* **data** – SPARQL / triple store service
* **identity** – DID / Verifiable Credential service
* **notifications** – Linked Data Notifications (LDN) inbox

---

# Chart Structure

```
```
fabric-node/
├── Chart.yaml
├── values.yaml
├── templates/
│   ├── catalog-deployment.yaml
│   ├── data-deployment.yaml
│   ├── identity-deployment.yaml
│   ├── notifications-deployment.yaml
│   ├── services.yaml
│   └── secret.yaml
```

---

## Chart.yaml

Defines chart metadata:

* `name` – chart name
* `version` – chart version
* `appVersion` – application version

Helm **requires this file** and expects the exact name `Chart.yaml`.

---

## values.yaml

Central configuration file used to customize deployment.

Contains:

* Node identity (`node.id`, `node.name`)
* Container images and tags
* Service ports
* Shared `fabric.yaml` config

This is the **main file you will modify** before deployment.

---

## templates/

Contains Kubernetes manifests rendered by Helm.

---

### Deployments

Each service has its own Deployment:

* `catalog-deployment.yaml`
* `data-deployment.yaml`
* `identity-deployment.yaml`
* `notifications-deployment.yaml`

Each Deployment defines:

* container image
* environment variables
* ports
* health checks (`/health`)
* mounted configuration (from Secret)

---

### services.yaml

Defines Kubernetes Services for internal communication:

| Service       | Port |
| ------------- | ---- |
| catalog       | 8081 |
| data          | 8082 |
| identity      | 8083 |
| notifications | 8084 |

These enable DNS-based access inside the cluster:

```
http://catalog:8081
http://data:8082
```

---

### secret.yaml

Creates a Kubernetes Secret containing:

```
fabric.yaml
```

This file is mounted into all services at:

```
/config/fabric.yaml
```

Use this for shared configuration across services.

---

# Configuration

## Update images

Edit `values.yaml`:

```
image:
  catalog: your-registry/catalog:tag
  data: your-registry/data:tag
  identity: your-registry/identity:tag
  notifications: your-registry/notifications:tag
```

---

## Update node identity

```
node:
  id: chess-node
  name: "CHESS Federated Knowledge Fabric Node"
```

---

## Provide fabric.yaml

Paste your configuration into:

```
fabricConfig: |
  your: config
  goes: here
```

---

## Change ports (optional)

```
ports:
  catalog: 8081
  data: 8082
  identity: 8083
  notifications: 8084
```

---

# Deployment

## 1. Extract chart

```
tar -xzf fabric-node-helm-chart.tar.gz
cd fabric-node
```

---

## 2. Customize configuration

Edit:

```
values.yaml
```

---

## 3. Install chart

```
helm install fabric-node .
```

---

## 4. Verify deployment

```
kubectl get pods
kubectl get svc
```

---

## 5. Check logs

```
kubectl logs -l app=catalog
kubectl logs -l app=identity
```

---

# Upgrade

After changing `values.yaml`:

```
helm upgrade fabric-node .
```

---

# Uninstall

```
helm uninstall fabric-node
```

---

# Service Access

Inside cluster:

```
catalog        → http://catalog:8081
data           → http://data:8082
identity       → http://identity:8083
notifications  → http://notifications:8084
```

---

# Notes

* `localhost` **must not be used** inside Kubernetes
* services communicate via **DNS names**
* `depends_on` from Docker Compose is replaced by:

  * readiness probes
  * Kubernetes scheduling

---

# Future Improvements

You may want to extend this chart with:

* Ingress (external access)
* TLS via cert-manager
* Horizontal Pod Autoscaling (HPA)
* Persistent storage for data service
* Resource limits/requests

---

# Summary

This Helm chart provides:

* Modular deployment of Fabric Node services
* Shared configuration via Secret
* Internal service discovery via Kubernetes DNS
* Easy customization via `values.yaml`
