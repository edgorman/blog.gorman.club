---
name: implement-issue
description: Take one GitHub issue in edgorman/blog.gorman.club from issue to open pull request - read it and its parent, check its blockers are merged, implement it against its acceptance criteria, verify, and open the PR. Use when asked to implement, work on, or pick up an issue or sub-issue by number.
argument-hint: <issue number>
---

# Implementing an issue

The issue to implement: $ARGUMENTS. If that's empty, use the one named in the request.

Rules for the code itself are in the `AGENTS.md` of each area you touch, and CI and PR conventions are in the `steward` skill. This is only the order of work.

## 1. Read before touching anything

- Read the issue and its comments. If it has a parent, read that too: it usually holds the order of work (e.g. #190's chain table) and context the sub-issue leaves out.
- **Blockers**: everything the issue or its parent says it depends on must be closed. If one is still open, stop and say which.
- **Already taken**: if an open PR already closes this issue, stop and link it.
- **Stale issues**: an issue is often written before the change that lands just ahead of it (#182 needed rewriting after #187 and the moon migration). Where it contradicts the current code or an `AGENTS.md`, follow the code and say so in the PR body. If the contradiction changes what should be built, stop and ask.

## 2. Implement

- Start the branch (in a cloud session, the one the session was given) from an up-to-date `origin/main`.
- Work through the acceptance criteria one at a time. An issue with none gets its criteria from the problem it describes; write them into the PR body.
- Keep to the issue's scope. List anything else you find in the PR body as a follow-up instead of fixing it here.

## 3. Verify

- Run steward's "Before any push" commands.
- Check every acceptance criterion and record how: a command and its result, or why it can't be checked here (`:validate` and `services/backend:image` can't run in a cloud session).
- Run `/code-review` on the diff and fix what it finds before opening the PR.

## 4. Open the PR

- Title and body per steward's "Branch and merge rules", labelled with the areas it checks. The body's `Closes #<n>` line is what closes the issue on merge.
- Under Testing, list the commands you ran and each acceptance criterion with its result.
- From here the PR is steward's.

## 5. After merge

- If the issue is still open, close it as completed.
- If it was its parent's last open sub-issue, close the parent too, with a comment listing the PRs that delivered it.
