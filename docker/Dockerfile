# ---------- Build stage ----------
    FROM golang:1.22-alpine AS build
    WORKDIR /src
    
    # Install git (Go may try to read VCS info)
    RUN apk add --no-cache git
    
    # Copy module file first to leverage layer cache
    COPY go-app/go.mod ./
    # If you later add deps, this will download them and cache the layer
    RUN go mod download
    
    # Copy the rest of the sources
    COPY go-app/*.go ./
    
    # Accept platform from Buildx (defaults provided for plain docker build)
    ARG TARGETOS=linux
    ARG TARGETARCH=amd64
    
    # Create output dir, build static binary, disable VCS stamping for reliability
    RUN mkdir -p /out \
     && go version \
     && ls -la \
     && CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH GOFLAGS="-buildvcs=false" \
        go build -v -ldflags="-s -w" -o /out/app .
    
    # ---------- Runtime stage ----------
    FROM scratch
    COPY --from=build /out/app /app
    EXPOSE 8080
    USER 65532:65532
    ENTRYPOINT ["/app"]
    