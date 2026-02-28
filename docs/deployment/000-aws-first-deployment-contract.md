# AWS-First Deployment Contract (Phase 2)

This document defines the minimum deployment contract between:

1. `triad-app`
2. `triad-landing-zones`
3. `triad-kubernetes-platform`

The goal is to move the proven Phase 1 vertical slice into AWS without changing core application behavior.

## Scope

Phase 2 deploys the existing PulseCart vertical slice to AWS only:

- `api-gateway`
- `orders`
- `worker`
- `notifications`

Deferred for later:

- `auth`
- `inventory`
- multi-cloud parity
- advanced traffic management

## Application Contract (`triad-app`)

The application layer must provide:

1. Containerizable services
   - one image per service
   - clear start command
   - health endpoints where applicable

2. Stable runtime endpoints
   - `api-gateway`: `:8080`
   - `orders`: `:8081`
   - `notifications`: `:8082`
   - `worker`: metrics on `:9091`

3. Stable dependency contract
   - Postgres via `DATABASE_URL`
   - Redis via `REDIS_ADDR`
   - NATS via `NATS_URL`
   - Notifications internal HTTP via `NOTIFICATIONS_URL`

4. Kubernetes readiness expectations
   - HTTP services expose `/healthz`, `/readyz`, `/metrics`
   - worker exposes `/metrics` on dedicated metrics port
   - services tolerate restart without duplicate business side effects

5. Deployment verification
   - equivalent of local `make e2e` can be executed against the cluster

## Landing Zone Contract (`triad-landing-zones`)

The landing zone layer must provide:

1. AWS account/environment boundary
   - dev environment first

2. Network foundation
   - VPC
   - public subnets for ingress/load balancer
   - private subnets for EKS nodes and data services
   - outbound path for image pulls and package fetches

3. Identity foundation
   - IAM boundary suitable for EKS OIDC
   - separation between cluster infra roles and workload roles

4. Managed data service baseline
   - RDS PostgreSQL
   - ElastiCache Redis

5. DNS/TLS prerequisites
   - a routable DNS zone strategy for dev ingress
   - ACM/cert-manager compatible path

## Kubernetes Platform Contract (`triad-kubernetes-platform`)

The platform layer must provide:

1. EKS cluster baseline
   - one dev cluster first
   - OIDC enabled for IRSA-compatible future state

2. GitOps baseline
   - ArgoCD bootstrap
   - app-of-apps root for platform + app workloads

3. Core add-ons
   - ingress controller
   - cert-manager
   - metrics scraping path for `/metrics`

4. Namespace and workload standards
   - app namespace for PulseCart
   - per-service Deployment + Service
   - ConfigMap/Secret contract for runtime env

5. Messaging runtime
   - NATS initially self-hosted in cluster

## Runtime Environment Contract

Minimum required runtime configuration:

1. `api-gateway`
   - `ORDERS_URL`

2. `orders`
   - `DATABASE_URL`
   - `REDIS_ADDR`
   - `NATS_URL`

3. `worker`
   - `REDIS_ADDR`
   - `NATS_URL`
   - `NOTIFICATIONS_URL`
   - `WORKER_METRICS_PORT`

4. `notifications`
   - `PORT`

## Phase 2 Definition of Ready

Phase 2 implementation can begin when all are true:

1. Container build contract is documented for each active service.
2. AWS dev network and IAM assumptions are written down.
3. EKS add-on list is fixed for first deployment.
4. GitOps repo layout for app manifests is decided.
5. Smoke criteria for deployed vertical slice are agreed.

## Phase 2 Exit Criteria

Phase 2 is complete when:

1. A public endpoint reaches `api-gateway`.
2. The full order flow completes in AWS.
3. Duplicate protection still works.
4. Metrics remain reachable for gateway, orders, and worker.
5. Restart/redeploy does not create duplicate side effects.
