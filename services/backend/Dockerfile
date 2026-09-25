# Multi-stage build for Cloud Run. COMMIT_SHA is baked in via -ldflags since images are never rebuilt after staging.
# CMD picks the binary under cmd/: backend (the API) by default, or worker.
FROM golang:1.26-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG COMMIT_SHA=unknown
ARG CMD=backend
RUN CGO_ENABLED=0 go build -ldflags "-X main.commit=${COMMIT_SHA}" -o /out/app ./cmd/${CMD}

# distroless/static: no shell or package manager for a compromised process to use.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/app /app

EXPOSE 8080
ENTRYPOINT ["/app"]
