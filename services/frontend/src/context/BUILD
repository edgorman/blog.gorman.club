typescript_sources(
    sources=["AppContext.ts"],
)

tsx_sources(
    name="tsx",
    sources=["AppProvider.tsx"],
)

# See src/components/BUILD - setupTests.ts needs an explicit dependency on every test target.
tsx_tests(
    name="tests",
    sources=["AppProvider.test.tsx"],
    dependencies=["//services/frontend/src:ts"],
)
