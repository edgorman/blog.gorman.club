# Multi-stage build targeting GCP Cloud Run (see CLAUDE.md). COMMIT_SHA is
# baked into the binary at build time via -ldflags: per CLAUDE.md, images are
# tagged with the commit SHA in staging and never rebuilt for production —
# promotion copies the same bytes by digest — so the commit has to be fixed
# at build time rather than read from an env var at deploy time.
FROM golang:1.24-alpine AS build
WORKDIR /src

COPY go.mod ./
RUN go mod download

COPY . .

ARG COMMIT_SHA=unknown
RUN CGO_ENABLED=0 go build -ldflags "-X main.commit=${COMMIT_SHA}" -o /out/backend .

# distroless/static has no shell, package manager, or libc - nothing for a
# compromised process to execute beyond the statically-linked binary itself.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/backend /backend

EXPOSE 8080
ENTRYPOINT ["/backend"]
