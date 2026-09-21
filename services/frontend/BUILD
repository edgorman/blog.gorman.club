# Jest's own config, which babel-jest needs to apply the TypeScript/JSX transform. Not picked up
# by source-code dependency inference (nothing `import`s a config file), so it has to be an
# explicit package_json dependency instead - the sandbox Pants builds for `test`/`package` is the
# transitive closure of declared dependencies plus the npm install, not the whole directory tree.
# Without this, babel-jest silently parses .ts/.tsx as plain JS and every test target fails (CI
# caught this: "SyntaxError: Unexpected token, expected \"from\"" on `import type` syntax).
files(
    name="babel-jest-config",
    sources=[
        "babel.config.cjs",
        "babel-plugin-import-meta-env.cjs",
        "jest.config.cjs",
        "jest.setup-env.cjs",
    ],
)

# The anchor target, same role go_mod(name="mod") plays for the backend: everything else in this
# tree resolves against it (npm's lockfile, the node_modules install, the `test`/`build` scripts).
#
# No `name=` here, unlike go_mod(name="mod") - Pants' own internals (e.g. resolve_for_package)
# address a project's package_json by Address(root_dir), which defaults the target name to the
# directory's basename ("frontend"). Naming it "pkg" made that default address resolve to nothing
# (CI caught this: "The address services/frontend:frontend ... does not exist").
package_json(
    dependencies=[":babel-jest-config"],
)

# vite.config.ts lives outside src/ (it configures the bundler itself, not the app), so it gets its
# own source target here rather than being picked up by src/BUILD.
typescript_sources(
    name="ts",
    sources=["vite.config.ts"],
)

# Wraps `npm run build` (`tsc -b && vite build`) as a packaged artifact. Pants doesn't replace
# Vite; it just needs to own the invocation and know dist/ is the output - see #119's "Wrap `vite
# build` in a `node_build_script`" scope item. `pants package` produces the identical bytes
# `frontend-build`/`frontend-publish` upload today; the publish path itself is untouched (#119 is
# explicitly out of scope for switching it over).
#
# No `name=` here - unlike package_json/typescript_sources above, NodeBuildScript doesn't accept
# one; its address is derived from `entry_point` instead (CI caught this: "NodeBuildScript.create()
# got an unexpected keyword argument 'name'").
node_build_script(
    entry_point="build",
    output_directories=["dist"],
)
