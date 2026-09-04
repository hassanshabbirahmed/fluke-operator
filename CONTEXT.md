# fluke-operator

A teaching Kubernetes operator (Go, Kubebuilder) that manages a `Fluke` custom
resource and includes a minimal Dynamic Resource Allocation (DRA) driver.
Built incrementally as a series of lessons — see the map at
[issue #1](https://github.com/hassanshabbirahmed/fluke-operator/issues/1).

## Language

**DeviceClass**:
A cluster-scoped catalog entry describing one category of allocatable device (e.g. "a fluke device"), including the CEL selectors used to match specific devices to it. Defined once per device category, referenced by many claims.
_Avoid_: device type, device kind

**ResourceClaim**:
A namespaced request for one or more devices matching a `DeviceClass`, created either directly or generated from a `ResourceClaimTemplate`. Its `.status.allocation` is where the scheduler records which actual device(s) were assigned.
_Avoid_: claim, device request

**ResourceClaimTemplate**:
A namespaced stencil for `ResourceClaim`s, referenced from a Pod's `spec.resourceClaims`. Kubernetes generates one `ResourceClaim` per Pod from it, and garbage-collects that claim when the Pod is deleted.
_Avoid_: claim template

**ResourceSlice**:
A Node-owned, driver-published inventory of the actual devices available on that node (their attributes and capacity), grouped into pools. This is what a DRA driver publishes and what allocation matches `ResourceClaim`s against.
_Avoid_: device slice, device inventory
