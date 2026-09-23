go_mod(name="mod")

# The Dockerfile's `COPY . .` is unchanged so a standalone `docker build services/backend` (what
# backend-deploy/action.yml still runs) keeps working - see services/backend/AGENTS.md's "Container image under Pants"
# section. That also means Pants can't infer this target's dependencies from the Dockerfile, so
# they're declared by hand: `:src` supplies the file bytes the build context needs (a go_package's
# sources aren't picked up transitively by the Docker context assembler), and cmd/backend below
# supplies the import-graph edge `:src` alone can't - the one `pants dependents` needs to reach this
# target from an internal/ change.
files(
    name="src",
    sources=["**/*.go", "go.mod", "go.sum"],
)

docker_image(
    name="image",
    dependencies=[":src", "services/backend/cmd/backend"],
    # Matches the Dockerfile's COPY paths, which stay relative to services/backend/ for that
    # standalone `docker build` to keep working.
    context_root="./",
    # Mirrors the --build-arg COMMIT_SHA backend-deploy/action.yml already passes.
    extra_build_args=["COMMIT_SHA"],
)
