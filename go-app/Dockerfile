# ---------- Build stage ----------
    FROM golang:1.22-alpine AS build
    WORKDIR /src
    RUN apk add --no-cache git
    
    # Copy module first for better caching
    COPY go.mod ./
    RUN go mod download || true
    
    # Copy sources and build
    COPY . .
    ARG TARGETOS=linux
    ARG TARGETARCH=amd64
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
    