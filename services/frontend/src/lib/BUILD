typescript_sources(
    sources=["api.ts", "config.ts", "format.ts", "markdown.ts", "tags.ts", "theme.ts"],
)

# See src/components/BUILD - setupTests.ts needs an explicit dependency on every test target.
typescript_tests(
    name="tests",
    sources=["api.test.ts", "config.test.ts", "markdown.test.ts", "tags.test.ts"],
    dependencies=["//services/frontend/src:ts"],
)
