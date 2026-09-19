go_mod(name="mod")

# (a) from #117's two options, not (b): the Dockerfile's `COPY . .` is what lets it still build
# standalone via a bare `docker build services/backend` - the command backend-deploy/action.yml
# runs today and #117 explicitly leaves alone. Reducing it to a runtime-only stage that COPYs in a
# packaged `go_binary` (option (b)) would make that COPY source only exist inside a Pants-assembled
# build context, breaking the Makefile-driven build this target runs alongside. See the PR
# description for the full writeup, including why the (b) ldflags concern the issue called out
# turned out to be solvable (`extra_build_args` + a `go_binary`'s `linker_flags`) but didn't change
# this conclusion.
#
# `COPY . .` also means Pants can't infer this target's dependencies from the Dockerfile itself
# (dependency inference only resolves literal COPY sources, not "."), so the dependency on the Go
# source tree - and the graph edge `pants dependents` needs to reach this target from any
# internal/ package - is declared by hand via cmd/backend's go_package, which already depends
# (through ordinary Go import inference) on every internal/ package it transitively imports.
docker_image(
    name="image",
    dependencies=["services/backend/cmd/backend"],
    # Mirrors the `--build-arg COMMIT_SHA=...` backend-deploy/action.yml already passes: CI sets
    # COMMIT_SHA before invoking `pants package`, and the Dockerfile's own `ARG COMMIT_SHA=unknown`
    # covers a local `pants package` run where it isn't set.
    extra_build_args=["COMMIT_SHA"],
)
