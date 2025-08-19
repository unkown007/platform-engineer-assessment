# ---------- Build stage ----------
    FROM golang:1.22-alpine AS build
    WORKDIR /src
    COPY go-app/ ./
    RUN apk add --no-cache git
    
    # Accept platform from Buildx; default to amd64 if not provided
    ARG TARGETOS=linux
    ARG TARGETARCH=amd64
    
    # Create output dir, then build static binary
    RUN mkdir -p /out \
     && CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
        go build -ldflags="-s -w" -o /out/app .
    
    # ---------- Runtime stage ----------
    FROM scratch
    COPY --from=build /out/app /app
    EXPOSE 8080
    USER 65532:65532
    ENTRYPOINT ["/app"]
    