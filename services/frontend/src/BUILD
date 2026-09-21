tsx_sources(
    name="tsx",
    sources=["App.tsx", "main.tsx", "testUtils.tsx"],
)

# setupTests.ts is Jest's setupFilesAfterEach entry point (see ../jest.config.cjs);
# vite-env.d.ts is Vite's own ambient type declaration - both are plain .ts, not .tsx.
typescript_sources(
    name="ts",
    sources=["setupTests.ts", "vite-env.d.ts"],
)

# main.tsx imports this directly; without an owning target, Pants' dependency inference can't
# resolve it and pants package's real `vite build` wouldn't find it in the sandbox.
resources(
    name="css",
    sources=["index.css"],
)
