# Lesson 2 — Local environment

Resolves [Set up the local Kubernetes environment](https://github.com/hassanshabbirahmed/fluke-operator/issues/5).

## What's installed

| Tool | Version | Purpose |
|---|---|---|
| Go | 1.27.1 | already installed in Lesson 1 |
| Docker CLI | 29.8.0 (client) | Colima's default runtime needs the `docker` client binary; Colima supplies the daemon inside its VM, no Docker Desktop involved |
| Colima | 0.10.3 | runs a Linux VM (via macOS's `Virtualization.framework`) that hosts the container runtime `kind` needs |
| kind | v0.33.0 | runs Kubernetes nodes as containers inside that VM |
| Kubebuilder | v4.15.0 | scaffolds the operator (Lesson 3 onward) |

All installed via Homebrew (`brew install colima kind kubebuilder docker`).

## Why Colima, not Docker Desktop

Colima is open-source with no licensing question; it runs an equivalent Linux
VM and speaks the same Docker API, so `kind` doesn't know the difference.

## Starting the VM

```bash
colima start --cpu 4 --memory 8 --disk 60
```

Sizing rationale (from the DRA research, [issue #3](https://github.com/hassanshabbirahmed/fluke-operator/issues/3)):
the operator, a 3-node cluster, and later the DRA driver's images all share
this VM's resources, so it's given real headroom rather than Colima's default.
Runs on macOS's built-in `Virtualization.framework` — no separate hypervisor
to install, and it's fully native on Apple Silicon.

## Why Kubernetes ≥ 1.34, and why no feature gates

Dynamic Resource Allocation went **GA as `resource.k8s.io/v1` in Kubernetes
1.34** ([issue #3](https://github.com/hassanshabbirahmed/fluke-operator/issues/3)).
On a 1.34+ node image, DRA's core API types (`DeviceClass`, `ResourceClaim`,
`ResourceClaimTemplate`, `ResourceSlice` — see `CONTEXT.md`) are served with
**no feature gate and no `--runtime-config` needed**. The only thing the
cluster needs beyond a recent node image is CDI support enabled in
containerd, which is how a driver injects an allocated device into a
container.

## The cluster config — `hack/kind-config.yaml`

```yaml
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
containerdConfigPatches:
  - |-
    [plugins."io.containerd.grpc.v1.cri"]
      enable_cdi = true
nodes:
  - role: control-plane
  - role: worker
  - role: worker
```

- `containerdConfigPatches` turns on CDI (Container Device Interface) — the
  mechanism a DRA driver uses to hand a container an allocated device.
- Three nodes (1 control-plane, 2 workers) so there's more than one place for
  the scheduler to actually make a placement decision once DRA is exercised
  for real, in Phase 3.

## Creating the cluster

```bash
kind create cluster --name fluke --config hack/kind-config.yaml \
  --image kindest/node:v1.36.4@sha256:099e049362a1526b2db71494e1947aae99bd16290d7c895f2b7ea312e3cbfaed
```

The image is pinned by digest, not just a tag — kind's release notes are the
source of truth for which digest goes with which kind version
(`gh api repos/kubernetes-sigs/kind/releases/tags/v0.33.0 --jq .body`). `v1.36.4`
was chosen to track the locally-installed `kubectl` (v1.36.3) closely, while
comfortably clearing the 1.34 DRA-GA line. **Never run `kind build node-image`**
for this project — it requires actual Docker (not Colima's runtime) and builds
Kubernetes from source; the stock prebuilt image already has everything
needed.

## Apple Silicon caveats hit

- `kindest/node` images are multi-arch (amd64 + arm64); the pinned digest
  above resolved to a native `linux/arm64` image with no emulation.
- Colima's default runtime (`docker`) requires the `docker` **client** binary
  to exist locally, even though Colima supplies the daemon — `brew install
  docker` installs just the CLI, no separate daemon or Docker Desktop.

## Verification

```bash
kubectl get nodes                          # 3 nodes, all Ready
kubectl api-resources | grep resource.k8s.io
kubectl get deviceclasses                  # "No resources found" — not an error, proves the API is served
```

`api-resources` confirms all four DRA types are live at `resource.k8s.io/v1`:
`deviceclasses`, `resourceclaims`, `resourceclaimtemplates`, `resourceslices`.

## What's next

Lesson 3 scaffolds the actual project with `kubebuilder init` and
`kubebuilder create api`, once the `Fluke` CRD design
([issue #6](https://github.com/hassanshabbirahmed/fluke-operator/issues/6))
is settled.
