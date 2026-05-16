# OCI

## Endpoints

- `GET /v2/`
- `GET|POST /v2/auth`
- `GET|POST /v2/token`
- `GET|HEAD|PUT|DELETE /v2/:name/manifests/:reference`
- `GET|HEAD|DELETE /v2/:name/blobs/:digest`
- `POST /v2/:name/blobs/uploads/`
- `PATCH|PUT|DELETE|GET /v2/:name/blobs/uploads/:uuid`
- `GET /v2/:name/tags/list`
- `GET /v2/_catalog`

## Auth

Use a Bearer token or API key. The middleware also issues Docker-style `WWW-Authenticate` challenges and enforces `repository:<name>:pull|push` scope on `/v2/*`.

## Publish

```bash
curl -X PUT "http://localhost:8080/v2/lodestone/manifests/latest" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.oci.image.manifest.v1+json" \
  --data-binary @manifest.json
```

## List / download

```bash
curl "http://localhost:8080/v2/lodestone/tags/list" \
  -H "Authorization: Bearer $TOKEN"

curl -I "http://localhost:8080/v2/lodestone/manifests/latest" \
  -H "Authorization: Bearer $TOKEN"
```

## Caveats

- Root OCI routes are mounted at `/v2/*` for Docker compatibility.
- The current runtime may return `503` when OCI is disabled in registry settings.
- Blob upload is session-based; manifests and blobs are handled separately.
