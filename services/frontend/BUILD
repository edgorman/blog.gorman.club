# The anchor target, same role go_mod(name="mod") plays for the backend: everything else in this
# tree resolves against it (npm's lockfile, the node_modules install, the `test`/`build` scripts).
package_json(
    name="pkg",
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
node_build_script(
    name="build",
    entry_point="build",
    output_directories=["dist"],
)
