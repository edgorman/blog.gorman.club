# Jest's config, needed by babel-jest to apply the TS/JSX transform. Not reachable by dependency
# inference (nothing `import`s a config file), so it's declared explicitly - without it, babel-jest
# parses .ts/.tsx as plain JS and every test target fails.
files(
    name="babel-jest-config",
    sources=[
        "babel.config.cjs",
        "babel-plugin-import-meta-env.cjs",
        "jest.config.cjs",
        "jest.setup-env.cjs",
    ],
)

# No `name=` here: Pants addresses a project's package_json by Address(root_dir), which defaults
# the target name to the directory's basename ("frontend") - naming it anything else breaks that
# default address.
#
# `scripts=` is where node_build_script belongs. It is a field value, not a target - which is why
# it rejects `name=` - so calling it at top level, as this file did from #119 until now, built the
# object and dropped it: the vite build was never in the graph, and `pants package
# services/frontend::` exited in 190ms having matched nothing.
package_json(
    dependencies=[":babel-jest-config"],
    scripts=[
        # Wraps `npm run build` (`tsc -b && vite build`), producing the identical bytes
        # frontend-build/frontend-publish upload today.
        node_build_script(
            entry_point="build",
            output_directories=["dist"],
        ),
    ],
)

# vite.config.ts lives outside src/ (it configures the bundler, not the app), so it gets its own
# source target here rather than being picked up by src/BUILD.
typescript_sources(
    name="ts",
    sources=["vite.config.ts"],
)

# public/theme.js is a classic script tag in index.html, not a module, so nothing imports it and
# `pants tailor --check` wants an owning target for it. Declared from here, reaching down into
# public/, rather than from a public/BUILD: Vite copies that directory into dist/ verbatim, so a
# BUILD file placed inside it ships to the bucket and gets served at the site root.
javascript_sources(
    name="public",
    sources=["public/*.js"],
)
