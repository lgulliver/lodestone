# Go

## Endpoints

- `GET /api/v1/go/:module/@latest`
- `GET /api/v1/go/:module/@v/list`
- `GET /api/v1/go/:module/@v/:version[.info|.mod|.zip]`
- `PUT /api/v1/go/:module/@v/:version`
- `DELETE /api/v1/go/:module/@v/:version`

## Auth

Use `Authorization: Bearer <token>` or `X-API-Key`.

## Publish

```bash
curl -X PUT "http://localhost:8080/api/v1/go/lodestone/@v/v1.0.0" \
  -H "X-API-Key: $API_KEY" \
  --data-binary @lodestone-v1.0.0.zip
```

## Download / list

```bash
curl "http://localhost:8080/api/v1/go/lodestone/@v/list" \
  -H "X-API-Key: $API_KEY"

curl -O "http://localhost:8080/api/v1/go/lodestone/@v/v1.0.0.zip" \
  -H "X-API-Key: $API_KEY"
```

## Caveats

- The current route shape only matches single-segment module names, such as `lodestone`.
- `@v/:version.mod` returns a placeholder `go 1.19` module file.
