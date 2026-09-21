# The explicit dependency on packages/protos is what makes `pants dependents`/`--changed-since`
# resolve this package (and its TypeScript counterpart) from a `.proto`-only edit - see #155. It's
# a plain BUILD-graph edge, not a codegen trigger: pants.backend.experimental.codegen.protobuf.go
# is deliberately disabled (see pants.toml), so nothing tries to generate Go from this target the
# way an enabled Go protobuf codegen backend would.
go_package(
    dependencies=["//packages/protos"],
)
