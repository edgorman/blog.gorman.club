typescript_sources(
    sources=["useGoogleAuth.ts", "useReactions.ts"],
)

# See src/components/BUILD - setupTests.ts needs an explicit dependency on every test target.
typescript_tests(
    name="tests",
    sources=["useGoogleAuth.test.ts"],
    dependencies=["//services/frontend/src:ts"],
)

# useReactions.test.tsx renders JSX (via a hook-wrapper component), unlike the plain .ts hook it
# tests, so it's a tsx_test rather than a typescript_test.
tsx_tests(
    name="tests_tsx",
    sources=["useReactions.test.tsx"],
    dependencies=["//services/frontend/src:ts"],
)
