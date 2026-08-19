---
name: cerebrium
description: Use when deploying, scaling, and managing serverless GPU/CPU inference workloads on Cerebrium. Reach for this skill when building REST APIs, streaming endpoints, WebSocket services, or real-time AI applications that need low latency, automatic scaling, and pay-per-second billing. Also use when optimizing cold starts, configuring multi-region deployments, or setting up CI/CD pipelines for ML models.
license: MIT
metadata:
  version: "1.1"
---

# Cerebrium Skill

## Product summary

Cerebrium is a serverless GPU/CPU platform for deploying real-time and high-performance AI workloads. Agents use it to turn Python functions into scalable REST APIs, streaming endpoints, WebSocket services, and async workers with automatic scaling, low cold starts (2-5 seconds), and per-second billing.

**Key files and commands:**
- `cerebrium.toml` — single configuration file for deployment, hardware, scaling, and dependencies
- `cerebrium init [PROJECT_NAME]` — scaffold a new project
- `cerebrium deploy` — deploy the app to production
- `cerebrium run main.py::function_name` — execute code remotely for testing
- Primary docs: https://cerebrium.ai/docs

## Getting started from zero

If Cerebrium is not set up yet, do this first — the rest of this skill assumes an
authenticated CLI.

1. **Create an account** at https://dashboard.cerebrium.ai, where API keys and
   authentication tokens are created.
2. **Install the CLI** (Python 3.8+):
   ```bash
   pip install cerebrium
   ```
3. **Authenticate**:
   ```bash
   cerebrium login
   ```
4. **Scaffold and deploy**:
   ```bash
   cerebrium init my-app && cd my-app && cerebrium deploy
   ```

Compute is billed by the second. See https://www.cerebrium.ai/pricing for current
rates and any starting credit.

## Working examples

`https://github.com/CerebriumAI/examples` holds runnable reference implementations:
vLLM, SDXL, Pipecat voice agents, ASGI apps and more. Prefer adapting an example over
writing a deployment from scratch; each one carries a working `cerebrium.toml`.

Live docs can also be queried directly:
- **MCP server**: `https://cerebrium.ai/docs/mcp`
- **Markdown**: append `.md` to any docs page URL
- **This skill**: `npx skills add https://cerebrium.ai/docs`

## When to use

Reach for this skill when:
- **Deploying inference APIs** — serving LLMs, embeddings, image generation, or other ML models as REST endpoints
- **Building real-time services** — streaming responses, WebSocket connections, or voice/video agents
- **Optimizing for latency** — need low cold starts, multi-region deployment, or automatic scaling
- **Managing GPU workloads** — configuring GPU type, memory, concurrency, and batching
- **Setting up CI/CD** — automating deployments via GitHub Actions or service accounts
- **Tuning performance** — reducing initialization time, configuring scaling metrics, or implementing batching
- **Handling bursty traffic** — scaling from zero to many replicas based on demand

## Quick reference

### Essential CLI commands

| Command | Purpose |
|---------|---------|
| `cerebrium login` | Authenticate CLI session |
| `cerebrium init my-app` | Create new project with main.py and cerebrium.toml |
| `cerebrium deploy` | Deploy app to production |
| `cerebrium run main.py::run --prompt "text"` | Execute function remotely (testing/iteration) |
| `cerebrium projects set PROJECT_ID` | Set active project (`project` is a legacy alias) |
| `cerebrium region set REGION` | Set default region for file commands |
| `cerebrium logs APP_NAME` | View an app's logs, rather than the web dashboard |
| `cerebrium apps list` / `apps get` | List apps, inspect one |
| `cerebrium secrets add` / `secrets list` | Manage project secrets from the terminal |
| `cerebrium scale APP_ID` | Update scaling configuration |
| `cerebrium status` | Check Cerebrium service status |

### Core cerebrium.toml sections

