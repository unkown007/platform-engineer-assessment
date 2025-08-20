# Platform Engineer Assessment — Final Version

> A small but complete platform: **Terraform** provisions the host, **Ansible** bootstraps **K3s**, **GitHub Actions** runs **tests → code quality → build/push → deploy**, and we have **monitoring (Prometheus/Grafana/Alertmanager)**. The Go API exposes `/healthz`, a JWT-protected `/analyze` (GET/POST), and **OpenAPI docs** at `/docs`.

---

## Architecture

```
┌──────────────────────────────────────┐
│ GitHub (main)                        │
│  1) Unit tests (coverage)            │
│  2) Sonar (quality gate)             │
│  3) Build & Push image               │
│  4) Deploy to K3s (kubectl apply)    │
└──────────────────────────────────────┘
                    │  docker.io/<you>/go-analyzer:sha|latest
                    ▼
           ┌─────────────────────────────┐
           │ AWS EC2 (Ubuntu, t3.micro)  │
           │  • K3s single-node          │
           │  • NodePort: 30080          │
           └──────────┬──────────────────┘
                      │
        ┌─────────────┴─────────────┐
        │ K3s Workload               │
        │  • Deployment: go-analyzer │
        │  • Service: NodePort 30080 │
        │  • Secret: JWT_SECRET      │
        │  • /metrics exposed        │
        └────────────────────────────┘
                      │
        ┌─────────────┴─────────────┐
        │ Monitoring (monitoring ns) │
        │  • kube-prometheus-stack   │
        │  • Grafana NodePort 30090  │
        │  • ServiceMonitor + Alerts │
        └────────────────────────────┘
```

---

## Repository Layout

```
.
├── terraform/                  # One-time infra (EC2, SG, keypair)
├── ansible/                    # One-time K3s bootstrap (k3s, kubeconfig)
├── go-app/
│   ├── main.go                 # HTTP + JWT + /docs + /openapi.yaml + /metrics
│   ├── go.mod                  # go 1.23
│   └── Dockerfile              # multi-stage, golang:1.23-alpine → scratch
├── k8s/
│   ├── deployment.yaml         # uses image from CI, reads secret JWT_SECRET
│   └── service.yaml            # NodePort 30080 → container 8080
├── monitoring/
│   ├── servicemonitor-go.yaml  # Prometheus scrape for /metrics
│   └── alerts-go.yaml          # Example PrometheusRule alerts
├── sonar-project.properties     # Sonar configuration (project/org/coverage)
└── .github/workflows/
    └── cd-main.yml             # tests → sonar → build → deploy
    └── platform.yml            # terraform → ansible → tests → sonar → build → deploy
```

---

## Application Endpoints

- `GET /healthz` — liveness (no auth)
- `GET /analyze?sentence=...` — **JWT required**
- `POST /analyze` — **JWT required**, body `{"sentence":"..."}`
- `GET /docs` and `GET /docs/` — Swagger UI
- `GET /openapi.yaml` — OpenAPI 3 spec (embedded, served by app)
- `GET /metrics` — Prometheus metrics (internal scrape; no auth)

### JWT
- Alg **HS256**. Env var **`JWT_SECRET`** provided via K8s Secret.
- Claim `role` must be `user` or `admin` (used by middleware).

---

## Local Dev & Tests

```bash
cd go-app

# Run unit tests with coverage
go test ./... -v -coverprofile=coverage.out

# Run locally
export JWT_SECRET=dev-secret
go run .
```

Quick calls:
```bash
curl -i http://localhost:8080/healthz

# Mint a short-lived JWT (OpenSSL)
SECRET=dev-secret
NOW=$(date +%s); EXP=$((NOW+600))
HDR=$(printf '{"alg":"HS256","typ":"JWT"}' | openssl base64 -A | tr '+/' '-_' | tr -d '=')
PAY=$(printf '{"role":"user","exp":%s}' "$EXP" | openssl base64 -A | tr '+/' '-_' | tr -d '=')
SIG=$(printf '%s' "$HDR.$PAY" | openssl dgst -binary -sha256 -hmac "$SECRET" | openssl base64 -A | tr '+/' '-_' | tr -d '=')
TOKEN="$HDR.$PAY.$SIG"

curl -i -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/analyze?sentence=Hello%20Authz"

curl -i -X POST "http://localhost:8080/analyze" \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"sentence":"Platform Engineer FTW"}'

# Docs
# macOS: open http://localhost:8080/docs
# Linux: xdg-open http://localhost:8080/docs
```

