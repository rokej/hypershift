# hcp - Hypershift Product CLI

The `hcp` CLI provides commands for creating, managing, and destroying HostedClusters and NodePools.

## Maestro Integration

When used with [Maestro](https://github.com/openshift-online/maestro), the hcp CLI allows users to create, update, list, get, and delete hosted clusters from a central ACM hub without direct access to management clusters. Maestro transports Kubernetes resources (ManifestWorks) to target clusters via CloudEvents and relays status back.

### Prerequisites

- Maestro server and agent deployed (e.g., on the ACM hub)
- Port-forward Maestro gRPC for local access:
  ```bash
  oc port-forward svc/maestro-grpc 8090:8090 -n maestro
  ```

### Shared Maestro Flags

| Flag | Description |
|------|-------------|
| `--maestro-server` | Maestro HTTP API URL (required for Maestro operations) |
| `--maestro-grpc-server` | Maestro gRPC server address (default: 127.0.0.1:8090) |
| `--maestro-consumer` | Maestro consumer name (target cluster / management cluster name) |
| `--maestro-insecure-skip-verify` | Skip TLS verification for Maestro HTTP API |

---

### Create

Create a hosted cluster and apply manifests to Maestro. Supported on KubeVirt platform with `--target-cluster` and `--maestro-server`.

```bash
hcp create cluster kubevirt \
  --name mycluster \
  --target-cluster my-management-cluster \
  --maestro-server https://maestro.example.com \
  --maestro-consumer my-management-cluster
```

- `--target-cluster` enables render mode and saves manifests to `<target-cluster>.yaml`
- When `--maestro-server` is set, the rendered manifests are applied to Maestro as a ManifestWork
- `--maestro-consumer` defaults to `--target-cluster` when not specified

---

### Import (Discover Existing Clusters)

Import HostedClusters that were created outside of Maestro so they appear in `hcp list clusters`:

```bash
# 1. Port-forward Maestro gRPC
oc port-forward svc/maestro-grpc 8090:8090 -n maestro &

# 2. Export the HostedCluster from the consumer cluster (mce-1)
oc get hostedcluster virt-hcp-1 -n clusters -o yaml > virt-hcp-1.yaml

# 3. Apply to Maestro (optionally edit to remove status, resourceVersion, uid)
hcp apply manifests virt-hcp-1.yaml \
  --maestro-server https://maestro.example.com \
  --maestro-consumer mce-1
```

The imported cluster will now appear when running `hcp list clusters`.

---

### Update

Update an existing HostedCluster by re-applying modified manifests or by patching specific fields. See **Patch** for field-level updates, or use `hcp apply manifests <file>` to re-apply a full YAML file.

---

### Patch

Patch specific fields in the HostedCluster spec without re-applying the full manifest. Uses RFC 7396 JSON merge patch format. Only the HostedCluster in the ManifestWork is modified; other resources (Secrets, etc.) are unchanged.

```bash
# Patch node pool replicas
hcp patch cluster mycluster \
  --patch '{"spec":{"nodePools":[{"name":"nodepool-1","replicas":3}]}}' \
  --maestro-server https://maestro.example.com \
  --maestro-consumer my-management-cluster

# Patch from file
hcp patch cluster mycluster --patch-file patch.json \
  --maestro-server https://maestro.example.com \
  --maestro-consumer my-management-cluster
```

| Flag | Description |
|------|-------------|
| `--patch` | JSON merge patch to apply to HostedCluster (inline) |
| `--patch-file` | Path to file containing JSON merge patch |

Exactly one of `--patch` or `--patch-file` is required.

---

### List

List hosted clusters from Maestro (single consumer or all consumers).

```bash
# List from a specific consumer
hcp list clusters \
  --maestro-server https://maestro.example.com \
  --maestro-consumer my-management-cluster

# List from all consumers
hcp list clusters --maestro-server https://maestro.example.com
```

---

### Get

Get HostedCluster resource and status from Maestro.

```bash
hcp get cluster mycluster \
  --maestro-server https://maestro.example.com \
  --maestro-consumer my-management-cluster
```

---

### Destroy

Delete a hosted cluster by removing its ManifestWork from Maestro. Supported on all platforms (KubeVirt, AWS, Azure, Agent, OpenStack).

```bash
hcp destroy cluster kubevirt \
  --name mycluster \
  --maestro-server https://maestro.example.com \
  --maestro-consumer my-management-cluster
```

When `--maestro-server` is set, the destroy command deletes the ManifestWork from Maestro instead of applying directly to the management cluster.

---

## Building

```bash
make product-cli
```

The binary is built to `bin/hcp`.