| Section | Purpose | Key fields |
|---------|---------|-----------|
| `[cerebrium.deployment]` | App name, Python version, files to include | `name`, `python_version`, `disable_auth`, `include`, `exclude` |
| `[cerebrium.hardware]` | CPU, memory, GPU type and count | `cpu`, `memory`, `compute`, `gpu_count`, `region` |
| `[cerebrium.scaling]` | Auto-scaling behavior and concurrency | `min_replicas`, `max_replicas`, `replica_concurrency`, `scaling_metric`, `scaling_target` |
| `[cerebrium.dependencies.pip]` | Python packages | `package = "version"` |
| `[cerebrium.dependencies.apt]` | System packages | `ffmpeg = "latest"` |
| `[cerebrium.runtime.custom]` | Custom web server (FastAPI, etc.) | `entrypoint`, `port`, `healthcheck_endpoint` |

### Endpoint URL format

```
https://api.cerebrium.ai/v4/p-{PROJECT_ID}/{APP_NAME}/{FUNCTION_NAME}
```

POST with JSON body; returns `{run_id, run_time_ms, result}`.

### Default environment variables

- `APP_NAME` — app name from cerebrium.toml
- `PROJECT_ID` — Cerebrium project ID
- `BUILD_ID` — unique build identifier
- `HF_HOME` — `/persistent-storage/.cache/huggingface` (for model caching)

## Decision guidance

### When to use X vs Y

| Decision | Use this | When | Use that | When |
|----------|----------|------|----------|------|
| **Default vs custom runtime** | Default (Cortex) | Simple Python functions, REST APIs | Custom runtime | Need ASGI/WSGI server, WebSockets, custom auth, batching |
| **Scaling metric** | `concurrency_utilization` | GPU workloads, variable request size | `requests_per_second` | Steady request rate, CPU workloads |
| **Cold start strategy** | `min_replicas=0` + `scaling_buffer` | Bursty traffic, cost-sensitive | `min_replicas=1+` | Latency-critical, steady traffic |
| **Model storage** | `/persistent-storage` | Single-region apps, region-local cache | `/global-persistent-storage` | Multi-region apps, shared data |
| **Batching** | Framework-native (vLLM) | Framework supports it natively | Custom batching (LitServe) | Need fine-grained control |
| **Authentication** | `disable_auth=true` | Public endpoints, testing | `disable_auth=false` | Production, sensitive data |
| **Deployment** | `cerebrium deploy` | Manual, ad-hoc | GitHub Actions + service account | Automated CI/CD, team workflows |

## Workflow

### Typical deployment workflow

1. **Initialize project**
   - Run `cerebrium init my-app`
   - Creates `main.py` (entrypoint) and `cerebrium.toml` (config)

2. **Define the function**
   - Write a `run()` function in `main.py` that accepts JSON-serializable inputs
   - Return JSON-serializable output
   - Code outside `run()` executes once at startup (load models, initialize)

3. **Configure cerebrium.toml**
   - Set `name`, `python_version`, `disable_auth`
   - Define `[cerebrium.hardware]`: CPU, memory, GPU type
   - Set `[cerebrium.scaling]`: min/max replicas, concurrency, scaling metric
   - List dependencies in `[cerebrium.dependencies.pip]` and `[cerebrium.dependencies.apt]`

4. **Test locally**
   - Run `cerebrium run main.py::run --prompt "test input"` to execute in cloud
   - Check for errors with `cerebrium logs APP_NAME`

5. **Deploy**
   - Run `cerebrium deploy` from project root
   - CLI reads cerebrium.toml, builds container, uploads code, starts app
   - Returns endpoint URL

6. **Call the endpoint**
   - POST to `https://api.cerebrium.ai/v4/p-{PROJECT_ID}/{APP_NAME}/run`
   - Include `Authorization: Bearer {API_KEY}` if `disable_auth=false`
   - Send JSON body matching function signature

7. **Monitor and iterate**
   - Check logs and startup behaviour with `cerebrium logs APP_NAME`
   - Adjust scaling, concurrency, or hardware in cerebrium.toml
   - Re-run `cerebrium deploy` to update

### Cold start optimization workflow

