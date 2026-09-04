# Shipping Kubebuilder operator CRDs and conversion webhooks through a Helm chart

Research notes on CRD and webhook **lifecycle** when a Kubebuilder-built operator is
distributed as a Helm chart. Every claim is sourced inline. Primary sources only
(official docs, specs, source, and the real charts named in the brief).

Scope note: this is background research for a teaching project (a `Fluke` CRD in group
`fluke.example.com`, later doing a real `v1alpha1` -> `v1alpha2` conversion webhook
lesson). No "recommended approach" section here by design.

Verified against: Helm docs at v4.2.4, Kubebuilder book (master), cert-manager v1.21.x
docs, Kubernetes docs (current), prometheus-community charts `main` branch. Date of
research: 2026-09-04.

---

## 1. The `crds/` directory vs templated CRDs trade-off

### What the `crds/` directory is

Helm 3 introduced a dedicated top-level `crds/` directory in a chart. "These CRDs are
not templated, but will be installed by default when running a `helm install` for the
chart. If you wish to skip the CRD installation step, you can pass the `--skip-crds`
flag."
(<https://helm.sh/docs/chart_best_practices/custom_resource_definitions/>)

Install ordering: Helm "will upload the CRDs, pause until the CRDs are made available by
the API server, and then start the template engine" — i.e. CRDs are applied *before*
any template in `templates/` is rendered.
(<https://helm.sh/docs/topics/charts/> — "Custom Resource Definitions (CRDs)" section)

### The documented limitations of `crds/`

From <https://helm.sh/docs/topics/charts/> ("Limitations on CRDs"):

- "CRDs are never reinstalled. If Helm determines that the CRDs in the `crds/`
  directory are already present (regardless of version), Helm will not attempt to
  install or upgrade."
- "CRDs are never installed on upgrade or rollback. Helm will only create CRDs on
  installation operations."
- "CRDs are never deleted. Deleting a CRD automatically deletes all of the CRD's
  contents across all namespaces in the cluster. Consequently, Helm will not delete
  CRDs."

And: "CRD files *cannot be templated*. They must be plain YAML documents."
(<https://helm.sh/docs/topics/charts/>)

Also not supported for `crds/`:

- No Helm hooks / hook annotations on `crds/` content (they are applied out-of-band,
  before rendering).
- "The `--dry-run` flag of `helm install` and `helm upgrade` is not currently supported
  for CRDs."
  (<https://helm.sh/docs/chart_best_practices/custom_resource_definitions/>)

### Why Helm refuses to upgrade/delete `crds/`

This was a deliberate design decision, documented in HIP-0011 ("CRD Handling in Helm 3",
<https://helm.sh/community/hips/hip-0011/>) and echoed in the best-practices page:
"There is no support at this time for upgrading or deleting CRDs using Helm. This was an
explicit decision after much community discussion due to the danger for unintentional
data loss."
(<https://helm.sh/docs/chart_best_practices/custom_resource_definitions/>)

HIP-0011's reasoning:

- "CRDs (being a globally shared resource) are fragile. Once a CRD is installed, we
  typically have to assume (all other things being equal) that it is shared across
  namespaces and groups of users."
- "CRDs are cluster-wide. A user may have no idea that by updating a Helm chart, it
  breaks other parts of the cluster that the user does not even have access to."
- On deletion: deleting a CRD cascade-deletes every custom resource of that kind in
  every namespace, and "there is no way to determine when it is safe to remove a CRD
  short of determining that the cluster itself is being destroyed."
- Design principle ("users first"): "We can assume that the *person using the chart
  does not know or understand the internals of the packages they install*."

HIP-0011 also explains why templating was removed: "If a chart installed a CRD, `helm`
no longer had a valid set of API versions to work against. This is also the reason
behind removing templating support from CRDs."
(paraphrased in
<https://helm.sh/docs/chart_best_practices/custom_resource_definitions/>)

The `crd-install` hook that existed in Helm 2 was **removed** in Helm 3 and replaced by
the `crds/` directory. (<https://helm.sh/community/hips/hip-0011/>; also
<https://helm.sh/docs/topics/v3_migration/>)

### Templated CRDs under `templates/` — the alternative

The best-practices page explicitly allows the other approach: CRDs can instead be placed
in `templates/` (Method 1 variant) or factored into a **separate chart** (Method 2):
"There are two approaches... The first method is to simply put the CRD in your
`templates/` directory... The second method for handling CRDs is to create a separate
chart."
(<https://helm.sh/docs/chart_best_practices/custom_resource_definitions/>)

Trade-off:

- **`crds/` directory:** safe-by-default, install-only, no templating, no
  conditionals, never upgraded, never removed. Good for "the CRD must exist before the
  first CR/template renders" and for avoiding accidental data loss. Bad for iterating on
  schema — every `helm upgrade` silently ignores your new CRD YAML.
- **Templated CRD in `templates/`:** upgraded and deleted like any other manifest
  (so schema changes actually roll out on `helm upgrade`), supports
  `{{ if .Values.crds.enabled }}` guards and value-injected fields (e.g. the webhook
  `caBundle`). Costs: `helm uninstall` will delete the CRD (and cascade-delete all CRs)
  unless you add `helm.sh/resource-policy: keep`; CRD YAML counts against the release
  secret size; ordering vs the rest of the release is not guaranteed without hooks.

### Helm v3 vs v4 — CRD handling is unchanged

As of Helm **4.2.4** the `crds/` behavior is identical to Helm 3. The v4.2.4 docs page
still carries the same "no support for upgrading or deleting CRDs" language and the same
three "never reinstalled / never on upgrade / never deleted" limitations
(<https://helm.sh/docs/chart_best_practices/custom_resource_definitions/>,
<https://helm.sh/docs/topics/charts/> — both served at doc version 4.2.4).

HIP-0011 flagged CRD handling as something to "revisit" for Helm 4 ("We are considering
options for a Helm 4 time frame, and this document is a first step in that exercise"),
but concluded that "Kubernetes is not yet mature enough for us to be able to do this."
(<https://helm.sh/community/hips/hip-0011/>)

The Helm 4 changelog only shows cosmetic/robustness CRD changes, not a behavior change:
"show crds command output separated by document separator" (PR #12624), "Add linter
support for the `crds/` directory" (PR #31015), "crd resources can be empty" (bugfix,
PR #31578) — all landing around v4.2.4.
(<https://helm.sh/docs/changelog/>)

Helm 4 is a pure in-place CLI upgrade — existing Helm 3 release secrets are read
unchanged, and `apiVersion: v2` charts run as-is. The `crds/` directory "same approach
continues to be used in Helm 4."
(<https://helm.sh/docs/overview/>; corroborated by third-party migration writeups, but
the primary confirmation is that the v4.2.4 docs pages above still describe the v3
behavior verbatim.)

One adjacent v3 feature worth knowing for CRD ownership migrations: `--take-ownership`
was added in **Helm 3.17** ("causes an upgrade to ignore the check for helm annotations
and take ownership of existing resources"). It is used when moving CRDs that were
previously `kubectl apply`-ed (or shipped in `crds/`) under templated Helm management —
the first `helm upgrade` needs `--take-ownership` to adopt them.
(<https://helm.sh/docs/helm/helm_upgrade/>; feature introduced in helm/helm 3.17
release notes.)

---

## 2. Handling CRD schema changes across `helm upgrade` in practice

Because `helm upgrade` never touches `crds/`, chart authors use one of the following.

### (a) Templated CRDs in `templates/` with `helm.sh/resource-policy: keep`

Put the CRD manifests under `templates/` (often gated by `{{- if .Values.crds.enabled }}`)
so `helm upgrade` re-applies the current schema, and annotate each CRD with
`helm.sh/resource-policy: keep` so `helm uninstall` does **not** cascade-delete it.

The annotation contract: "The annotation `helm.sh/resource-policy: keep` instructs Helm
to skip deleting this resource when a helm operation (such as `helm uninstall`,
`helm upgrade` or `helm rollback`) would result in its deletion. **However, this
resource becomes orphaned. Helm will no longer manage it in any way.**"
(<https://helm.sh/docs/howto/charts_tips_and_tricks/>)

- Pros: schema changes actually roll out on `helm upgrade`; one `helm upgrade` command;
  values can template fields (conditional CRDs, `caBundle`, extra labels); GitOps tools
  (Argo CD / Flux) diff and sync them normally.
- Cons: `keep` means the CRD drifts out of Helm's management the moment it would be
  deleted (orphaned); large CRDs bloat the release Secret (1 MB etcd / Secret limit);
  Helm applies CRDs in the same pass as everything else, so a CR in the same release can
  race the CRD unless ordering is forced; a *destructive* schema change (removing a
  served version, tightening validation) is still applied bluntly with no migration
  safety — Helm just does a 3-way merge / apply.
- This is the approach the **Kubebuilder `helm/v2-alpha` plugin** takes (see §5) and
  the approach the **prometheus-operator-crds** chart takes (below).

### (b) Separate CRD-only chart or subchart

Method 2 in the Helm docs: "put the CRD definition in one chart, and then put any
resources that use that CRD in *another* chart... this is a much more flexible method
if you want to apply updates or upgrades in the future."
(<https://helm.sh/docs/chart_best_practices/custom_resource_definitions/>)

Real example — **`prometheus-operator-crds`**
(<https://github.com/prometheus-community/helm-charts/tree/main/charts/prometheus-operator-crds>):
a standalone chart whose only job is the `monitoring.coreos.com` CRDs. CRDs are shipped
as **templated Helm templates** (under the chart's `charts/crds/templates/`), each
conditionally rendered via a `.Values.<crd>.enabled` flag, plus a shared
`crds.annotations` value. Because they are templates, a `helm upgrade` of this chart
updates the CRD schemas. A `hack/update_crds.sh` script pulls the CRD YAML from the
prometheus-operator repo at a pinned version.
(<https://deepwiki.com/prometheus-community/helm-charts/2.4-prometheus-operator-crds>;
values structure confirmed at
<https://raw.githubusercontent.com/prometheus-community/helm-charts/main/charts/prometheus-operator-crds/values.yaml>)

- Pros: CRD lifecycle is decoupled from the operator lifecycle; cluster admins can
  upgrade CRDs on their own cadence; multiple operator releases / namespaces can share
  one CRD chart; clean GitOps ownership boundary.
- Cons: two release objects to manage and order (CRD chart must be upgraded first); the
  operator chart must tolerate the CRD chart being absent or stale; version-skew
  matrix between the two.

### (c) Separate `kubectl apply` step (documented manual procedure)

**cert-manager** historically shipped its CRDs as a separate `cert-manager.crds.yaml`
that the admin `kubectl apply`s before/independently of `helm install` (still an option
alongside `--set crds.enabled=true`).
(<https://cert-manager.io/docs/installation/helm/>)

**kube-prometheus-stack** documents this as the primary CRD-upgrade path: "With Helm v3,
CRDs created by this chart are not updated by default and should be manually updated."
Its `UPGRADE.md` gives explicit per-release command blocks, e.g.:

```
kubectl apply --server-side -f https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/v0.93.0/example/prometheus-operator-crd/monitoring.coreos.com_alertmanagerconfigs.yaml
```

repeated for each CRD, run **before** `helm upgrade`.
(<https://raw.githubusercontent.com/prometheus-community/helm-charts/main/charts/kube-prometheus-stack/README.md>;
<https://github.com/prometheus-community/helm-charts/blob/main/charts/kube-prometheus-stack/UPGRADE.md>)

Note `--server-side` (server-side apply) is used deliberately: the prometheus CRDs are
large enough that a client-side `kubectl apply` blows the
`kubectl.kubernetes.io/last-applied-configuration` annotation size limit.

- Pros: fully explicit, admin sees exactly what changes, works with any deploy tool.
- Cons: out-of-band from Helm — easy to forget, drift-prone, hard to automate cleanly
  in GitOps; operator can silently misbehave (missing new fields) if the step is
  skipped.

### (d) CRD-upgrade Job driven by a Helm pre-upgrade hook

**kube-prometheus-stack** added `crds.upgradeJob.enabled` in chart version **68.4.0**:
"Since 68.4.0 it is also possible to use `crds.upgradeJob.enabled` for upgrading the
CRDs." It is implemented as "a pre-upgrade helm hook to update the CRD by triggering a
job that applies the latest CRD with `kubectl apply`."
(<https://github.com/prometheus-community/helm-charts/blob/main/charts/kube-prometheus-stack/UPGRADE.md>;
<https://github.com/prometheus-community/helm-charts/pull/5175>;
origin issue <https://github.com/prometheus-community/helm-charts/issues/704>)

Practical wrinkles reported upstream: server-side-apply conflicts on annotation keys
like `controller-gen.kubebuilder.io/version` and `operator.prometheus.io/version`, which
required adding a `forceConflicts` option (`kubectl apply --server-side
--force-conflicts`).
(<https://github.com/prometheus-community/helm-charts/pull/5175>)

- Pros: one `helm upgrade` command does everything; works in restricted / GitOps
  pipelines that only run Helm.
- Cons: needs a privileged ServiceAccount (cluster-wide `customresourcedefinitions`
  `patch`/`update`); a Job baked into the release with its own image; hook-weight /
  ordering and SSA-conflict handling to get right; failure modes are opaque
  (Job logs, not Helm output); Argo CD needs the hook annotations translated.

### (e) cert-manager's `startupapicheck` pattern (related but different problem)

cert-manager's Helm chart ships a post-install/post-upgrade Helm-hook Job called
`startupapicheck` that "waits for the cert-manager webhook to be ready" by attempting a
dry-run create of a `SelfSubjectAccessReview`/`Certificate`, so that `helm install`
doesn't return success until the webhook (and thus the API surface the CRDs depend on)
is actually serving.
(<https://cert-manager.io/docs/concepts/startupapicheck/> — historically titled
"startupapicheck"; behaviour also described in
<https://cert-manager.io/docs/installation/helm/>)

This does not upgrade CRDs; it solves the adjacent race where a chart (or the next
chart in a GitOps wave) creates a CR immediately after install, before the
validating/conversion webhook backing that CRD is reachable. Relevant to an operator
chart that also defines a conversion webhook: the same "is my webhook up before anyone
submits a CR" gap exists.

### CRD-size / annotation pitfalls common to all templated approaches

- CRDs with full structural schemas are large. Held in a `templates/` render they count
  against the Helm release Secret (etcd object) ~1 MB limit; applied client-side they
  exceed the `last-applied-configuration` annotation limit — hence server-side apply.
  (behaviour discussed in
  <https://github.com/prometheus-community/helm-charts/blob/main/charts/kube-prometheus-stack/UPGRADE.md>)
- `controller-gen` stamps `controller-gen.kubebuilder.io/version` on generated CRDs;
  mismatched tool versions between chart releases create SSA field-manager conflicts.

---

## 3. Conversion webhooks in a Helm-installed operator

### The CRD wiring (`spec.conversion`)

A CRD with more than one version and non-identical schemas needs
`spec.conversion.strategy: Webhook`. Default is `None`: "None conversion assumes the
same schema for all versions and only sets the `apiVersion` field of custom resources
to the proper value". "If the conversion involves schema changes and requires custom
logic, a conversion webhook should be used."
(<https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definition-versioning/>)

Shape:

```yaml
spec:
  conversion:
    strategy: Webhook
    webhook:
      conversionReviewVersions: ["v1"]      # first entry the API server also supports wins
      clientConfig:
        service:                            # in-cluster: Service reference
          namespace: <ns>
          name: <svc>
          path: /convert
          port: 443
        caBundle: <base64 CA cert>          # API server uses this to trust the webhook TLS
```

`clientConfig` is either a `service` reference (in-cluster) **or** a `url`
(`https://host:port/path`, for out-of-cluster) — not both.
(<https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definition-versioning/>)

The `caBundle` is mandatory for the API server to trust the webhook's serving cert:
"the requirement that the CRD have a `caBundle` for the API server to trust the
webhook". Deployment order matters: "Create and deploy the conversion webhook service
before updating the CRD" and the webhook "must present a valid certificate signed by
the CA in `caBundle`."
(<https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definition-versioning/>)

### Kubebuilder's model

Kubebuilder picks one API version as the **Hub** and the others as **spokes**; spokes
implement the `Convertible` interface (`ConvertTo()` / `ConvertFrom()` against the hub).
"Conversion webhooks use CRD conversion configuration
(`.spec.conversion.webhook.clientConfig.service.path` in the CRD) rather than webhook
marker annotations." "Conversion webhooks do not support custom paths via command-line
flags." A single webhook server (the manager) serves defaulting, validating and
conversion endpoints; "all that is needed is to wire up main to serve the webhook."
(<https://book.kubebuilder.io/multiversion-tutorial/conversion>)

Kubebuilder's generated kustomize config expresses the `caBundle` wiring declaratively:

- `config/crd/patches/` contains a `webhook_in_<resource>.yaml` patch (adds
  `spec.conversion`) and a `cainjection_in_<resource>.yaml` patch (adds the
  `cert-manager.io/inject-ca-from` annotation on the CRD). Both are commented out until
  you "uncomment all the sections with `[WEBHOOK]` prefix including the one in
  `crd/kustomization.yaml`".
  (<https://book.kubebuilder.io/cronjob-tutorial/running-webhook.html>)
- `config/certmanager/` contains a cert-manager `Certificate` + self-signed `Issuer`.
- kustomize `replacements` copy the Certificate's namespace/name into both the
  `cert-manager.io/inject-ca-from` annotation value and the webhook `Service` DNS SANs,
  so everything points at the same cert.
  (<https://book.kubebuilder.io/cronjob-tutorial/running-webhook.html>;
  <https://github.com/kubernetes-sigs/kubebuilder/blob/master/docs/book/src/cronjob-tutorial/cert-manager.md>)

### Serving-cert options

**Option A — cert-manager `Certificate` + cainjector (Kubebuilder's default recommendation).**
"Use cert-manager for provisioning the certificates for the webhook server."
cert-manager issues the serving cert into a Secret the manager mounts, rotates it in
place before expiry, and its **cainjector** component copies the CA into the consumer's
`caBundle`.
(<https://book.kubebuilder.io/cronjob-tutorial/cert-manager.html>;
<https://github.com/kubernetes-sigs/kubebuilder/blob/master/docs/book/src/cronjob-tutorial/cert-manager.md>)

The cainjector "helps to configure the CA certificates for: Mutating Webhooks,
Validating Webhooks, Conversion Webhooks and API Services" and writes the `caBundle`
field on `ValidatingWebhookConfiguration`, `MutatingWebhookConfiguration`,
`CustomResourceDefinition` (conversion), and `APIService`. Three annotation modes:

| Annotation | Source of CA | Notes |
|---|---|---|
| `cert-manager.io/inject-ca-from: <ns>/<certName>` | a cert-manager `Certificate` | ties `caBundle` to the Certificate's lifecycle |
| `cert-manager.io/inject-ca-from-secret: <ns>/<secretName>` | a Secret (needs `cert-manager.io/allow-direct-injection: "true"` on the Secret) | "will work without the cert-manager CRDs installed" — useful for bootstrap |
| `cert-manager.io/inject-apiserver-ca: "true"` | the cluster's own CA | webhook must serve a cert signed by the cluster CA |

(<https://cert-manager.io/docs/concepts/ca-injector/>)

"Every injectable resource must have one of the three annotations... to enable
automatic CA injection."
(<https://cert-manager.io/docs/concepts/ca-injector/>)

**Option B — Helm-templated self-signed cert (Sprig `genCA` / `genSignedCert`).**
A common pattern for charts that don't want a cert-manager dependency: a template
generates a CA and a serving cert with Sprig and writes both the Secret and the
`caBundle` in `spec.conversion` / webhook configs from the same generated value.

Sprig signatures:
`genCA "cn" days` -> `{Cert, Key}` (PEM); `genSignedCert "cn" ipList dnsList days $ca`
-> `{Cert, Key}`; `genSelfSignedCert "cn" ipList dnsList days`; `buildCustomCert
"b64cert" "b64key"`.
(<https://masterminds.github.io/sprig/crypto.html>)

Pitfall: these functions are **not deterministic** — every `helm upgrade` (and every
`helm template`) regenerates a fresh cert/CA, rotating it on every release and breaking
in-flight connections, unless the template first does a `lookup` of the existing Secret
and reuses it. Sprig itself does not address this; the reuse-via-`lookup` idiom is the
standard mitigation (e.g. Bitnami common helpers). `lookup` returns empty during
`helm template`/`--dry-run`, so CI renders differ from live.
(Sprig: <https://masterminds.github.io/sprig/crypto.html>; `lookup` semantics:
<https://helm.sh/docs/chart_template_guide/functions_and_pipelines/> — "the `lookup`
function... will return an empty ... value when `helm template` or `helm install
--dry-run` is used")

**Option C — built-in cert generation by the webhook server itself.**
controller-runtime's webhook server *can* self-provision: "The Secret doesn't need to be
prepopulated... if the Secret is empty, during bootstrapping the server will generate a
certificate and write it into the Secret."
(<https://book.kubebuilder.io/cronjob-tutorial/running-webhook.html> /
controller-runtime webhook package docs)

But controller-runtime **does not rotate or renew** that cert, and it does **not**
populate `caBundle` on the CRD / webhook configs — so on its own it's insufficient for a
conversion webhook. Kubebuilder therefore steers to cert-manager: "Use cert-manager for
provisioning the certificates for the webhook server. Other solutions should also work
as long as they put the certificates in the desired location."
(<https://github.com/kubernetes-sigs/kubebuilder/blob/master/docs/book/src/cronjob-tutorial/cert-manager.md>)

Third-party rotator libraries fill this gap where cert-manager isn't wanted:
`open-cluster-management/addon-framework` and `openshift/library-go`'s
`dynamiccertificates`, and the widely-used `qinqon/kube-admission-webhook` which
"rotate[s] the TLS cert/key pair... and inject[s] the CA bundle into the webhook
configuration and CRDs".
(<https://github.com/qinqon/kube-admission-webhook/blob/main/README.md>)

**Option D — operator patches its own CRD.**
The operator, on startup, PATCHes `spec.conversion.webhook.clientConfig.caBundle` on its
own CRD using a CA it generates or reads. Requires the operator's RBAC to include
`customresourcedefinitions` `get`/`patch` cluster-wide. Not a documented Kubebuilder
path, but used by some operators to avoid both cert-manager and Helm cert templating.

### How `caBundle` actually gets into `CustomResourceDefinition.spec.conversion.webhook.clientConfig`

Three mechanisms, mutually exclusive per CRD:

1. **cainjector** watches CRDs with `cert-manager.io/inject-ca-from` (or
   `-from-secret`) and writes `caBundle` after the fact.
   (<https://cert-manager.io/docs/concepts/ca-injector/>)
2. **Helm templating**: the chart renders `caBundle: {{ $ca.Cert | b64enc }}` directly
   into the CRD manifest from a value/generated cert. No controller needed; the field is
   correct at apply time. Requires the CRD to be a *template* (§1), not in `crds/`.
3. **Self-patching operator** (Option D).

With cert-manager's cainjector you get an ordering subtlety: Helm applies the CRD with
an empty/placeholder `caBundle`, then cainjector patches it moments later — a CR
`convert` call in that window fails. `startupapicheck`-style readiness gating (§2e) or
Argo/Flux sync waves address this.

---

## 4. What `helm uninstall` removes and does not remove

`helm uninstall` "removes all of the resources associated with the last release of the
chart as well as the release history". `--keep-history` keeps the release record.
(<https://helm.sh/docs/helm/helm_uninstall/>)

### CRDs

- **CRDs shipped in `crds/`: never removed.** "CRDs are never deleted... Consequently,
  Helm will not delete CRDs."
  (<https://helm.sh/docs/topics/charts/>) They become permanently orphaned — Helm never
  tracked them as release resources in the first place.
- **CRDs templated in `templates/`: removed on uninstall** (cascade-deleting every CR of
  that kind, cluster-wide) **unless** annotated `helm.sh/resource-policy: keep`, in
  which case they are left behind and orphaned ("Helm will no longer manage it in any
  way").
  (<https://helm.sh/docs/howto/charts_tips_and_tricks/>)

Real-chart behaviour:

- **cert-manager**: "the `CustomResourceDefinition` for `Issuers`, `ClusterIssuers`,
  `Certificates`, `CertificateRequests`, `Orders` and `Challenges` are not removed by
  the Helm uninstall command. This is to prevent data loss". Version note: **before
  v1.15.0** cert-manager's chart *did* delete CRDs on uninstall (data loss); `crds.keep`
  (default `true`) now controls this.
  (<https://cert-manager.io/docs/installation/upgrade/>;
  <https://cert-manager.io/docs/installation/helm/>)
- **kube-prometheus-stack / prometheus-operator-crds**: "CRDs created by this chart are
  not removed by default and should be manually cleaned up" with explicit
  `kubectl delete crd ...` for each of the ten kinds.
  (<https://raw.githubusercontent.com/prometheus-community/helm-charts/main/charts/kube-prometheus-stack/README.md>)

### Custom resources (CRs)

- CRs created by the chart's own `templates/` are deleted with the release like any
  manifest.
- CRs created *by users or by the operator* after install are **not** known to Helm and
  are not deleted by `helm uninstall` — but if the CRD is subsequently deleted, the API
  server garbage-collects **all** CRs of that kind in every namespace.
  (<https://helm.sh/community/hips/hip-0011/> — "Deleting a CRD... deletes all of the
  CRD's contents across all namespaces")

### Finalizer-stuck resources

If the operator sets finalizers on its CRs and the operator Deployment is removed first
by `helm uninstall`, nothing is left to process the finalizer: `kubectl delete` on those
CRs (or on the namespace) **hangs** until the finalizer is manually cleared
(`kubectl patch <cr> -p '{"metadata":{"finalizers":[]}}' --type=merge`). This is a
generic Kubernetes finalizer property, not Helm-specific.
(Kubernetes finalizer semantics:
<https://kubernetes.io/docs/concepts/overview/working-with-objects/finalizers/>)

### Namespaces

Helm only deletes a namespace if the chart itself templated the `Namespace` object.
Namespaces created via `helm install --create-namespace` are **not** removed by
`helm uninstall`. A namespace stuck `Terminating` almost always means a finalizer-stuck
CR (above) or a lingering `APIService`/webhook the API server can't reach.
(<https://helm.sh/docs/helm/helm_uninstall/>; `--create-namespace` behaviour:
<https://helm.sh/docs/helm/helm_install/>)

### Clean-teardown mental model (order matters)

1. Delete all CRs of the operator's kinds first, while the operator is still running, so
   finalizers are processed. (`kubectl delete fluke --all -A`)
2. If any CR hangs, remove its finalizers manually.
3. `helm uninstall <release>` — removes the operator Deployment, RBAC, webhook configs,
   Services, and (if templated without `keep`) the CRDs.
4. Only if you truly want the API types gone:
   `kubectl delete crd flukes.fluke.example.com` — this is the destructive,
   cluster-wide, irreversible step Helm deliberately refuses to do for you.
5. Delete the namespace if it was `--create-namespace`d.
6. Clean up any cert-manager `Certificate`/`Issuer` and leftover Secrets if they were in
   `crds/`-style un-managed locations.

(Order rationale synthesised from
<https://helm.sh/community/hips/hip-0011/> and
<https://cert-manager.io/docs/installation/upgrade/>.)

---

## 5. Is there a SUPPORTED Kubebuilder / operator-sdk Helm-packaging path?

### Kubebuilder `helm` plugin — alpha, evolving, not yet stable

- **`helm/v1-alpha`**: the first experimental Helm-chart scaffolder. **Now deprecated** —
  "use `helm/v2-alpha` instead."
  (<https://book.kubebuilder.io/plugins/available/helm-v1-alpha>)
- **`helm/v2-alpha`**: current experimental plugin. "Generates Helm charts from your
  project's kustomize output, letting you distribute your operator as either a bundle or
  a Helm chart." Still **alpha** ("This is experimental software").
  (<https://book.kubebuilder.io/plugins/available/helm-v2-alpha>;
  <https://raw.githubusercontent.com/kubernetes-sigs/kubebuilder/master/docs/book/src/plugins/available/helm-v2-alpha.md>)

  How it handles the lifecycle questions above:
  - **CRDs go in `templates/crd/`, not `crds/`** — deliberately. "Helm will install CRDs
    once during the initial release, but it will ignore CRD changes on subsequent
    upgrades." Placing them in `templates/` means "Helm treats CRDs like any other
    resource, ensuring they are applied and upgraded as expected"; the plugin
    "prioritizes correctness and maintainability over Helm's default convention."
  - **CR instances are excluded**: "The plugin ignores CR instances even if you add them
    to kustomize output" (so `config/samples/` is not shipped).
  - **No namespace management** — left to `helm install` flags.
  - **Webhooks auto-detected**; `--set webhook.enabled=false` is available, but "charts
    with CR version conversion reject `webhook.enabled=false`" because "the API server
    needs the webhook Service to convert custom resources."
  - **cert-manager integration** via `certManager.enabled`; generates a
    `selfsigned-issuer` + `serving-cert` under `cert-manager/`, wiring certs for webhook
    and metrics endpoints.
  - Regeneration: `Chart.yaml` and hand-edited files are preserved unless `--force`;
    `--force` is needed after webhook or API changes so `values.yaml` matches new
    manifests.
  (all quotes:
  <https://raw.githubusercontent.com/kubernetes-sigs/kubebuilder/master/docs/book/src/plugins/available/helm-v2-alpha.md>)

Verdict: Kubebuilder **does** offer an official Helm packaging path, but it is
**alpha** as of 2026 and marked experimental; there is no `stable` Helm plugin. Many
projects still hand-roll the chart or post-process `make manifests` / `kustomize build`
output.

### The kustomize -> Helm story

Kubebuilder's native, stable distribution artifact is the `config/` kustomize tree
(`kustomize build config/default` / `make deploy` / `make build-installer`). Helm
packaging sits on top of that via the alpha plugin. There is no separate "officially
blessed" kustomize->Helm converter beyond the `helm/v2-alpha` plugin itself.
(<https://book.kubebuilder.io/reference/artifacts>;
<https://book.kubebuilder.io/plugins/available/helm-v2-alpha>)

### operator-sdk "Helm operator" — a different thing

operator-sdk's **Helm-based operator** is *not* a way to package a Go operator as a
chart. It is "a Helm Based Operator" — a controller (using
`helm.operator-sdk`/`operator-lib`) that watches a CR and **reconciles a Helm chart's
release** in response, i.e. wrapping an existing chart as an operator. Opposite
direction.
(<https://sdk.operatorframework.io/docs/building-operators/helm/>)

operator-sdk's Go-operator distribution stories are OLM bundles and plain manifests, not
a first-party "operator-as-Helm-chart" generator. (operator-sdk and Kubebuilder share
the same underlying project scaffolding; the Helm chart plugin above is the shared
answer.)

### Summary for §5

| Tool | First-party "operator as a Helm chart"? | Status 2026 |
|---|---|---|
| Kubebuilder `helm/v1-alpha` | yes | **deprecated** |
| Kubebuilder `helm/v2-alpha` | yes (kustomize -> chart, CRDs in `templates/crd/`, cert-manager) | **alpha / experimental** |
| operator-sdk "Helm operator" | no — it's chart-as-controller, the reverse | stable, but not this use case |
| kustomize `config/` tree | n/a (not Helm) | stable, the canonical Kubebuilder artifact |

---

## Cross-cutting surprises / version-specific notes

- **Helm 4 changed nothing about `crds/`** — still install-only, still never
  upgraded/deleted, as of docs v4.2.4. The long-promised HIP-0011 revisit has not
  landed. (<https://helm.sh/docs/chart_best_practices/custom_resource_definitions/>)
- **Kubebuilder's own Helm plugin ignores Helm's `crds/` best practice on purpose**,
  putting CRDs in `templates/crd/` so upgrades work — a direct, documented disagreement
  with helm.sh guidance.
  (<https://raw.githubusercontent.com/kubernetes-sigs/kubebuilder/master/docs/book/src/plugins/available/helm-v2-alpha.md>)
- **cert-manager deleted CRDs on `helm uninstall` before v1.15.0** — a real historical
  data-loss footgun; `crds.keep=true` is now the default.
  (<https://cert-manager.io/docs/installation/upgrade/>)
- **controller-runtime generates but never rotates** webhook serving certs, and never
  sets `caBundle` — cert-manager (or a third-party rotator) is doing real work, not just
  convenience. (<https://github.com/kubernetes-sigs/kubebuilder/blob/master/docs/book/src/cronjob-tutorial/cert-manager.md>)
- **Sprig `genSignedCert` is non-deterministic** — the naive "self-signed cert in a
  template" pattern silently rotates the CA on every `helm upgrade` unless it reuses an
  existing Secret via `lookup`, and `lookup` is empty under `helm template`/`--dry-run`.
  (<https://masterminds.github.io/sprig/crypto.html>;
  <https://helm.sh/docs/chart_template_guide/functions_and_pipelines/>)
- **kube-prometheus-stack's `crds.upgradeJob`** (chart ≥ 68.4.0) is a concrete example
  of the "pre-upgrade hook Job runs `kubectl apply --server-side`" pattern, including
  the `--force-conflicts` follow-up they needed for `controller-gen` annotation
  conflicts.
  (<https://github.com/prometheus-community/helm-charts/pull/5175>)
- **`--take-ownership`** (Helm ≥ 3.17) is the migration lever for adopting previously
  `kubectl apply`-ed or `crds/`-installed CRDs into templated Helm management.
  (<https://helm.sh/docs/helm/helm_upgrade/>)
