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

### Discover (Existing Clusters)

Discover HostedClusters that were created outside of Maestro so they appear in `hcp list clusters`. The discover command creates a **ReadOnly** ManifestWork -- the Maestro agent observes the live resource and reports status without modifying it. This avoids SSA field ownership conflicts.

```bash
hcp discover cluster virt-hcp-2 \
  --namespace clusters \
  --maestro-server https://maestro.example.com \
  --maestro-consumer mce-1
```

| Flag | Description |
|------|-------------|
| `-n`, `--namespace` | Namespace of the HostedCluster on the consumer cluster (default: `clusters`) |

The discovered cluster will now appear when running `hcp list clusters` with full status (VERSION, PROGRESS, AVAILABLE, etc.). Since the ManifestWork uses ReadOnly strategy with Orphan delete policy, removing the discovery from Maestro will not affect the actual HostedCluster on the consumer cluster.

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

Get HostedCluster resource and status from Maestro. By default, displays a summary table matching the `oc get hostedcluster` format. Use `-o yaml` for the full YAML output.

```bash
# Table output (default)
hcp get cluster mycluster \
  --maestro-server https://maestro.example.com \
  --maestro-consumer my-management-cluster

# Full YAML output
hcp get cluster mycluster -o yaml \
  --maestro-server https://maestro.example.com \
  --maestro-consumer my-management-cluster
```

| Flag | Description |
|------|-------------|
| `-o`, `--output` | Output format: table (default) or yaml |

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

## Example Session

Below is a sample session showing create, list, get operations against a Maestro server.

### Create a KubeVirt HostedCluster

```bash
./hcp create cluster kubevirt \
  --name my-cluster \
  --target-cluster mgmt-cluster-1 \
  --maestro-server https://maestro.example.com \
  --maestro-consumer mgmt-cluster-1 \
  --maestro-insecure-skip-verify \
  --pull-secret /path/to/pull-secret.txt \
  --ssh-key ~/.ssh/id_rsa.pub \
  --node-pool-replicas 2 \
  --release-image quay.io/openshift-release-dev/ocp-release:4.21.3-multi \
  --memory 6Gi \
  --cores 2 \
  --infra-availability-policy SingleReplica \
  --control-plane-availability-policy SingleReplica
```

```
I0301 10:48:42.792150   12345 protocol.go:126] "subscribing events for source" source="mw-client-example" eventDataType="io.open-cluster-management.works.v1alpha1.manifestbundles"
I0301 10:48:43.381547   12345 protocol.go:93] "publishing event" messageID="a1b2c3d4-e5f6-7890-abcd-ef1234567890"
{"level":"info","ts":"2026-03-01T10:48:43-05:00","msg":"Applied manifests to Maestro","consumer":"mgmt-cluster-1","file":"mgmt-cluster-1.yaml"}
```

### Discover an existing HostedCluster

```bash
./hcp discover cluster existing-cluster \
  --namespace clusters \
  --maestro-server https://maestro.example.com \
  --maestro-consumer mgmt-cluster-1 \
  --maestro-insecure-skip-verify
```

```
Discovered HostedCluster clusters/existing-cluster from consumer mgmt-cluster-1 (read-only)
```

### List clusters for a specific consumer

```bash
./hcp list clusters \
  --maestro-server https://maestro.example.com \
  --maestro-consumer mgmt-cluster-1 \
  --maestro-insecure-skip-verify
```

```
NAMESPACE   NAME         VERSION   KUBECONFIG                    PROGRESS    AVAILABLE   PROGRESSING   MESSAGE
clusters    my-cluster   4.21.3    my-cluster-admin-kubeconfig   Completed   True        False         The hosted control plane is available
```

### List clusters across all consumers

```bash
./hcp list clusters \
  --maestro-server https://maestro.example.com \
  --maestro-insecure-skip-verify
```

```
NAMESPACE   NAME         CONSUMER         VERSION   KUBECONFIG                    PROGRESS    AVAILABLE   PROGRESSING   MESSAGE
clusters    my-cluster   mgmt-cluster-1   4.21.3    my-cluster-admin-kubeconfig   Completed   True        False         The hosted control plane is available
```

### Get cluster details (table)

```bash
./hcp get cluster my-cluster \
  --maestro-server https://maestro.example.com \
  --maestro-insecure-skip-verify \
  --maestro-consumer mgmt-cluster-1
```

```
NAMESPACE   NAME         VERSION   KUBECONFIG                    PROGRESS    AVAILABLE   PROGRESSING   MESSAGE
clusters    my-cluster   4.21.3    my-cluster-admin-kubeconfig   Completed   True        False         The hosted control plane is available
```

### Get cluster details (YAML)

```bash
./hcp get cluster my-cluster -o yaml \
  --maestro-server https://maestro.example.com \
  --maestro-insecure-skip-verify \
  --maestro-consumer mgmt-cluster-1
```

```yaml
---
# HostedCluster spec and status from Maestro ManifestWork
---
apiVersion: hypershift.openshift.io/v1beta1
kind: HostedCluster
metadata:
  name: my-cluster
  namespace: clusters
spec:
  autoscaling: {}
  capabilities: {}
  configuration: {}
  controllerAvailabilityPolicy: SingleReplica
  dns:
    baseDomain: ""
  etcd:
    managed:
      storage:
        persistentVolume:
          size: 8Gi
        type: PersistentVolume
    managementType: Managed
  fips: false
  infraID: my-cluster-ab1cd
  infrastructureAvailabilityPolicy: SingleReplica
  networking:
    clusterNetwork:
    - cidr: 10.132.0.0/14
    networkType: OVNKubernetes
    serviceNetwork:
    - cidr: 172.31.0.0/16
  olmCatalogPlacement: management
  platform:
    kubevirt:
      baseDomainPassthrough: true
    type: KubeVirt
  pullSecret:
    name: my-cluster-pull-secret
  release:
    image: quay.io/openshift-release-dev/ocp-release:4.21.3-multi
  secretEncryption:
    aescbc:
      activeKey:
        name: my-cluster-etcd-encryption-key
    type: aescbc
  services:
  - service: APIServer
    servicePublishingStrategy:
      type: LoadBalancer
  - service: Ignition
    servicePublishingStrategy:
      type: Route
  - service: Konnectivity
    servicePublishingStrategy:
      type: Route
  - service: OAuthServer
    servicePublishingStrategy:
      type: Route
```

---

## Building

```bash
make product-cli
```

The binary is built to `bin/hcp`.
