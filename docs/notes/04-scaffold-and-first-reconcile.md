# Lesson 3 — scaffold fluke-operator and get a trivial reconcile running

Resolves [Lesson 3 — scaffold fluke-operator and get a trivial reconcile running](https://github.com/hassanshabbirahmed/fluke-operator/issues/9).

## The two commands that built this

```bash
kubebuilder init --domain example.com --repo github.com/hassanshabbirahmed/fluke-operator --project-name fluke-operator
kubebuilder create api --group fluke --version v1alpha1 --kind Fluke --resource --controller
```

`init` scaffolds a whole Go module — `go.mod`, `cmd/main.go` (the manager's
entrypoint), a `Makefile` with the `make manifests`/`make install`/`make run`
targets used below, RBAC/CRD kustomize trees under `config/`, and CI
workflows. `create api` adds the actual `Fluke` type
(`api/v1alpha1/fluke_types.go`) and its controller
(`internal/controller/fluke_controller.go`) — the two files this lesson
actually edited; everything else is infrastructure kubebuilder generates once
and you rarely touch by hand.

## From design to types

[`docs/notes/03-fluke-design.md`](./03-fluke-design.md) became
`api/v1alpha1/fluke_types.go` almost verbatim: `Replicas`, `FlukesPerReplica`,
`Image` on the spec; `Phase`, `ReadyReplicas`, `AllocatedFlukes`, `Conditions`
on the status. Two additions beyond the design doc, both routine Kubebuilder
practice:

- **Validation markers** (`+kubebuilder:validation:Minimum=1`,
  `MinLength=1`, `+kubebuilder:default=1`) — turned into real OpenAPI schema
  on the CRD by `make manifests`, so the API server itself rejects a `Fluke`
  with no `image` rather than the controller finding out later.
- **`+kubebuilder:printcolumn` markers** — what makes `kubectl get fluke`
  show `PHASE`/`REPLICAS`/`READY`/`FLUKES`/`AGE` columns instead of just
  `NAME`/`AGE`.

`make manifests generate` reads these markers and regenerates
`config/crd/bases/fluke.example.com_flukes.yaml` (the actual CRD to apply)
and `api/v1alpha1/zz_generated.deepcopy.go` (the `DeepCopy`/`DeepCopyObject`
methods every Kubernetes type needs — controller-runtime writes this file
for you; never hand-edit it).

## The trivial reconcile

`internal/controller/fluke_controller.go`'s `Reconcile` does exactly three
things, on purpose nothing more:

1. `r.Get` the `Fluke`; if it's gone, return (nothing to do).
2. `r.Get` the Deployment of the same name; if missing, build one from the
   spec, set the Fluke as its owner (`ctrl.SetControllerReference`), and
   `r.Create` it.
3. If the Deployment already exists, log and stop — **no drift correction
   yet**. Lesson 4 is precisely "what happens in that branch."

`SetupWithManager` also gained `.Owns(&appsv1.Deployment{})` alongside
`.For(&flukev1alpha1.Fluke{})` — this makes the controller re-run `Reconcile`
whenever the *owned Deployment* changes, not just the `Fluke`. It's inert
right now (step 3 above just logs), but it's the hook Lesson 4's
self-healing hangs off, so it's wired up once rather than revisited.

## What running it actually showed

```bash
make install   # applies the CRD to kind-fluke
make run       # runs the manager out-of-cluster, against the current kubeconfig context
```

Applying [`config/samples/fluke_v1alpha1_fluke.yaml`](../../config/samples/fluke_v1alpha1_fluke.yaml)
(`replicas: 2, flukesPerReplica: 1, image: busybox`) produced a real
Deployment `demo` with 2 replicas and the busybox image, `ownerReferences`
pointing back at the `Fluke`. The manager's log showed **several reconciles
firing for one `kubectl apply`** — one for the `Fluke` create, more as the
Deployment's own status updated while pods came up — each one independently
concluding "Deployment already exists, nothing to do." That's Lesson 1's
level-triggered/idempotent idea, observed for real rather than just read
about.

`kubectl delete fluke demo` then cascade-deleted the Deployment (and its
pods) with **zero controller code involved** — that's Kubernetes' built-in
garbage collector acting on the owner reference, not anything `Reconcile`
does. This is exactly why [the design doc](./03-fluke-design.md) scopes the
Lesson 6 finalizer narrowly: owner-reference GC already handles the
Deployment; the finalizer only earns its place once there's device state
*outside* Kubernetes' own object graph to release first.

## Verification

- `go build ./...`, `go vet ./...` — clean
- `make test` — envtest suite passes (a real, ephemeral API server, not a
  fake client), 71.4% coverage on `internal/controller`
- Manual run against `kind-fluke`, as described above

The generated `internal/controller/fluke_controller_test.go` needed one
fix: its `BeforeEach` created a bare `Fluke{}` with no spec, which the new
`MinLength=1` validation on `image` now rejects. Gave it a minimal valid
spec (`replicas: 1, flukesPerReplica: 1, image: busybox`) instead.

## What's next

Lesson 4 — real reconcile: correcting the Deployment when it drifts from
`spec` (image changed, replica count changed, or someone deletes it by
hand), which is what actually exercises the `.Owns()` watch wired up here.
