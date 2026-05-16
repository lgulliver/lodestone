# RubyGems

## Endpoints

- `GET /api/v1/gems`
- `GET /api/v1/gems/:name.json`
- `GET /api/v1/versions/:name.json`
- `GET /api/v1/gems/:filename`
- `POST /api/v1/gems`
- `DELETE /api/v1/gems/yank?gem_name=:name&version=:version`
- `GET /api/v1/gems/specs.4.8.gz`
- `GET /api/v1/gems/latest_specs.4.8.gz`
- `GET /api/v1/gems/prerelease_specs.4.8.gz`

## Auth

Use `Authorization: Bearer <token>` or `X-API-Key`.

## Publish

```bash
curl -X POST "http://localhost:8080/api/v1/gems" \
  -H "X-API-Key: $API_KEY" \
  -F "gem=@lodestone-0.1.0.gem"
```

## List / download

```bash
curl "http://localhost:8080/api/v1/gems?query=lodestone" \
  -H "X-API-Key: $API_KEY"

curl -O "http://localhost:8080/api/v1/gems/lodestone-0.1.0.gem" \
  -H "X-API-Key: $API_KEY"
```

## Caveats

- This feed is still unstable; published gems may return `500` on upload.
- The `specs*.gz` endpoints currently return empty payloads.
- Yank uses `gem_name` and `version` query parameters.
