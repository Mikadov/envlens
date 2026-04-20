# Security Policy

## Reporting a vulnerability

If you believe you have found a security vulnerability in envlens, please
**do not** open a public GitHub issue. Instead, report it privately through
GitHub's built-in private vulnerability reporting:

1. Go to the repository's **Security** tab
2. Click **Report a vulnerability**
3. Submit a detailed description

Alternatively, you can reach out via email to the maintainers listed in the
repository metadata.

## What to include

To help us reproduce and prioritise the issue quickly, please include:

- A clear description of the vulnerability and its potential impact
- Steps to reproduce (minimal `.env` fixture or command invocation)
- Affected version(s) — the output of `envlens --version`
- Your environment (OS, Go version)
- Any proof-of-concept code or screenshots

## Expected response time

We aim to acknowledge new reports within **72 hours** and to provide an
initial assessment within **7 days**. Critical issues will be prioritised for
a patch release once a fix is confirmed.

## Scope

envlens is a local CLI tool that reads user-supplied `.env` files. In-scope
vulnerabilities include, but are not limited to:

- Parser bugs that cause a crash or hang on crafted input
- Logic bugs that leak sensitive values (masked keys displayed unmasked)
- Any issue in the release artefacts (tar.gz / zip) that compromises integrity

Out-of-scope:

- Issues in dependencies (report upstream; we will track updates via
  Dependabot)
- Misconfiguration of the user's own environment

Thank you for helping keep envlens and its users safe.
