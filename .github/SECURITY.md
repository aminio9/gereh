# Gereh security policy

## Reporting a vulnerability

Do not disclose security vulnerabilities through a public GitHub issue.

Use GitHub's private vulnerability reporting or create a private
security advisory for this repository.

Include:

- affected component and version;
- reproduction steps;
- expected and observed behavior;
- potential impact;
- logs or proof-of-concept material with secrets removed;
- suggested mitigation, when known.

## Supported versions

Until the first production release, only the latest commit on `main`
is supported.

After production releases begin, this document will list supported
release branches and security maintenance windows.

## Secret handling

Never commit:

- model-provider API keys;
- cloud credentials;
- signing keys;
- database production passwords;
- OAuth client secrets;
- webhook signing secrets;
- tenant credentials.

Development credentials under `deployments/local/.env.example` are
local-only placeholders and must never be reused outside local
development or CI.
