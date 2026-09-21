tsx_sources(
    sources=[
        "EditPost.tsx",
        "EditProfile.tsx",
        "Landing.tsx",
        "NewPost.tsx",
        "Post.tsx",
        "UserProfile.tsx",
    ],
)

# See src/components/BUILD - setupTests.ts needs an explicit dependency on every test target.
tsx_tests(
    name="tests",
    sources=[
        "EditPost.test.tsx",
        "EditProfile.test.tsx",
        "Landing.test.tsx",
        "NewPost.test.tsx",
        "Post.test.tsx",
        "UserProfile.test.tsx",
    ],
    dependencies=["//services/frontend/src:ts"],
)
