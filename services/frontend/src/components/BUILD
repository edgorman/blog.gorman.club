tsx_sources(
    sources=[
        "AccountPanel.tsx",
        "AssistantPanel.tsx",
        "Comments.tsx",
        "FeedList.tsx",
        "GoogleSignInButton.tsx",
        "NavBar.tsx",
        "ReactionBar.tsx",
        "TagList.tsx",
    ],
)

# setupTests.ts isn't imported anywhere (it's wired in only via jest.config.cjs's
# setupFilesAfterEnv), so dependency inference can't find it; every test target needs it as an
# explicit dependency or Pants' sandbox won't include it (CI caught this: "Module
# <rootDir>/src/setupTests.ts in the setupFilesAfterEnv option was not found").
tsx_tests(
    name="tests",
    sources=[
        "AssistantPanel.test.tsx",
        "Comments.test.tsx",
        "NavBar.test.tsx",
        "ReactionBar.test.tsx",
    ],
    dependencies=["//services/frontend/src:ts"],
)