---

## Container

```bash
DOCKER_USER=<your-dockerhub-username>
docker build -t docker.io/$DOCKER_USER/go-analyzer:dev -f go-app/Dockerfile ./go-app
docker run --rm -e JWT_SECRET=dev-secret -p 8080:8080 docker.io/$DOCKER_USER/go-analyzer:dev
```

Dockerfile highlights:
- Build stage: `golang:1.23-alpine`, cached `go mod download`, static build.
- Runtime: `scratch`, `USER 65532`, `ENTRYPOINT ["/app"]`.

---

## Kubernetes on EC2 (K3s)

- **NodePort**: Service exposes **30080** → container **8080**.
- Ensure the EC2 **Security Group** allows TCP **30080** (and **22** for SSH) from your IP.
- Kubeconfig lives at `/home/ubuntu/.kube/config` (copied by Ansible).

Evidence commands on the host:
```bash
alias k='sudo k3s kubectl'
k3s --version
k get nodes -o wide
k get deploy go-analyzer -o wide
k get svc go-analyzer -o wide
k get endpoints go-analyzer
k logs deploy/go-analyzer --tail=200
```

---

## CI/CD (on push to `main`)

Workflow: `.github/workflows/cd-main.yml`

1. **unit_tests** — `go test ./... -v -coverprofile=go-app/coverage.out`
2. **sonar** — SonarCloud scan with **quality gate** (disable Sonar “Automatic Analysis” to avoid conflicts)
3. **build_push** — build multi-arch image and push:
   - `docker.io/${DOCKERHUB_USERNAME}/go-analyzer:${{ github.sha }}`
   - `docker.io/${DOCKERHUB_USERNAME}/go-analyzer:latest`
4. **deploy** — resolve EC2 IP by tag (`Name=pe-assessment-ec2`), fetch kubeconfig, create/patch `go-analyzer-secrets` (JWT), inject image, `kubectl apply`, wait for rollout, smoke tests.

> **Terraform/Ansible** are one-time/bootstrap and should not run on every push.

### Required GitHub Secrets

| Secret                    | Purpose                                              |
|---------------------------|------------------------------------------------------|
| `DOCKERHUB_USERNAME`      | Docker Hub account                                   |
| `DOCKERHUB_TOKEN`         | Docker Hub token                                     |
| `AWS_ACCESS_KEY_ID`       | EC2 IP lookup via AWS CLI                            |
| `AWS_SECRET_ACCESS_KEY`   | EC2 IP lookup via AWS CLI                            |
| `AWS_REGION`              | e.g., `us-east-1`                                    |
| `SSH_PRIVATE_KEY`         | SSH key for `ubuntu@EC2` (scp kubeconfig)            |
| `JWT_SECRET`              | HMAC secret for API JWT                              |
| `SONAR_TOKEN`             | SonarCloud (or SonarQube) user token                 |
| `SONAR_HOST_URL`          | Only for self-hosted SonarQube                       |
| `DOCKER_REGISTRY`         | Optional; defaults to `docker.io`                    |
| `GRAFANA_ADMIN_PASSWORD`  | Optional; admin password for Grafana (monitoring)    |

---

## Sonar Integration

`sonar-project.properties` at repo root (SonarCloud example):
```properties
sonar.projectKey=YOUR_ORG_KEY_go-analyzer
sonar.projectName=go-analyzer
sonar.organization=YOUR_ORG_KEY

sonar.sources=go-app
sonar.tests=go-app
sonar.test.inclusions=go-app/**/*_test.go
sonar.go.coverage.reportPaths=go-app/coverage.out
```

Tips:
- Disable **Automatic Analysis** in SonarCloud (use CI-based analysis), otherwise you’ll get: *“You are running CI analysis while Automatic Analysis is enabled.”*
- `actions/checkout@v4` must use `fetch-depth: 0` for PR decoration/blame.

---

## Monitoring (Prometheus + Grafana)

We use **kube-prometheus-stack** via Helm, with a lightweight config for a t3.micro.

