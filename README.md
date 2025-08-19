# Platform Engineer Assessment

> End-to-end platform: **Terraform** provisions an EC2 host, **Ansible** installs **K3s**, and **GitHub Actions** (CI/CD) runs on every push to `main` to **test → build & push → deploy** a Go API to the cluster.  
> The app exposes `/healthz`, a JWT-protected `/analyze` API, and embedded **OpenAPI docs** at `/docs`.

---

## Architecture

```
┌────────────────────────────┐
│ GitHub (main branch)       │
│  • CI: unit tests          │
│  • Build: Docker (sha/latest) ─────┐  docker.io/<you>/go-analyzer:sha|latest
│  • CD: kubectl apply + smoke tests │
└────────────────────────────┘       │
                                     ▼
                         ┌─────────────────────────────┐
                         │ AWS EC2 (Ubuntu, t3.micro)  │
                         │  • K3s (single-node)        │
                         │  • containerd               │
                         │  • NodePort :30080          │
                         └──────────┬──────────────────┘
                                     │
                       ┌─────────────┴─────────────┐
                       │ K3s Workload              │
                       │  • Deployment: go-analyzer│
                       │  • Service: NodePort 30080│
                       │  • Secret: JWT_SECRET     │
                       └───────────────────────────┘
```

---

## Repository layout

```
.
├── terraform/                # Optional one-time infra (EC2, SG, keypair)
├── ansible/                  # Optional one-time k3s bootstrap (k3s, kubeconfig)
├── go-app/
│   ├── main.go               # HTTP server + JWT + /docs + /openapi.yaml
│   ├── go.mod
│   └── Dockerfile            # scratch final; multi-arch capable
├── k8s/
│   ├── deployment.yaml       # image: IMAGE_PLACEHOLDER (replaced by CD)
│   └── service.yaml          # NodePort 30080
└── .github/workflows/
    └── cd-main.yml           # tests → build → push → deploy on push to main
```

---

## What’s implemented

- **Single-node K3s** on EC2 (Ubuntu) with kubeconfig at `/home/ubuntu/.kube/config`.
- **Go API** with endpoints:
  - `GET /healthz` — liveness (no auth)
  - `GET /analyze?sentence=...` — **JWT required**, returns counts
  - `POST /analyze` — **JWT required**, JSON body `{ "sentence": "..." }`
  - `GET /docs` & `GET /docs/` — Swagger UI
  - `GET /openapi.yaml` — OpenAPI 3 spec (embedded; no file needed in image)
- **JWT Auth (HS256)**:
  - Requires `Authorization: Bearer <JWT>`
  - Claims: `role` must be `user` or `admin`
  - Secret comes from K8s Secret `go-analyzer-secrets` → env `JWT_SECRET`
- **CD pipeline**:
  - Unit tests (`go test`)
  - Build & push image to Docker Hub (`sha` + `latest`)
  - Resolve EC2 by tag, scp kubeconfig, apply secret, inject image, `kubectl apply`

---

## Required GitHub Secrets

| Secret                    | Purpose                                           |
|---------------------------|---------------------------------------------------|
| `DOCKERHUB_USERNAME`      | Docker Hub account name                           |
| `DOCKERHUB_TOKEN`         | Docker Hub access token                           |
| `AWS_ACCESS_KEY_ID`       | AWS creds (to query EC2 IP by tag)                |
| `AWS_SECRET_ACCESS_KEY`   | AWS creds (to query EC2 IP by tag)                |
| `AWS_REGION`              | (optional) defaults to `us-east-1`                |
| `SSH_PRIVATE_KEY`         | Private key that can SSH into the EC2 instance    |
| `JWT_SECRET`              | HMAC secret used to validate JWTs                 |
| `DOCKER_REGISTRY`         | (optional) defaults to `docker.io`                |

> The deploy job finds the instance tagged `Name=pe-assessment-ec2` (configurable via `NAME_PREFIX` env).

---

## Local development

```bash
# run unit tests
cd go-app
go test ./... -v

# run the app locally
export JWT_SECRET=dev-secret
go run .
```

Open endpoints:

```bash
curl -i http://localhost:8080/healthz   # 200 OK
curl -i http://localhost:8080/docs      # Swagger UI
```

Create a **JWT** (pure OpenSSL, no deps):

```bash
SECRET=dev-secret
NOW=$(date +%s); EXP=$((NOW+600))
HDR=$(printf '{"alg":"HS256","typ":"JWT"}' | openssl base64 -A | tr '+/' '-_' | tr -d '=')
PAY=$(printf '{"role":"user","exp":%s}' "$EXP" | openssl base64 -A | tr '+/' '-_' | tr -d '=')
SIG=$(printf '%s' "$HDR.$PAY" | openssl dgst -binary -sha256 -hmac "$SECRET" | openssl base64 -A | tr '+/' '-_' | tr -d '=')
TOKEN="$HDR.$PAY.$SIG"

# test GET
curl -i -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/analyze?sentence=Hello%20Authz"

# test POST
curl -i -X POST "http://localhost:8080/analyze" \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"sentence":"Platform Engineer FTW"}'
```

