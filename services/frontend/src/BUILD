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
