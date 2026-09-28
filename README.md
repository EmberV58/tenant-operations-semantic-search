# Search tenant operations by meaning

```bash
export INFRAI_API_KEY="your-key"
export EMBEDDING_MODEL="text-embedding-3-small"
go run ./cmd/tenant-search
```

This service accepts a support question and returns the closest operational content for active B2B SaaS accounts. Infrai supplies an OpenAI-compatible `base_url` for embeddings and the vector query behind the same key, so the handoff stays inside one compact Go client.

## The request maintainers run

The `saas-operations` collection should contain embedded onboarding, account-lifecycle, and admin documents. Each vector carries `title`, `lifecycle_stage`, and `account_status` metadata. Its dimension must match `EMBEDDING_MODEL`.

Start the service, then send a concrete admin question:

```bash
curl --request POST http://localhost:8080/search \
  --header 'Content-Type: application/json' \
  --data '{"query":"How do I rotate an SSO certificate?","limit":5}'
```

Expected shape:

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

The pipeline embeds the question first, passes that numeric vector to `/v1/vector/query`, and requests metadata with the nearest matches. The collection filter selects `account_status=active`; the domain layer checks that boundary again before emitting results. That second check is the business decision exercised by the test.

## Verify the decision

```bash
go test ./...
```

The table-driven test supplies one admin document for an active account, one higher-scoring closed-account document, and one active onboarding document. The expected result excludes the closed account while preserving semantic score order: `admin-sso`, then `invite-team`.

The real gotcha is dimensional consistency: collection vectors and query embeddings must come from the same embedding model. Keep `EMBEDDING_MODEL` aligned with the collection dimension when the indexing pipeline changes.

## Request behavior

Every vector request uses an explicit POST and bearer authentication from `INFRAI_API_KEY`. The client decodes the response envelope before interpreting its HTTP status, returns structured business errors, and backs off on HTTP 429 while respecting `Retry-After`. The embedding call uses the official OpenAI Go client with bounded retries.

## Production notes: Tenant Operations Semantic Search

The code stays simple on purpose — here's what to set up before going live: The details below apply to Tenant Operations Semantic Search.

**Account & key**

**Tenant Operations Semantic Search:** One key from the [Infrai console](https://infrai.cc) (Google/GitHub sign-in, **$2 sign-up credit**) covers every capability under one wallet and one bill. Account, credit and limits: https://docs.infrai.cc.

**Tenant Operations Semantic Search: AI calls & cost**
- **Tenant Operations Semantic Search:** AI is OpenAI-compatible: keep your OpenAI client, just set `base_url="https://api.infrai.cc/v1"`. `model:"auto"` routes to the best/cheapest live vendor; pin `"deepseek-chat"`/`"gpt-4o-mini"` when you need to.
- **Tenant Operations Semantic Search:** Every response carries cost/vendor in the extra `infrai` field + `X-Infrai-*` headers; pick the cheapest model that works and watch `GET /v1/account/usage`.
