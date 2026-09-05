# Fluke v1alpha1 design

Resolves [Design the Fluke custom resource and its reconcile behaviour](https://github.com/hassanshabbirahmed/fluke-operator/issues/6).

Validated interactively via a [click-through prototype](https://github.com/hassanshabbirahmed/fluke-operator/tree/prototype/fluke-reconcile/docs/prototypes/PROTOTYPE-fluke-reconcile.html)
(kept on the throwaway branch `prototype/fluke-reconcile`, not on `main`) —
five scenarios exercised (happy path, self-healing after drift, scaling
mid-flight, graceful deletion, device shortage); no changes requested.

See `CONTEXT.md` for the `Fluke` / `fluke device` vocabulary this introduces.

## Spec

```go
type FlukeSpec struct {
	Replicas         int32  `json:"replicas"`
	FlukesPerReplica int32  `json:"flukesPerReplica"`
	Image            string `json:"image"`
}
```

Kept intentionally small — this is Phase 1's teaching vehicle, not a real
workload API. `Replicas * FlukesPerReplica` is the total fluke devices a
`Fluke` wants.

## Status

```go
type FlukePhase string

const (
	FlukePending     FlukePhase = "Pending"
	FlukeProgressing FlukePhase = "Progressing"
	FlukeReady       FlukePhase = "Ready"
	FlukeDegraded    FlukePhase = "Degraded"
)

type FlukeStatus struct {
	Phase           FlukePhase         `json:"phase,omitempty"`
	ReadyReplicas   int32              `json:"readyReplicas"`
	AllocatedFlukes int32              `json:"allocatedFlukes"`
	Conditions      []metav1.Condition `json:"conditions,omitempty"`
}
```

`Phase` is derived, never set directly by a user — it's the reconcile loop's
summary of `ReadyReplicas` vs `Spec.Replicas` and `AllocatedFlukes` vs desired.

## What the controller owns

One `Deployment`, named after the `Fluke`, set as its owned object via an
owner reference (cascading delete, and the trigger for self-healing when
something else touches it). Later — once Phase 3 (DRA) lands — its pod
template will also carry a `ResourceClaimTemplate` reference sized for
`FlukesPerReplica` devices per pod.

**Prototype simplification, noted so it isn't mistaken for the real
mechanism:** the prototype tracks one aggregate `allocatedDevices` counter
against a fixed pool. The real allocation ([issue #3](https://github.com/hassanshabbirahmed/fluke-operator/issues/3))
is per-pod: one `ResourceClaim` generated per pod from a shared
`ResourceClaimTemplate`, each claim independently satisfied against the
cluster's `ResourceSlice`s. `AllocatedFlukes` in the real controller is
computed by summing bound claims across the owned pods, not read off a
single counter. This distinction is exactly what Lesson 11 (building the
fluke DRA driver) has to get right.

## Reconcile order, and why

1. **Not found** → nothing to do, return.
2. **Being deleted** (`deletionTimestamp` set):
   - Finalizer present → release allocated fluke devices, let the owner
     reference cascade-delete the Deployment, remove the finalizer.
   - Finalizer absent → the API server removes the object; nothing to do.
3. **Finalizer absent** (and not being deleted) → add `fluke.example.com/cleanup`.
   This has to happen *before* anything is created, or a delete racing the
   first reconcile could leak the Deployment and its devices with nothing
   left to clean them up.
4. **Ensure the Deployment** matches `spec` (create if missing, correct if
   drifted) — self-healing lives here.
5. **Ensure fluke devices** are allocated toward `Replicas * FlukesPerReplica`.
6. **Compute and write status** from what steps 4–5 actually observed:
   `Progressing` while pods aren't ready, `Degraded` if devices are short
   even once pods are ready, `Ready` once both hold.

The whole loop is **level-triggered and idempotent** — every step re-derives
its decision from current state rather than remembering what it did last
time, so running it any number of times with no state change is a no-op.
This is why `Reconcile` never needs to know *why* it was invoked, only what
*is* true right now.

## The finalizer's actual job here

Not a generic "always add a finalizer" — it exists specifically because
fluke devices are external state (allocated in `resource.k8s.io`, outside
this object) that owner-reference cascade-delete doesn't reach on its own.
The finalizer is what makes "delete the `Fluke`" mean "release its devices
first," rather than leaving them dangling.

## Example lifecycle (from the prototype's happy path)

```yaml
# kubectl apply
apiVersion: fluke.example.com/v1alpha1
kind: Fluke
metadata:
  name: demo
spec:
  replicas: 2
  flukesPerReplica: 1
  image: busybox
```

```yaml
# after two reconciles and the kubelet reporting both pods ready
apiVersion: fluke.example.com/v1alpha1
kind: Fluke
metadata:
  name: demo
  finalizers: ["fluke.example.com/cleanup"]
spec:
  replicas: 2
  flukesPerReplica: 1
  image: busybox
status:
  phase: Ready
  readyReplicas: 2
  allocatedFlukes: 2
  conditions:
    - type: Available
      status: "True"
      reason: AllReplicasReady
      message: all replicas ready with devices allocated
```

## What's next

[Decide the consumer-side DRA approach](https://github.com/hassanshabbirahmed/fluke-operator/issues/7)
is now unblocked. Lesson 3 — scaffolding this design for real with
`kubebuilder create api` and getting a trivial reconcile running against
`kind-fluke` — graduates from fog into its own ticket now that the shape is
settled.
