# Companion business skill contract

Keep the generated business skill concise and specific to its CLI.

## Required content

- Trigger description covering the business domain, resources, and login
  recovery, even when the user does not name the CLI.
- Exact natural-language-to-command map with supported filters and identifiers.
- A clear statement of the exact supported scope, including which commands
  mutate data.
- Rules for adding `--json` when structured output is useful.
- Login recovery by exit code and error class.
- Response rules for empty lists, totals, identifiers, and status fields.
- For mutation commands, require an explicit target and summarize the confirmed
  effect without exposing request credentials.

## Login recovery

Run the requested business command first. Do not preflight with cookie existence,
because an expired cookie may still be present.

Handle failures as follows:

```text
exit 0  -> answer from command output
exit 2  -> correct arguments; do not change login state
exit 10 + login required -> run <command> login
exit 10 + credentials rejected -> run <command> login --refresh
exit 11 -> report timeout and preserve the login page
exit 20/21/22/30 -> diagnose awc/profile/extension infrastructure
```

Tell the user before opening an interactive login page. Let the user enter any
password, SSO approval, or CAPTCHA. Set the command tool timeout to at least
330 seconds, wait for login to finish, and retry the original query exactly
once.

Never read, guess, repeat, or expose passwords, CAPTCHA values, cookies, JWTs,
tokens, or `cookieHeader`.
