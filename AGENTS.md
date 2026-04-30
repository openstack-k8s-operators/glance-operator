# AGENTS.md - glance-operator

## Project overview

glance-operator is a Kubernetes operator that manages
[OpenStack Glance](https://docs.openstack.org/glance/latest/) (the image
service: discovery, registration, and retrieval of VM images) on
OpenShift/Kubernetes. It is part of the
[openstack-k8s-operators](https://github.com/openstack-k8s-operators) project.

Key Glance domain concepts: **backends** (Ceph RBD, Cinder, Swift, NFS, S3),
**API types** (internal, external, single, edge), **image cache**,
**image import** (web-download, glance-direct, copy-image), **quotas**.

Go module: `github.com/openstack-k8s-operators/glance-operator`
API group: `glance.openstack.org`
API version: `v1beta1`

## Tech stack

| Layer | Technology |
|-------|------------|
| Language | Go (modules, multi-module workspace via `go.work`) |
| Scaffolding | [Kubebuilder v4](https://book.kubebuilder.io/) + [Operator SDK](https://sdk.operatorframework.io/) |
| CRD generation | controller-gen (DeepCopy, CRDs, RBAC, webhooks) |
| Config management | Kustomize |
| Packaging | OLM bundle |
| Testing | Ginkgo/Gomega + envtest (functional), KUTTL (integration) |
| Linting | golangci-lint (`.golangci.yaml`) |
| CI | Zuul (`zuul.d/`), Prow (`.ci-operator.yaml`), GitHub Actions |

## Custom Resources

| Kind | Purpose |
|------|---------|
| `Glance` | Top-level CR. Owns the database, keystone service, and spawns one or more `GlanceAPI` sub-CRs based on the requested layout (single, split, edge). |
| `GlanceAPI` | Manages a single Glance API deployment (StatefulSet, Service, endpoints). Created and owned by the `Glance` controller -- not intended to be created directly by users. |

Both CRDs have defaulting and validating admission webhooks.

## Directory structure

| Directory | Contents |
|-----------|----------|
| `api/v1beta1/` | CRD types (`glance_types.go`, `glanceapi_types.go`, `common_types.go`), conditions, webhook markers |
| `cmd/` | `main.go` entry point |
| `internal/controller/` | Reconcilers: `glance_controller.go`, `glanceapi_controller.go`, shared helpers |
| `internal/glance/` | Glance-level resource builders (db-sync Job, CronJob, PVC, volumes) |
| `internal/glanceapi/` | GlanceAPI-level resource builders (StatefulSet, cache job) |
| `internal/webhook/v1beta1/` | Webhook implementation |
| `templates/` | Config files and scripts mounted into pods via `OPERATOR_TEMPLATES` env var |
| `config/crd,rbac,manager,webhook/` | Generated Kubernetes manifests (CRDs, RBAC, deployment, webhooks) |
| `config/samples/` | Example CRs (Kustomize overlays). `layout/{single,split,edge,multiple}` for API topologies. `backends/{ceph,cinder,nfs,s3,swift,...}` for storage. Feature dirs: `image_cache/`, `quotas/`, `notifications/`, `copy_image/`, `import_plugins/`, `policy/`, others. |
| `test/functional/` | envtest-based Ginkgo/Gomega tests |
| `test/kuttl/` | KUTTL integration tests |
| `hack/` | Helper scripts (CRD schema checker, local webhook runner) |
| `docs/` | Design decisions, troubleshooting, probes documentation |

## Build commands

After modifying Go code, always run: `make generate manifests fmt vet`.

## Code style guidelines

- Follow standard openstack-k8s-operators conventions and lib-common patterns.
- Use `lib-common` modules for conditions, endpoints, TLS, storage, and other
  cross-cutting concerns rather than re-implementing them.
- CRD types go in `api/v1beta1/`. Controller logic goes in
  `internal/controller/`. Resource-building helpers go in `internal/glance/`
  or `internal/glanceapi/`.
- Config templates are plain files in `templates/` -- they are mounted at
  runtime via the `OPERATOR_TEMPLATES` environment variable.
- Webhook logic is split between the kubebuilder markers in `api/v1beta1/` and
  the implementation in `internal/webhook/v1beta1/`.

## Testing

- Functional tests use the envtest framework with Ginkgo/Gomega and live in
  `test/functional/`.
- KUTTL integration tests live in `test/kuttl/`.
- Run all functional tests: `make test`.
- When adding a new field or feature, add corresponding test cases in
  `test/functional/` and update `test/functional/glance_test_data.go` with
  fixture data.
- When modifying samples, `test/functional/sample_test.go` validates that all
  sample CRs can be parsed.

## Key dependencies

- [lib-common](https://github.com/openstack-k8s-operators/lib-common): shared modules for conditions, endpoints, database, TLS, secrets, etc.
- [infra-operator](https://github.com/openstack-k8s-operators/infra-operator): RabbitMQ and topology APIs.
- [mariadb-operator](https://github.com/openstack-k8s-operators/mariadb-operator): database provisioning.
- [keystone-operator](https://github.com/openstack-k8s-operators/keystone-operator): identity service registration.
- [gophercloud](https://github.com/gophercloud/gophercloud): Go OpenStack SDK used for Keystone endpoint operations.