1. **Measure baseline** — read startup timings from `cerebrium logs APP_NAME`
2. **Move initialization to module scope** — load models outside `run()` function
3. **Store weights on persistent storage** — use `/persistent-storage` instead of baking into image
4. **Use Tensorizer or FlashPack** — for direct GPU loading of large models
5. **Enable checkpointing** — capture GPU/CPU state after initialization
6. **Configure scaling** — set `min_replicas`, `scaling_buffer`, or `cooldown` to keep warm containers

## Common gotchas

- **`disable_auth` defaults to `true`** — endpoints are public by default. Set `disable_auth=false` and use API keys for production.
- **Port mismatch in custom runtime** — the port in `entrypoint` must match the `port` parameter in `[cerebrium.runtime.custom]`.
- **Model weights in container image** — baking large models into the image increases cold starts. Use `/persistent-storage` instead.
- **Concurrency defaults differ by compute type** — GPU defaults to 1, CPU to 100. Adjust `replica_concurrency` for your workload.
- **APT/Conda changes trigger full rebuild** — changing system packages rebuilds the entire image. Batch updates together.
- **Python version changes trigger full rebuild** — changing `python_version` or `docker_base_image_url` rebuilds everything.
- **Secrets are not environment variables by default** — add them with `cerebrium secrets add`; they become env vars at runtime.
- **Region-local storage** — `/persistent-storage` is per-region. Use `/global-persistent-storage` for multi-region apps.
- **Private Docker Hub images need login** — run `docker login -u username` (not OAuth flow) before deploying with private base images.
- **Initialization timeout** — `deployment_initialization_timeout` defaults to 600s. Increase if model loading takes longer.
- **Async functions run for max 12 hours** — bounded by `response_grace_period` in scaling config (default 15 minutes).
- **Streaming requires generator/iterator** — use `yield` in the function; client receives SSE stream.
- **WebSockets require custom runtime** — cannot use default Cortex runtime for WebSocket endpoints.

## Verification checklist

Before deploying, verify:

- [ ] `cerebrium.toml` has `[cerebrium.deployment]` with `name` and `[cerebrium.hardware]` with `cpu`, `memory`, `compute`
- [ ] `main.py` has a `run()` function (or custom entrypoint defined in config)
- [ ] All Python dependencies listed in `[cerebrium.dependencies.pip]`
- [ ] System dependencies (ffmpeg, etc.) listed in `[cerebrium.dependencies.apt]`
- [ ] Port in custom runtime `entrypoint` matches `port` parameter
- [ ] Model weights stored on `/persistent-storage` or `/global-persistent-storage`, not in container
- [ ] Secrets added with `cerebrium secrets add`, not hardcoded in code
- [ ] `disable_auth` set correctly (false for production, true for testing)
- [ ] Scaling parameters (`min_replicas`, `max_replicas`, `replica_concurrency`) match traffic pattern
- [ ] No large files in `include` list; use `exclude` to skip unnecessary files
- [ ] Test with `cerebrium run` before deploying
- [ ] Check build logs with `cerebrium logs APP_NAME` for errors (APT install, pip install, shell commands)
- [ ] Verify endpoint is callable with correct URL and auth header

## Resources

- **Full page navigation**: https://cerebrium.ai/docs/llms.txt — comprehensive list of all documentation pages
- **TOML Reference**: https://cerebrium.ai/docs/toml-reference/toml-reference — complete configuration options
- **REST API**: https://cerebrium.ai/docs/endpoints/inference-api — request/response format, authentication
- **Scaling & Concurrency**: https://cerebrium.ai/docs/scaling/batching-concurrency — batching strategies and concurrency tuning
- **Cold Start Optimization**: https://cerebrium.ai/docs/performance/faster-cold-starts — techniques to reduce initialization time
- **CI/CD Pipelines**: https://cerebrium.ai/docs/deployments/ci-cd — GitHub Actions automation with service accounts

---

> For additional documentation and navigation, see: https://cerebrium.ai/docs/llms.txt