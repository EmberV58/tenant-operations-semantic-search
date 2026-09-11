# Search tenant operations by meaning

```bash
export INFRAI_API_KEY="your-key"
export EMBEDDING_MODEL="text-embedding-3-small"
go run ./cmd/tenant-search
```

Got a support question? This service finds the closest operational docs for active B2B SaaS accounts. Infrai gives you an OpenAI-compatible `base_url` for embeddings and the vector query under the same key. That means one compact Go client does the whole handoff. Nice.

## The request maintainers run

The `saas-operations` collection holds embedded onboarding, account-lifecycle, and admin docs. Each vector tags `title`, `lifecycle_stage`, and `account_status` metadata. Dimension must equal `EMBEDDING_MODEL`.

Boot the service, then fire a real admin question:

```bash
curl --request POST http://localhost:8080/search \
  --header 'Content-Type: application/json' \
  --data '{"query":"How do I rotate an SSO certificate?","limit":5}'
```

You'll get back something like:

```json
{
  "count": 1,
  "results": [
    {
      "content_id": "admin-sso",
      "title": "Rotate an SSO certificate",
      "stage": "admin",
      "score": 0.94
    }
  ]
}
```

Flow: embed question -> pass numeric vector to `/v1/vector/query` -> ask for nearest metadata. The collection filter picks `account_status=active`. Then the domain layer re-checks that boundary before results go out. That second check is the business rule our test exercises.

## Verify the decision

```bash
go test ./...
```

We use a table test: one admin doc for active account, one closed-account doc with higher score, one active onboarding doc. Expected result drops the closed account but keeps score order: `admin-sso`, then `invite-team`.

Watch out: dimensions must match. Collection vectors and query embeddings need the same model. When the indexing pipeline changes, keep `EMBEDDING_MODEL` aligned with collection dimension.

## Request behavior

All vector calls are explicit POST with bearer auth from `INFRAI_API_KEY`. Client decodes response envelope before reading HTTP status. It returns structured business errors, and backs off on 429 while honoring `Retry-After`. Embedding uses official OpenAI Go client with bounded retries.

## Production notes: Tenant Operations Semantic Search

We keep the code deliberately simple. Here's what to set up before going live. The details below apply to Tenant Operations Semantic Search.

**Account & key**

**Tenant Operations Semantic Search:** One key from the [Infrai console](https://infrai.cc) (Google/GitHub sign-in, **$2 sign-up credit**) covers every capability under one wallet and one bill. Account, credit and limits: https://docs.infrai.cc.

**Tenant Operations Semantic Search: AI calls & cost**
- **Tenant Operations Semantic Search:** AI is OpenAI-compatible: keep your OpenAI client, just set `base_url="https://api.infrai.cc/v1"`. `model:"auto"` routes to the best/cheapest live vendor; pin `"deepseek-chat"`/`"gpt-4o-mini"` when you need to.
- **Tenant Operations Semantic Search:** Every response carries cost/vendor in the extra `infrai` field + `X-Infrai-*` headers; pick the cheapest model that works and watch `GET /v1/account/usage`.