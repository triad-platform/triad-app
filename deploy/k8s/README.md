# deploy/k8s

Kubernetes base manifests for the Phase 2 AWS-first PulseCart deployment.

## What Is Real Now

1. Namespace shape
2. Deployment and Service objects for:
   - api-gateway
   - orders
   - notifications
   - worker
3. Shared ConfigMap plus ExternalSecret contract
4. Ingress placeholder

## What Is Still Placeholder

1. Container images
   - The ECR registry is now real (`971146591534.dkr.ecr.us-east-1.amazonaws.com`)
   - CI now promotes these manifests to immutable service-specific branch-SHA tags (for example `orders-develop-<12-char-sha>`) after image push succeeds
   - The base manifests may temporarily show a branch tag or the last promoted immutable tag depending on where the latest CI promotion landed
   - `imagePullPolicy: Always` remains in place for safe dev convergence
   - CI publishes three dev tag forms:
     - `<service>-develop` (mutable branch tag)
     - `<service>-sha-<12-char-sha>` (immutable)
     - `<service>-develop-<12-char-sha>` (immutable but easier to identify in branch history)
2. External dependency endpoints
   - Redis now points at the real Phase 2 ElastiCache endpoint
   - `REDIS_TLS_ENABLED=true` is required for the current ElastiCache configuration
   - Postgres now points at the real Phase 2 RDS endpoint with TLS required
   - NATS remains cluster-local in Phase 2
3. Ingress host
   - `pulsecart-dev.cloudevopsguru.com` is the current Phase 2 target host
4. Ingress class
   - `alb` is the current Phase 2 target ingress class
5. TLS
   - the ingress now references the real ACM certificate ARN for the dev hostname
6. DNS automation
   - the ingress now includes the `external-dns` hostname annotation for future Route 53 automation

## Do Not Apply Yet

Do not apply these manifests to a cluster yet unless all of the following are true:

1. Real image names/tags exist and are pushable by CI
   - ensure the service-specific `*-develop` tags have been pushed at least once for each service
2. The platform add-ons (ingress, NATS, metrics path) exist in `triad-kubernetes-platform`
3. The data service DNS names are replaced with real AWS or in-cluster targets
   - RDS hostname
   - ElastiCache hostname
4. The secret values are replaced with real environment values
   - `DB_PASSWORD` still requires the password from the RDS Secrets Manager secret

## Secret Synchronization Path

The RDS password is no longer stored in Terraform input files.

Base manifests now assume:

1. `external-secrets` is installed in the cluster
2. `deploy/k8s/secretstore.yaml` points at AWS Secrets Manager in `us-east-1`
3. `deploy/k8s/externalsecret-db-password.yaml` syncs the live RDS master password into `pulsecart-secrets`
4. `orders` builds its Postgres DSN in-process from:
   - `DB_HOST`
   - `DB_PORT`
   - `DB_NAME`
   - `DB_USER`
   - `DB_PASSWORD`
   - `DB_SSLMODE`

`secret.example.yaml` remains only as a local fallback example and should not be part of the base `kustomization.yaml`.

## Safe Next Use

These manifests are ready to serve as the GitOps target for:

- `/Users/lseino/triad-platform/triad-kubernetes-platform/apps/workloads/pulsecart-workloads.yaml`

Once real image and dependency values exist, this path becomes the first deployable app workload set.
