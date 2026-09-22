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
package_json(
    dependencies=[":babel-jest-config"],
)

# vite.config.ts lives outside src/ (it configures the bundler, not the app), so it gets its own
# source target here rather than being picked up by src/BUILD.
typescript_sources(
    name="ts",
    sources=["vite.config.ts"],
)

# Wraps `npm run build` (`tsc -b && vite build`) as a packaged artifact, producing the identical
# bytes frontend-build/frontend-publish upload today. No `name=` here - unlike package_json/
# typescript_sources above, NodeBuildScript derives its address from `entry_point` instead.
node_build_script(
    entry_point="build",
    output_directories=["dist"],
)
