# Covers every .tsx source (App.tsx/main.tsx/testUtils.tsx here, plus components/, context/,
# pages/ below) with one recursive glob rather than a target per directory (#155) - the directory
# split bought very little `--changed-dependents` precision at this size, since each `tsx_sources`
# glob is itself a target generator (one atomic per-file target underneath), the same granularity
# the eight-BUILD-file layout had.
tsx_sources(
    name="tsx",
    sources=["**/*.tsx", "!**/*.test.tsx"],
)

tsx_tests(
    name="tests_tsx",
    sources=["**/*.test.tsx"],
    dependencies=[":ts"],
)

# setupTests.ts is Jest's setupFilesAfterEach entry point (see ../jest.config.cjs); vite-env.d.ts is
# Vite's own ambient type declaration - neither is ever `import`ed, so dependency inference can't
# reach them and every test target needs this as an explicit dependency. Kept as its own narrow
# target rather than folded into the tree-wide one below: doing that would make every test target's
# blanket dependency on this target reach every unrelated .ts source file in the tree too, which is
# exactly the imprecision `--changed-dependents=transitive` exists to avoid.
typescript_sources(
    name="ts",
    sources=["setupTests.ts", "vite-env.d.ts"],
)

# Every other .ts source (context/, hooks/, lib/, types/) - relies on real import edges for
# dependents, so it's never referenced as a blanket dependency the way :ts above is.
# src/gen/blog/v1 keeps its own BUILD (it also carries the packages/protos dependency), so it's
# excluded here rather than owned twice.
typescript_sources(
    sources=["**/*.ts", "!setupTests.ts", "!vite-env.d.ts", "!**/*.test.ts", "!gen/**"],
)

typescript_tests(
    name="tests_ts",
    sources=["**/*.test.ts"],
    dependencies=[":ts"],
)

# main.tsx imports this directly; without an owning target, Pants' dependency inference can't
# resolve it and pants package's real `vite build` wouldn't find it in the sandbox.
resources(
    name="css",
    sources=["index.css"],
)