---

## Build & run container locally

```bash
DOCKER_USER=<your-dockerhub-username>
docker build -t docker.io/$DOCKER_USER/go-analyzer:dev -f go-app/Dockerfile ./go-app
docker run --rm -e JWT_SECRET=dev-secret -p 8080:8080 docker.io/$DOCKER_USER/go-analyzer:dev
```

---

## Kubernetes (K3s)

### Service exposure

`k8s/service.yaml` exposes a **NodePort** on **30080**. Security group must allow inbound TCP `30080` from your IP (or 0.0.0.0/0 for demo).

### Runtime secret

The deployment expects `JWT_SECRET` via the K8s Secret **`go-analyzer-secrets`**. The CD pipeline creates/patches it on every deploy:

```bash
kubectl create secret generic go-analyzer-secrets \
  --from-literal=JWT_SECRET="${JWT_SECRET}" \
  --dry-run=client -o yaml | kubectl apply -f -
```

Rotate by changing the repo secret and running a new deploy (deployment restarts).

---

## CI/CD (push to `main`)

Workflow: `.github/workflows/cd-main.yml`

- **unit_tests** → `go test ./... -v -cover`
- **build_push** → build multi-arch image and push:
  - `docker.io/${DOCKERHUB_USERNAME}/go-analyzer:${{ github.sha }}`
  - `docker.io/${DOCKERHUB_USERNAME}/go-analyzer:latest`
- **deploy**:
  - Resolve EC2 public IP by tag `Name=pe-assessment-ec2`
  - `scp` kubeconfig from `/home/ubuntu/.kube/config`
  - Apply `go-analyzer-secrets` with `JWT_SECRET`
  - Inject built **SHA** image into `k8s/deployment.yaml`
  - `kubectl apply -f k8s/` + wait for rollout
  - **Smoke tests**:
    - `GET /healthz` → **200**
    - `GET /analyze` (no token) → **401**
    - `GET /analyze` (with minted token) → **200**

> Terraform/Ansible are kept separate (one-time). Running them on every push is **not** recommended; run only when infra/bootstrap code changes.

---

## Verification & evidence (from EC2)

```bash
alias k='sudo k3s kubectl'

# Prove K3s
k3s --version
k get nodes -o wide
k get pods -A -o wide | head -n 30

# Prove deployment & service
k get deploy go-analyzer -o wide
k get svc go-analyzer -o wide
k get endpoints go-analyzer

# App logs
k logs deploy/go-analyzer --tail=200

# HTTP checks
curl -i http://127.0.0.1:30080/healthz
curl -i http://127.0.0.1:30080/analyze?sentence=hi   # expect 401
```

Mint a token with the cluster secret and call the API:

```bash
SECRET=$(sudo k3s kubectl get secret go-analyzer-secrets -o jsonpath='{.data.JWT_SECRET}' | base64 -d)
NOW=$(date +%s); EXP=$((NOW+300))
HDR=$(printf '{"alg":"HS256","typ":"JWT"}' | openssl base64 -A | tr '+/' '-_' | tr -d '=')
PAY=$(printf '{"role":"user","exp":%s}' "$EXP" | openssl base64 -A | tr '+/' '-_' | tr -d '=')
SIG=$(printf '%s' "$HDR.$PAY" | openssl dgst -binary -sha256 -hmac "$SECRET" | openssl base64 -A | tr '+/' '-_' | tr -d '=')
TOKEN="$HDR.$PAY.$SIG"

curl -i -H "Authorization: Bearer $TOKEN" \
  "http://127.0.0.1:30080/analyze?sentence=Hello%20from%20EC2"
```

---

## Troubleshooting

- **`/docs` returns 404**  
  The pod is running an older image. Ensure the CD injected the new image and pulled it:
  ```bash
  kubectl get deploy go-analyzer -o jsonpath='{.spec.template.spec.containers[0].image}{"\n"}'
  kubectl rollout restart deploy/go-analyzer
  ```

- **`invalid token`**  
  Token not signed with current `JWT_SECRET`, wrong alg, or expired. Mint a token using the cluster secret (see above).

- **`ImagePullBackOff`**  
  Tag doesn’t exist or registry credentials missing on node. Verify the image exists in Docker Hub and deployment uses the exact tag.

- **Rollout stuck / old pod terminating**  
  ```bash
  kubectl delete pod <pod> --grace-period=0 --force
  # or
  kubectl scale deploy/go-analyzer --replicas=0 && sleep 5 && kubectl scale --replicas=1
  ```

- **Port closed externally**  
  Ensure the EC2 Security Group allows inbound TCP **30080** from your source IP.

---

## Security notes & next steps

- Prefer **GitHub OIDC → AWS IAM role** over long-lived keys.
- Add TLS + Ingress (Traefik is in K3s) and cert-manager for Let’s Encrypt.
- Add HPA to autoscale the deployment.
- Move secrets to AWS Secrets Manager/SSM via CSI driver.
- Split infra and app pipelines; add path filters + approvals for Terraform.
