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
   - Redis, NATS, and Postgres DNS names are placeholders until the platform layer is wired
3. Ingress host
   - `pulsecart-dev.example.com` is a placeholder
4. Ingress class
   - currently `nginx` as a placeholder; this may become ALB in AWS

## Do Not Apply Yet

Do not apply these manifests to a cluster yet unless all of the following are true:

1. Real image names/tags exist and are pushable by CI
   - ensure the service-specific `*-develop` tags have been pushed at least once for each service
2. The platform add-ons (ingress, NATS, metrics path) exist in `triad-kubernetes-platform`
3. The data service DNS names are replaced with real AWS or in-cluster targets
4. The secret values are replaced with real environment values

## Safe Next Use

These manifests are ready to serve as the GitOps target for:

- `/Users/lseino/triad-platform/triad-kubernetes-platform/apps/workloads/pulsecart-workloads.yaml`

Once real image and dependency values exist, this path becomes the first deployable app workload set.
