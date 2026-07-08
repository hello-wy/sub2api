# Project Agent Rules

## Push And Workflow Verification

- When pushing changes in this project, use `git push origin <branch>` for the project remote unless the user explicitly asks otherwise.
- After every push, use `gh` to read the GitHub Actions workflow results for the pushed commit.
- If any workflow fails, inspect the failed job and step logs with `gh run view --log-failed`, identify the root cause, fix it, commit, and push again.
- Repeat the check/fix/push loop until the relevant workflows for the pushed commit complete successfully, or until a failure requires user input or external access that is not available.
- Report the final workflow status, commit hash, and any unresolved blocker to the user.
