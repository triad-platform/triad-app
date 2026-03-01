# deploy/k8s

Kubernetes base manifests for the Phase 2 AWS-first PulseCart deployment.

## What Is Real Now

1. Namespace shape
2. Deployment and Service objects for:
   - api-gateway
   - orders
   - notifications
   - worker
3. Shared ConfigMap and Secret contract
4. Ingress placeholder

## What Is Still Placeholder

1. Container images
   - The ECR registry is now real (`971146591534.dkr.ecr.us-east-1.amazonaws.com`)
   - These base manifests intentionally use service-specific mutable branch tags for the first dev environment (for example `orders-develop`)
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

## Do Not Apply Yet

Do not apply these manifests to a cluster yet unless all of the following are true:

1. Real image names/tags exist and are pushable by CI
   - ensure the service-specific `*-develop` tags have been pushed at least once for each service
2. The platform add-ons (ingress, NATS, metrics path) exist in `triad-kubernetes-platform`
3. The data service DNS names are replaced with real AWS or in-cluster targets
   - RDS hostname
   - ElastiCache hostname
4. The secret values are replaced with real environment values
   - `DATABASE_URL` still requires the password from the RDS Secrets Manager secret

## Current Manual Secret Step

The RDS password is no longer stored in Terraform input files.

For now:

1. Read the generated master password from the Secrets Manager secret referenced by `rds_master_user_secret_arn`
2. Construct `DATABASE_URL`
3. Create the Kubernetes `pulsecart-secrets` Secret with that value

This manual step is temporary until the platform adds a proper secret synchronization path.

## Safe Next Use

These manifests are ready to serve as the GitOps target for:

- `/Users/lseino/triad-platform/triad-kubernetes-platform/apps/workloads/pulsecart-workloads.yaml`

Once real image and dependency values exist, this path becomes the first deployable app workload set.
