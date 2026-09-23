# protobuf_sources() only - for `pants fmt`/`pants lint` via buf (pants.backend.codegen.protobuf.lint.buf
# in pants.toml). Pants never generates code from this target: the committed Go bindings in
# services/backend/internal/gen and the committed TypeScript bindings in services/frontend/src/gen
# are both produced by `buf generate` run by hand/CI (see `packages/protos/CLAUDE.md`'s "Contract Layer" section),
# not by a Pants codegen backend.
protobuf_sources(
    sources=["**/*.proto"],
)