### On the EC2 host
```bash
alias k='sudo k3s kubectl'

# Install Helm (one of the methods)
curl -fsSL https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash

# Namespace + Grafana admin
k create namespace monitoring --dry-run=client -o yaml | k apply -f -
k -n monitoring create secret generic grafana-admin \
  --from-literal=admin-user=admin \
  --from-literal=admin-password='ChangeMe!' \
  --dry-run=client -o yaml | k apply -f -

# Add repo and pre-install CRDs to avoid timeouts on small nodes
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts || true
helm repo update
helm show crds prometheus-community/kube-prometheus-stack | k apply -f -
for crd in $(k get crd -o name | grep monitoring.coreos.com); do
  k wait --for=condition=Established --timeout=180s "$crd" || exit 1
done

# Minimal values (create values-monitoring.yml if you don’t have one)
cat > values-monitoring.yml <<'YAML'
grafana:
  admin:
    existingSecret: grafana-admin
    userKey: admin-user
    passwordKey: admin-password
  service:
    type: NodePort
    nodePort: 30090
  persistence:
    enabled: false
prometheus:
  prometheusSpec:
    retention: 2d
    resources: { requests: { cpu: 100m, memory: 256Mi } }
alertmanager:
  enabled: true
  alertmanagerSpec:
    resources: { requests: { cpu: 50m, memory: 128Mi } }
kube-state-metrics:
  resources: { requests: { cpu: 50m, memory: 100Mi } }
nodeExporter:
  resources: { requests: { cpu: 20m, memory: 64Mi } }
YAML

# Install/upgrade (skip CRDs, longer timeout)
helm upgrade --install kps prometheus-community/kube-prometheus-stack \
  -n monitoring -f values-monitoring.yml \
  --wait --timeout 10m0s --skip-crds

# Verify
k -n monitoring get pods
k -n monitoring get svc | egrep 'grafana|prometheus|alertmanager'
```

**Grafana** is available at: `http://<EC2_PUBLIC_IP>:30090` (admin / ChangeMe!).

### Scrape the app
Ensure your Service has a named `http` port and apply a ServiceMonitor.

`k8s/service.yaml` (port must be named `http`):
```yaml
spec:
  ports:
    - name: http
      port: 30080
      targetPort: 8080
```

`monitoring/servicemonitor-go.yaml`:
```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: go-analyzer
  namespace: monitoring
  labels: { release: kps }
spec:
  selector:
    matchLabels: { app: go-analyzer }
  namespaceSelector:
    matchNames: ["default"]
  endpoints:
    - port: http
      path: /metrics
      interval: 15s
```

Alerts (example) — `monitoring/alerts-go.yaml`:
```yaml
apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  name: go-analyzer-rules
  namespace: monitoring
  labels: { release: kps }
spec:
  groups:
    - name: go-analyzer.rules
      rules:
        - alert: GoAnalyzerDown
          expr: absent(up{job=~".*go-analyzer.*"} == 1)
          for: 2m
          labels: { severity: critical }
          annotations:
            summary: "go-analyzer is down"
            description: "No 'up' series for go-analyzer for 2m."
        - alert: PodRestartsHigh
          expr: increase(kube_pod_container_status_restarts_total{namespace="default",pod=~"go-analyzer.*"}[5m]) > 3
          for: 2m
          labels: { severity: warning }
          annotations:
            summary: "Restarts > 3 in 5m"
            description: "Container is restarting frequently."
```

Apply:
```bash
k -n monitoring apply -f monitoring/servicemonitor-go.yaml
k -n monitoring apply -f monitoring/alerts-go.yaml
```

---

## Networking Notes (AWS)

- **Elastic IP** recommended so public IP doesn’t change on stop/start.
- Security Group: allow inbound TCP **22** (SSH) and **30080** (NodePort) from your IP. Add **ICMP Echo** if you want ping.
- If curl/ssh fail, verify: instance **2/2 checks**, **public IP present**, SG rules, and subnet **route to Internet Gateway**.

---

## Troubleshooting

- **`/docs` returns 404** → old pod image. Check image on Deployment, `kubectl rollout restart deploy/go-analyzer`.
- **`invalid token`** → mint the JWT using the cluster’s `JWT_SECRET`; ensure `HS256` and not expired.
- **`ImagePullBackOff`** → tag mismatch or registry auth on node; verify image exists and tag is correct.
- **Prometheus CRD timeout** → pre-apply CRDs then `--skip-crds` (see Monitoring section).
- **Sonar error: Automatic Analysis enabled** → disable Automatic Analysis in SonarCloud when running CI-based scan.
