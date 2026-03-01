# triad-app (PulseCart)

PulseCart is a real-world reference app deployed onto the Triad multi-cloud platform.
It is intentionally designed to demonstrate:
- distributed systems boundaries
- reliability/SLO thinking
- secure delivery pipelines
- operational readiness

Services:
- api-gateway
- auth
- orders
- inventory
- notifications
- worker

CI:
- `.github/workflows/ci-tests.yml` runs Go service tests on `pull_request` to `develop`/`main` and pushes to `develop`.
- `.github/workflows/e2e-local.yml` is a manual `workflow_dispatch` job that runs `make e2e`.
- `.github/workflows/e2e-cloud.yml` remains available as a manual smoke trigger from the app repo.
- `.github/workflows/build-and-push-ecr.yml` builds service images on `develop` pushes, then updates the GitOps overlay in `triad-kubernetes-platform` to ECR image digests in the same workflow so ArgoCD only sees deployable immutable image references.
  - This workflow requires a GitHub secret named `TRIAD_PLATFORM_GITOPS_PAT` with write access to `triad-kubernetes-platform`.

Branching model:
- Day-to-day development happens on `develop`.
- `main` is updated by merging `develop` after validation.
- Feature branches are optional and can be introduced later as team size/scope grows.

Phase 2 deployment contract:
- `docs/deployment/000-aws-first-deployment-contract.md` defines the AWS-first handoff between app, landing zone, and Kubernetes platform repos.
- `deploy/k8s/` contains the first workload manifest scaffold for GitOps consumption.
