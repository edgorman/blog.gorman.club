# protobuf_sources() only - for `pants fmt`/`pants lint` via buf (pants.backend.codegen.protobuf.lint.buf
# in pants.toml). Pants never generates code from this target: the committed Go bindings in
# services/backend/internal/gen are produced by `buf generate` run by hand/CI (see CLAUDE.md's
# "Contract Layer" section), not by a Pants codegen backend.
protobuf_sources(
    sources=["**/*.proto"],
)
