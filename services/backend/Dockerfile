# Multi-stage build for Cloud Run. COMMIT_SHA is baked in via -ldflags since images are never rebuilt after staging.
FROM golang:1.25-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG COMMIT_SHA=unknown
RUN CGO_ENABLED=0 go build -ldflags "-X main.commit=${COMMIT_SHA}" -o /out/backend ./cmd/backend

# distroless/static: no shell or package manager for a compromised process to use.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/backend /backend

EXPOSE 8080
ENTRYPOINT ["/backend"]
