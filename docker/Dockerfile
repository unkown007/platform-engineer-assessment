# ---------- Build stage ----------
FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go-app/ ./
RUN apk add --no-cache git
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /out/app .

# ---------- Runtime stage ----------
FROM scratch
COPY --from=build /out/app /app
EXPOSE 8080
USER 65532:65532
ENTRYPOINT ["/app"]
