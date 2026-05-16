# npm

## Endpoints

- `GET /api/v1/npm/:name`
- `GET /api/v1/npm/:name/:version`
- `GET /api/v1/npm/@:scope/:name`
- `GET /api/v1/npm/@:scope/:name/:version`
- `GET /api/v1/npm/:name/-/:filename`
- `GET /api/v1/npm/@:scope/:name/-/:filename`
- `PUT /api/v1/npm/:name`
- `PUT /api/v1/npm/@:scope/:name`
- `GET /api/v1/npm/-/v1/search`

## Auth

Use `Authorization: Bearer <token>` or `X-API-Key` / `X-NuGet-ApiKey`.

## Publish

The publish body must be JSON with `_attachments`; each attachment needs base64 tarball data. The tarball `package.json.name` must match the route name.

```bash
curl -X PUT "http://localhost:8080/api/v1/npm/lodestone" \
  -H "X-API-Key: $API_KEY" \
  -H "Content-Type: application/json" \
  --data @publish.json
```

## Download / search

```bash
curl -L -O "http://localhost:8080/api/v1/npm/lodestone/-/lodestone-1.0.0.tgz" \
  -H "X-API-Key: $API_KEY"

curl "http://localhost:8080/api/v1/npm/-/v1/search?text=lodestone" \
  -H "X-API-Key: $API_KEY"
```

## Caveats

- `size` and `from` query params are accepted but not implemented.
- Scoped packages use the `@scope/name` path shape.
- Publish fails fast if `_attachments` is missing, the tarball is not base64, or the tarball package name does not match the path.
