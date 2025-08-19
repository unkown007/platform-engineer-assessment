# ---------- Build stage ----------
    FROM golang:1.22-alpine AS build

    # Build args for cross-compile (works with/without Buildx)
    ARG TARGETOS=linux
    ARG TARGETARCH=amd64
    
    # Bring in the whole repo; safer if folder names shift
    WORKDIR /src
    COPY . ./
    RUN apk add --no-cache git
    
    # Show structure in logs (helps debug)
    RUN ls -la && ls -la go-app || true
    
    # Build from the go-app subdir
    WORKDIR /src/go-app
    
    # Cache modules if present; ok if no external deps
    RUN go mod download || true
    
    # Build static binary
    RUN mkdir -p /out \
     && go version \
     && CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH GOFLAGS="-buildvcs=false" \
        go build -v -ldflags="-s -w" -o /out/app .
    
    # ---------- Runtime stage ----------
    FROM scratch
    COPY --from=build /out/app /app
    EXPOSE 8080
    USER 65532:65532
    ENTRYPOINT ["/app"]
    