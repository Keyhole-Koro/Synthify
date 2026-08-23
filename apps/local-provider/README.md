# Synthify local provider

This daemon connects a self-hosted Synthify Worker to an authenticated Google
Antigravity CLI (`agy`) over the generated ConnectRPC contract. It is not used by
the hosted production or staging deployments.

## Requirements

- Python 3.10 or newer
- Antigravity CLI 1.1.15 or newer
- An interactive `agy` login completed for the same OS user that runs the daemon
- **AI Credit Overages set to `Never`** in Antigravity before using the daemon
- A bearer token containing at least 32 printable bytes in an owner-only file

The daemon deliberately uses the CLI rather than the `google-antigravity` Python
SDK. The SDK exposes Gemini API-key and Vertex endpoints, while the CLI's cached
OAuth session is the supported path to an individual's Antigravity entitlement.
The daemon cannot inspect the overage preference, so it fails closed on reported
quota errors but cannot prove that the account-level setting is `Never`.

## Run

```bash
python -m pip install ./apps/local-provider
chmod 600 /path/to/provider-token
synthify-local-provider \
  --token-file /path/to/provider-token \
  --default-model gemini-3.7-flash-medium
```

The service binds to `127.0.0.1:7777` by default and rejects non-loopback bind
addresses. Configure the Worker with the same token file and endpoint:

```dotenv
DEPLOYMENT_MODE=self-hosted
LLM_PROVIDER=antigravity
LOCAL_PROVIDER_ENDPOINT=http://127.0.0.1:7777
LOCAL_PROVIDER_TOKEN_FILE=/path/to/provider-token
```

Each generation runs in a new empty temporary directory. Prompts travel to
`agy` through its NDJSON stdin protocol and are never placed in process
arguments. At most two generation subprocesses run concurrently by default;
use `--max-concurrent-generations` to set a bounded value from 1 through 16.
Source files remain disabled until Worker and daemon share an explicit
job-directory mount.

## Tests

The regular suite uses a deterministic fake `agy` executable and does not
consume subscription quota:

```bash
python -m unittest discover -s apps/local-provider/tests -p 'test_*.py' -v
SYNTHIFY_LOCAL_PROVIDER_PYTHON=python go test ./apps/worker/pkg/worker/llm
```

An authenticated live smoke test is intentionally a separate manual release
gate.
