# DRA mechanics in current Kubernetes

Research for issue [#3](https://github.com/hassanshabbirahmed/fluke-operator/issues/3). Unblocks #5 (env setup), #6 (CRD design), #7 (consumer-DRA approach).

**TL;DR**

- **DRA core is GA.** The `resource.k8s.io/v1` API and the `DynamicResourceAllocation` feature gate are **GA in Kubernetes 1.34** and **locked on in 1.35+**. On any cluster >= 1.34 you do **not** need a feature gate or `--runtime-config` for the core object model. Many *sub-features* (admin access, prioritized list, partitionable devices, consumable capacity, device taints, binding conditions, extended resources) are still alpha/beta and each has its own `DRA*` gate.
- **Object model:** `DeviceClass` (admin/driver-defined catalog + CEL selectors) → `ResourceClaim` / `ResourceClaimTemplate` (workload request) → scheduler writes `.status.allocation` → `ResourceSlice` (driver-published inventory per node/pool). A Pod references a claim via `pod.spec.resourceClaims[]` + `container.resources.claims[]`.
- **Driver shape:** a `DaemonSet` running a **kubelet plugin** built with `k8s.io/dynamic-resource-allocation/kubeletplugin`. It registers via the kubelet plugin-registration socket, publishes `ResourceSlice`s with `helper.PublishResources`, implements `NodePrepareResources` / `NodeUnprepareResources` gRPC (called by kubelet), and returns **CDI** device IDs that the runtime uses to inject the device into the container.
- **kind:** use a prebuilt multi-arch `kindest/node:v1.34+` image, plus `containerdConfigPatches` to turn on CDI. Feature gates only needed for alpha/beta extras.
- **`kubernetes-sigs/dra-example-driver` exists**, demonstrates a full mock-GPU driver + Helm chart, and **works natively on Apple Silicon / arm64** (pure Go, mock devices, multi-arch image, kind arm64 node images). The only amd64-only friction is `kind build node-image` (needs Docker, not Podman) — avoidable by using a stock prebuilt node image.

Versions referenced: Kubernetes **1.33–1.37** (1.37 is the current release as of 2026-09; local `kubectl` is v1.36.3). Facts are tagged with the version they apply to.

---

## 1. API versions, GA status, and feature gates

### 1.1 The two eras of DRA

DRA has been reworked twice. Do not trust pre-1.31 material.

| Era | K8s versions | What it was |
|---|---|---|
| "Classic" / control-plane-controller DRA | 1.26–1.31 (alpha only) | `resource.k8s.io/v1alpha2`, allocation driven by a vendor **control-plane controller**. Gated by `DRAControlPlaneController`. **Removed** — that gate existed 1.26→1.31 and is gone. ([feature-gates/DRAControlPlaneController](https://github.com/kubernetes/website/blob/main/content/en/docs/reference/command-line-tools-reference/feature-gates/DRAControlPlaneController.md), [KEP-4381 impl history](https://github.com/kubernetes/enhancements/tree/master/keps/sig-node/4381-dra-structured-parameters)) |
| **"Structured parameters" DRA** (current, KEP-4381) | 1.30 alpha → **1.34 GA** | Scheduler does allocation directly from `ResourceSlice` inventory; no vendor control-plane component. This is what "DRA" means today. |

KEP-4381 implementation history ([source](https://github.com/kubernetes/enhancements/tree/master/keps/sig-node/4381-dra-structured-parameters)):

> - Kubernetes 1.30: Code merged as extension of v1alpha2
> - Kubernetes 1.31: v1alpha3 with new API and several new features
> - Kubernetes 1.32: promotion to beta with v1beta1
> - Kubernetes 1.33: revised v1beta2 API with the same structure as intended for GA
> - Kubernetes 1.34: promotion to GA

### 1.2 `resource.k8s.io` API versions

| API version | Introduced | Status now |
|---|---|---|
| `v1alpha2` | 1.26 | removed |
| `v1alpha3` | 1.31 | served only if `--runtime-config=resource.k8s.io/v1alpha3=true`; carries alpha-only kinds (e.g. `DeviceTaintRule`) |
| `v1beta1` | 1.32 | **storage version**; must stay enabled for encode/decode; served by default until deprecation policy sunsets it |
| `v1beta2` | 1.33 | opt-in via `--runtime-config` for old drivers/workloads |
| **`v1`** | **1.34** | **served by default, GA, this is what you target** |

From KEP-4381 ([source](https://github.com/kubernetes/enhancements/tree/master/keps/sig-node/4381-dra-structured-parameters)):

> The storage version of this API group is `v1beta1`, the version introduced in 1.32. Only the `v1` version is served by default. `v1beta1` must remain supported for encoding/decoding.

From the v1.34 release blog ([kubernetes.io/blog/2025/09/01/kubernetes-v1-34-dra-updates](https://kubernetes.io/blog/2025/09/01/kubernetes-v1-34-dra-updates/)):

> Starting with Kubernetes 1.34, DRA is enabled by default; the DRA features that have reached beta are **also** enabled by default. That's because the default API version for DRA is now the stable `v1` version, and not the earlier versions (eg: `v1beta1` or `v1beta2`) that needed explicit opt in.

The DRA concept doc's `set-up-dra-cluster` task now carries `min-kubernetes-server-version: v1.34` ([source](https://github.com/kubernetes/website/blob/main/content/en/docs/tasks/configure-pod-container/assign-resources/set-up-dra-cluster.md)).

### 1.3 Feature gates (authoritative, from the versioned feature-gate metadata)

Source: `kubernetes/website` `content/en/docs/reference/command-line-tools-reference/feature-gates/*.md`.

| Gate | Components | Alpha | Beta (default on) | GA / locked |
|---|---|---|---|---|
| **`DynamicResourceAllocation`** | apiserver, scheduler, controller-manager, kubelet | 1.30 (was 1.26 for classic) | 1.32 | **stable 1.34** (unlocked), **locked 1.35** |
| `DRAAdminAccess` | apiserver, scheduler | 1.32 | 1.34 | stable 1.36 |
| `DRAPrioritizedList` | apiserver, scheduler | 1.33 | 1.34 | stable 1.36 |
| `DRAResourceClaimDeviceStatus` | apiserver | 1.32 | 1.33 | stable 1.37 |
| `DRADeviceTaints` | apiserver, scheduler, controller-manager | 1.33 | 1.36 | stable/locked 1.37 |
| `DRAPartitionableDevices` | apiserver, scheduler | 1.33 | 1.36 | — |
| `DRADeviceBindingConditions` | apiserver, scheduler | 1.34 | 1.36 | — |
| `DRAConsumableCapacity` | apiserver, scheduler, kubelet | 1.34 | 1.36 | — |
| `DRAExtendedResource` | apiserver, scheduler | 1.34 | 1.36 | stable 1.37 |
| `DRASchedulerFilterTimeout` | scheduler | 1.34 (beta-default per KEP) | — | — |
| `DRANodeAllocatableResources` | apiserver, scheduler, kubelet | 1.36 | — | — |
| `DRAWorkloadResourceClaims` (PodGroup claims) | apiserver, scheduler, controller-manager, kubelet | 1.36 | 1.37 | — |
| `DRADeviceCompatibilityGroups` | apiserver, scheduler | 1.37 | — | — |
| `DRAOptionalNodeOperations` | apiserver, scheduler, kubelet | 1.37 | — | — |

Which component needs `DynamicResourceAllocation` (KEP-4381 PRR):

> - Feature gate name: DynamicResourceAllocation
>   - Components depending on the feature gate: kube-apiserver, kubelet, kube-scheduler, kube-controller-manager

**For fluke-operator on a >= 1.34 cluster: the core gate is on by default and you touch nothing.** You only add `featureGates:` / `--runtime-config` entries when you deliberately exercise an alpha/beta extra (e.g. `DRAAdminAccess`, `DRADeviceBindingConditions`).

---

## 2. Object model

All four kinds live in `resource.k8s.io/v1` ([dra-api.md](https://github.com/kubernetes/website/blob/main/content/en/docs/concepts/resource-management/dynamic-resource-allocation/dra-api.md)).

### 2.1 `DeviceClass` — the catalog (cluster-scoped)

Created by the driver vendor and/or cluster admin. Defines a category of devices and the CEL selectors / config that apply to any claim referencing it.

```yaml
apiVersion: resource.k8s.io/v1
kind: DeviceClass
metadata:
  name: fluke.example.com          # e.g. one class per fluke device kind
spec:
  selectors:
  - cel:
      expression: 'device.driver == "fluke.example.com"'
  # optional: config:  (opaque vendor config passed to the driver)
```

`kubectl get deviceclasses` returning `No resources found` (rather than `error: the server doesn't have a resource type`) is the canonical "DRA is enabled" check ([set-up-dra-cluster.md](https://github.com/kubernetes/website/blob/main/content/en/docs/tasks/configure-pod-container/assign-resources/set-up-dra-cluster.md)).

### 2.2 `ResourceClaim` — an explicit request (namespaced)

A request for one or more devices. Create it directly when **multiple Pods share the same device** or the claim must outlive a Pod.

```yaml
apiVersion: resource.k8s.io/v1
kind: ResourceClaim
metadata:
  name: single-gpu
spec:
  devices:
    requests:
    - name: gpu
      exactly:
        deviceClassName: example-device-class
        allocationMode: All          # or ExactCount + count: N
        selectors:
        - cel:
            expression: |-
              device.attributes["driver.example.com"].type == "gpu" &&
              device.capacity["driver.example.com"].memory == quantity("64Gi")
```

Request shapes: `exactly:` (one alternative) or `firstAvailable:` (a prioritized list of subrequests, gate `DRAPrioritizedList`). Constraints (e.g. `matchAttribute`, `distinctAttribute`) restrict combinations across a multi-device request.

### 2.3 `ResourceClaimTemplate` — per-Pod claims (namespaced)

Use when **each Pod wants its own separate device**. The `resourceclaim-controller` in kube-controller-manager generates one `ResourceClaim` per Pod, owned by (and GC'd with) that Pod.

```yaml
apiVersion: resource.k8s.io/v1
kind: ResourceClaimTemplate
metadata:
  name: gpu-template
spec:
  spec:                              # note: nested spec
    devices:
      requests:
      - name: gpu
        exactly:
          deviceClassName: example-device-class
```

### 2.4 `ResourceSlice` — driver-published inventory (cluster-scoped, node-owned)

The DRA driver publishes one or more `ResourceSlice`s per node describing the devices it manages, their `attributes` (typed: string/int/bool/version) and `capacity` (quantities). Owned by the `Node` object, so they vanish when the node goes away. Grouped into **pools** (`spec.pool.name` / `generation` / `resourceSliceCount`); a pool can span slices.

```yaml
apiVersion: resource.k8s.io/v1
kind: ResourceSlice
spec:
  nodeName: worker-1
  driver: fluke.example.com
  pool: { name: worker-1, generation: 1, resourceSliceCount: 1 }
  devices:
  - name: gpu-0
    attributes:
      type:   { string: gpu }
      uuid:   { string: "gpu-18db0e85-..." }
    capacity:
      memory: { value: 64Gi }
```

Scheduler allocation strategy ([how-dra-works.md](https://github.com/kubernetes/website/blob/main/content/en/docs/concepts/resource-management/dynamic-resource-allocation/how-dra-works.md)):

> The scheduler uses a first-fit strategy and evaluates pools and ResourceSlices in lexicographical order by their names. Drivers can prioritize specific slices or pools by naming them appropriately.

### 2.5 Allocation results

After the scheduler picks devices it writes them onto the claim's `.status`:

```yaml
status:
  allocation:
    devices:
      results:
      - request: gpu
        driver: fluke.example.com
        pool: worker-1
        device: gpu-0
        # + consumedCapacity / shareID (DRAConsumableCapacity)
        # + skipNodeOperations   (DRAOptionalNodeOperations)
    nodeSelector: {...}              # node(s) that can reach the device
  reservedFor:
  - { resource: pods, name: my-pod, uid: ... }   # max 256 entries
```

`reservedFor` caps sharing at **256 Pods** per claim (the `DRAWorkloadResourceClaims` / PodGroup feature raises this by reserving for a PodGroup instead).

### 2.6 How a Pod references a claim

Two-level reference ([allocate-devices-dra.md](https://github.com/kubernetes/website/blob/main/content/en/docs/tasks/configure-pod-container/assign-resources/allocate-devices-dra.md)):

```yaml
apiVersion: v1
kind: Pod
spec:
  resourceClaims:                       # pod-level: name the claim(s)
  - name: gpu
    resourceClaimTemplateName: gpu-template   # OR resourceClaimName: single-gpu
  containers:
  - name: app
    image: ubuntu:24.04
    resources:
      claims:
      - name: gpu                        # container opts in to the pod-level claim
        # optional: request: gpu         # only a specific sub-request
```

A container only sees the device (and its [device metadata](https://github.com/kubernetes/website/blob/main/content/en/docs/concepts/resource-management/dynamic-resource-allocation/dra-features.md)) if it lists the claim in `resources.claims`. Pods with `spec.nodeName` preset bypass the scheduler and will get stuck unless the claim is already allocated + reserved — prefer a `nodeSelector` to pin a Pod.

---

## 3. How a DRA driver is structured

Reference implementation: [`kubernetes-sigs/dra-example-driver`](https://github.com/kubernetes-sigs/dra-example-driver) (mock-GPU driver + Helm chart, `k8s.io/dynamic-resource-allocation v0.37.0`). The in-tree e2e test driver is `kubernetes/kubernetes/test/e2e/dra/test-driver`.

### 3.1 Deployment shape

- A **`DaemonSet`** (`*-kubeletplugin`) on every node that has the hardware. In the example driver it runs `dra-example-kubeletplugin` and hostPath-mounts:
  - `/var/lib/kubelet/plugins_registry` (registration socket dir)
  - `/var/lib/kubelet/plugins` (plugin data / gRPC socket dir)
  - `/var/run/cdi` (where the driver writes CDI spec files)
- Optionally a control-plane **`Deployment`** (`*-controller`) for cluster-scoped bookkeeping (e.g. binding conditions) and a **webhook** for validating opaque config. Structured-parameters DRA does **not** need a vendor scheduler or allocation controller.
- RBAC: get/list/watch `resourceclaims`, `resourceslices`, `deviceclasses`; update `resourceclaims/status`; create/update/delete `resourceslices`.

### 3.2 The `k8s.io/dynamic-resource-allocation` helper module

`import "k8s.io/dynamic-resource-allocation/kubeletplugin"` — see [pkg.go.dev](https://pkg.go.dev/k8s.io/dynamic-resource-allocation/kubeletplugin). You implement a small interface; the `Helper` does registration, socket management, gRPC serving, versioning, and ResourceSlice reconciliation.

```go
helper, err := kubeletplugin.Start(ctx, driver,          // driver implements DRAPlugin
    kubeletplugin.KubeClient(coreclient),
    kubeletplugin.NodeName(nodeName),
    kubeletplugin.DriverName("fluke.example.com"),
    kubeletplugin.RegistrarDirectoryPath(kubeletplugin.KubeletRegistryDir), // /var/lib/kubelet/plugins_registry
    kubeletplugin.PluginDataDirectoryPath(pluginPath),                      // /var/lib/kubelet/plugins/<driver>
    kubeletplugin.RollingUpdate(podUID),
)
// publish inventory (non-blocking; reconciled toward the API server)
err = helper.PublishResources(ctx, state.driverResources)  // resourceslice.DriverResources
```

The `DRAPlugin` interface you implement:

| Method | Called when | Returns |
|---|---|---|
| `PrepareResourceClaims(ctx, []*ResourceClaim)` | kubelet is about to start a Pod that uses your claims | `map[claimUID]PrepareResult{ Devices: []kubeletplugin.Device{ ...CDIDeviceIDs }, Err }` |
| `UnprepareResourceClaims(ctx, []NamespacedObject)` | Pod terminated | `map[claimUID]error`; **must be idempotent** (kubelet may retry; claim may already be deleted, hence UID/namespace/name only) |
| `HandleError(ctx, err)` | background publish errors | log; `errors.Is(err, kubeletplugin.ErrRecoverable)` distinguishes fatal |
| `WatchHealthStatus(ctx)` | optional device health stream | channel of `DeviceHealthReport`, or `ErrHealthNotSupported` |

Helper-managed sockets: registration socket `<driver>-reg.sock` in `plugins_registry/`; the DRA gRPC socket in `plugins/<driver>/`. Rolling updates get alternate socket names (`RollingUpdate` option) to fit the `AF_UNIX` 108-char path limit.

### 3.3 kubelet ⇄ plugin gRPC API

Two services are served to the local kubelet over the plugin's UNIX socket ([pkg.go.dev/kubeletplugin](https://pkg.go.dev/k8s.io/dynamic-resource-allocation/kubeletplugin)):

1. **Plugin registration** (`pluginregistration.v1`) — kubelet watches `plugins_registry/`, dials the `-reg.sock`, calls `GetInfo` (type `DRAPlugin`, name, endpoint) then `NotifyRegistrationStatus`.
2. **DRA node service** — `NodePrepareResources` / `NodeUnprepareResources`. The helper serves **both `v1beta1` and `v1`** of this service (options `NodeV1beta1()` / `NodeV1()`) so a newer kubelet or an older one both work. Per KEP-4381 version-skew strategy: "A DRA driver has to implement all gRPC interfaces that might be used by older releases of kubelet" — the helper handles this for you.

`kubelet` flow on Pod admission: read `ResourceClaim.status.allocation` → for each driver, call `NodePrepareResources` → driver returns CDI device IDs → kubelet passes them to the CRI runtime → runtime applies the CDI spec. On Pod deletion, `NodeUnprepareResources`. With `DRAOptionalNodeOperations` (1.37 alpha) a driver can set `spec.skipNodeOperations` on its slices so kubelet skips these calls for control-plane-only devices.

### 3.4 CDI (Container Device Interface)

The driver does **not** mutate the container spec directly. During `PrepareResourceClaims` it:

1. Builds a CDI spec (JSON/YAML) describing the device edits — env vars, device nodes, mounts, hooks — using `tags.cncf.io/container-device-interface/pkg/cdi` (`cdiapi.Cache`, `WriteSpec`).
2. Writes it under `/var/run/cdi/` (e.g. `k8s.fluke.example.com-<class>.json`), with a qualified device name `fluke.example.com/<class>=<claimUID>-<device>`.
3. Returns those **CDI device IDs** in the `PrepareResult`.

kubelet forwards the IDs via CRI; **containerd/CRI-O must have CDI enabled** to honour them. For containerd that is `enable_cdi = true` under `[plugins."io.containerd.grpc.v1.cri"]`. Per KEP-4381, "At the time of Kubernetes 1.31, most container runtimes support this." Optionally the driver also exposes device metadata JSON at `/var/run/kubernetes.io/dra-device-attributes/...` via CDI bind-mounts (KEP-5304, 1.36 alpha, driver-side only, no gate).

---

## 4. `kind` cluster config for DRA

### 4.1 Minimum viable (core DRA only, K8s >= 1.34)

Core DRA needs only: (a) a node image >= 1.34, (b) CDI enabled in containerd. No feature gates, no `runtimeConfig`.

```yaml
# kind-dra.yaml
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
containerdConfigPatches:
  # https://tags.cncf.io/container-device-interface#containerd-configuration
  - |-
    [plugins."io.containerd.grpc.v1.cri"]
      enable_cdi = true
nodes:
  - role: control-plane
  - role: worker            # driver DaemonSet runs here
```

```bash
# get the exact <tag>@sha256:<digest> for your kind version from its release notes
kind create cluster --config kind-dra.yaml \
  --image kindest/node:v1.36.1@sha256:<digest-from-kind-release-notes>
kubectl get deviceclasses      # -> "No resources found"  => DRA is live
```

- Prebuilt `kindest/node` images are **multi-arch (amd64 + arm64)** — kind v0.32.0's release notes list digests for `v1.34.8`, `v1.35.5`, `v1.36.1` (and `v1.33.12`); older kind releases list older patch versions. Always copy the `tag@sha256:` pair from the release notes of the kind version you have. ([kind v0.32.0 release](https://github.com/kubernetes-sigs/kind/releases/tag/v0.32.0))
- On older kind node images that predate CDI-enabled containerd you would need `kind build node-image` — **not needed for 1.34+ stock images**.
- CDI directory default is `/etc/cdi` and `/var/run/cdi`; the example driver writes to `/var/run/cdi`.

### 4.2 With alpha/beta extras (mirrors `dra-example-driver`)

The example driver's `demo/scripts/kind-cluster-config.yaml` ([source](https://github.com/kubernetes-sigs/dra-example-driver/blob/main/demo/scripts/kind-cluster-config.yaml)) — it builds a node image from `KIND_K8S_TAG=v1.37.0`:

```yaml
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
featureGates:
  DynamicResourceAllocation: true          # redundant on >=1.34, harmless
  DRAAdminAccess: true
  DRAWorkloadResourceClaims: true
  GenericWorkload: true
  DRAExtendedResource: true
  DRADeviceBindingConditions: true
  DRANodeAllocatableResources: true
  DRAConsumableCapacity: true
runtimeConfig:
  resource.k8s.io/v1beta1: "true"
  scheduling.k8s.io/v1beta1: "true"
containerdConfigPatches:
  - |-
    [plugins."io.containerd.grpc.v1.cri"]
      enable_cdi = true
nodes:
  - role: control-plane
    kubeadmConfigPatches:
      - |
        kind: ClusterConfiguration
        scheduler:
          extraArgs: { v: "1" }
        controllerManager:
          extraArgs: { v: "1" }
  - role: worker
```

Notes:
- `kind`'s `featureGates:` map propagates the gate to **all** components (apiserver, scheduler, controller-manager, kubelet) — which is exactly DRA's requirement.
- `kind`'s `runtimeConfig:` map becomes `kube-apiserver --runtime-config=...`. Only needed for `v1beta1`/`v1beta2`/`v1alpha3` (old drivers, or alpha kinds like `DeviceTaintRule`). The DRA **`v1`** API needs no entry.
- `kubeadmConfigPatches` per-component `extraArgs` is the way to pass anything gate-map can't express.

### 4.3 Colima specifics (Apple Silicon)

- `colima start --cpu 4 --memory 8 --disk 60` (DRA + operator + registry need headroom). kind works on Colima's Docker or containerd runtime.
- Use `docker` as the container tool. `kind build node-image` requires **Docker, not Podman** (per the example driver's `build-kind-image.sh`) — another reason to prefer a stock prebuilt node image.
- No emulation needed for the arm64 path (see §5).

---

## 5. `dra-example-driver` and the Apple Silicon / arm64 verdict

**Yes, it exists**: <https://github.com/kubernetes-sigs/dra-example-driver>. It is the SIG-maintained "fork this to start your driver" repo.

**What it demonstrates:**
- A complete DRA driver for **mock GPU devices** (no real hardware) wrapped in a **Helm chart**: kubeletplugin `DaemonSet`, optional controller `Deployment`, optional validating webhook.
- `ResourceSlice` publishing (8 fake GPUs/node by default), CEL selectors, `ResourceClaim` vs `ResourceClaimTemplate`, shared claims across Pods/containers, opaque `GpuConfig` vendor config, admin access, prioritized alternatives, partitionable devices, consumable capacity, device taints/tolerations, binding conditions, extended-resource requests.
- Full local loop: `./demo/build-driver.sh` → `./demo/clusters/kind/create-cluster.sh` → `helm upgrade -i ... deployments/helm/dra-example-driver` → `kubectl apply -f demo/examples/<name>/`.
- CDI injection surfaces as env vars inside the workload container (e.g. `GPU_DEVICE_0=...`, `DRA_RESOURCE_DRIVER_NAME=...`).

**Apple Silicon / arm64 verdict: works natively.**

| Concern | Verdict |
|---|---|
| DRA itself on arm64 | Architecture-independent. It's apiserver/scheduler/kubelet logic + CRD-like built-in types. `kindest/node` images are multi-arch amd64+arm64. |
| The example driver binary | Pure Go, mock devices, no vendor/CUDA/hardware libs. `go.mod` targets `k8s.io/* v0.37.0`. Builds `linux/arm64` natively on Apple Silicon. |
| The example driver image | Published multi-arch: `linux/amd64,linux/arm64,linux/ppc64le`. README: *"On Apple Silicon, single-arch `linux/arm64` builds work natively; building `linux/amd64` uses emulation the same way."* |
| CDI on arm64 | containerd/CRI-O CDI support is arch-independent; enabled via the config patch. |
| `kind build node-image` | **amd64-friction point**: requires Docker (not Podman) and builds K8s from source. **Avoid it** — use a stock `kindest/node:v1.34+@sha256` image, which already has CDI-capable containerd. |
| Colima + amd64 emulation flakiness | Only relevant if you pull amd64 images. Stay on arm64 images and it's a non-issue. If you ever must run an amd64 image: `docker run --privileged --rm tonistiigi/binfmt --install all`. |

Bottom line for fluke-operator: build and run everything as `linux/arm64`, use a pinned prebuilt `kindest/node` image, never invoke `kind build node-image`.

---

## 6. What this means for fluke-operator

Project: a Kubebuilder operator + a minimal DRA driver `fluke.example.com`, local env Colima + kind on Apple Silicon, `kubectl` v1.36.3.

**Environment (feeds #5):**
- kind cluster from a pinned multi-arch `kindest/node:v1.34+` (or `v1.36.x` to match local kubectl) image + the `enable_cdi = true` containerd patch. No feature gates for core DRA.
- Colima `--cpu 4 --memory 8 --disk 60`, Docker runtime, everything `linux/arm64`.
- Add `featureGates:` entries only when a milestone explicitly needs an alpha/beta extra (likely none for a v1 MVP).
- Target the `resource.k8s.io/v1` API exclusively. Pin driver deps to `k8s.io/dynamic-resource-allocation` and `k8s.io/api` matching the cluster minor (v0.34+).

**Driver design (feeds #7):**
- Structure = `DaemonSet` running a kubelet plugin built on `k8s.io/dynamic-resource-allocation/kubeletplugin`. Implement `PrepareResourceClaims` / `UnprepareResourceClaims` (idempotent) + `PublishResources`. No vendor scheduler/controller needed.
- Consumer side: workloads reference devices via `pod.spec.resourceClaims[]` + `container.resources.claims[]`, backed by a `ResourceClaimTemplate` (per-Pod) or shared `ResourceClaim`. The operator likely owns a `DeviceClass` named for `fluke.example.com` and generates `ResourceClaimTemplate`s.
- Device injection is via **CDI**: write a spec to `/var/run/cdi/`, return CDI IDs. Even a "device" that's just an env var / mount goes through CDI. Fork `dra-example-driver`'s `cdi.go` / `state.go` as the starting point.
- Mock-device approach (like the example driver) is the right call for a teaching project on a Mac — no hardware, fully arm64-native.

**CRD design (feeds #6):**
- The DRA object model (`DeviceClass`, `ResourceClaim`, `ResourceClaimTemplate`, `ResourceSlice`) is **built-in**, not CRDs you define. Your operator's CRD is a *higher-level* abstraction (e.g. a `FlukeDevicePool` or `FlukeClaim`) that the operator reconciles **into** DRA objects — mirror how `ResourceClaimTemplate` nests a claim `spec`, and how allocation results land on `.status`.
- `ResourceSlice` attributes are strongly typed (string/int/bool/version) and capacities are `resource.Quantity`; design any CRD fields that map to them the same way.
- `reservedFor` 256-Pod cap and "no scheduler preemption for DRA" are constraints to document for users.

---

## Sources

Primary:
- Kubernetes docs — Dynamic Resource Allocation: [_index](https://github.com/kubernetes/website/blob/main/content/en/docs/concepts/resource-management/dynamic-resource-allocation/_index.md), [dra-api](https://github.com/kubernetes/website/blob/main/content/en/docs/concepts/resource-management/dynamic-resource-allocation/dra-api.md), [how-dra-works](https://github.com/kubernetes/website/blob/main/content/en/docs/concepts/resource-management/dynamic-resource-allocation/how-dra-works.md), [dra-features](https://github.com/kubernetes/website/blob/main/content/en/docs/concepts/resource-management/dynamic-resource-allocation/dra-features.md)
- Kubernetes docs — tasks: [set-up-dra-cluster](https://github.com/kubernetes/website/blob/main/content/en/docs/tasks/configure-pod-container/assign-resources/set-up-dra-cluster.md), [allocate-devices-dra](https://github.com/kubernetes/website/blob/main/content/en/docs/tasks/configure-pod-container/assign-resources/allocate-devices-dra.md)
- Feature-gate metadata: `kubernetes/website` `content/en/docs/reference/command-line-tools-reference/feature-gates/DRA*.md` and `DynamicResourceAllocation.md`
- [KEP-4381 — DRA with Structured Parameters](https://github.com/kubernetes/enhancements/tree/master/keps/sig-node/4381-dra-structured-parameters)
- [Kubernetes v1.34: DRA has graduated to GA](https://kubernetes.io/blog/2025/09/01/kubernetes-v1-34-dra-updates/)
- [`k8s.io/dynamic-resource-allocation/kubeletplugin` godoc](https://pkg.go.dev/k8s.io/dynamic-resource-allocation/kubeletplugin)
- [`kubernetes-sigs/dra-example-driver`](https://github.com/kubernetes-sigs/dra-example-driver) — README, `demo/scripts/kind-cluster-config.yaml`, `demo/scripts/common.sh`, `cmd/dra-example-kubeletplugin/{driver,cdi,state}.go`, `go.mod`, Helm `kubeletplugin.yaml`
- [kind — Quick Start](https://kind.sigs.k8s.io/docs/user/quick-start/), [kind v0.32.0 release](https://github.com/kubernetes-sigs/kind/releases/tag/v0.32.0)
- [CNCF TAG CDI — containerd configuration](https://tags.cncf.io/container-device-interface#containerd-configuration)

_Compiled 2026-09-04. Current Kubernetes release line: 1.37; local kubectl v1.36.3._
