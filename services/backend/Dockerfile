# Multi-stage build for Cloud Run. COMMIT_SHA is baked in via -ldflags since images are never rebuilt after staging.
# CMD picks the binary under cmd/: backend (the API) by default, or worker.
FROM golang:1.27-alpine@sha256:70bd7ddea8b6cbe5d6943cde279d6b67feb10cc924fb9100d1d8757c1575b280 AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG COMMIT_SHA=unknown
ARG CMD=backend
RUN CGO_ENABLED=0 go build -ldflags "-X main.commit=${COMMIT_SHA}" -o /out/app ./cmd/${CMD}

# distroless/static: no shell or package manager for a compromised process to use.
FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab
COPY --from=build /out/app /app

EXPOSE 8080
ENTRYPOINT ["/app"]
