---
description: Create a git commit based on the changes made.
---

Based on the diffs of changed files, run command to create git commit(s) with the message (english) that summarizes the changes made. If the commit conforms to more than one of the commit types, make multiple commits whenever possible.

## Git commit messsage guidelines

- Use conventional commit format: `type(scope): description`.
- Keep subject line under 50 characters.
- Use types: feat, fix, docs, style, refactor, perf, test, chore, ci.
- Include scope when relevant (e.g., ui, auth).
- For additional details, use a well-structured body section.
- Use bullet points (`*`) for clarity.
- Breaking changes:
  - Add `!` after type/scope for breaking changes: `feat!: description`
  - Include `BREAKING CHANGE:` in body with migration instructions
