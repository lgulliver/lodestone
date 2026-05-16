# OPA

## Endpoints

- `GET /api/v1/opa/bundles`
- `GET /api/v1/opa/bundles/:name`
- `GET /api/v1/opa/bundles/:name/:version`
- `PUT /api/v1/opa/bundles/:name`
- `PUT /api/v1/opa/bundles/:name/:version`
- `DELETE /api/v1/opa/bundles/:name/:version`

## Auth

Use `Authorization: Bearer <token>` or `X-API-Key`.

## Publish

```bash
curl -X PUT "http://localhost:8080/api/v1/opa/bundles/policy" \
  -H "X-API-Key: $API_KEY" \
  -H "X-Bundle-Version: 1.0.0" \
  --data-binary @policy.tar.gz
```

## List / download

```bash
curl "http://localhost:8080/api/v1/opa/bundles" \
  -H "X-API-Key: $API_KEY"

curl -O "http://localhost:8080/api/v1/opa/bundles/policy/1.0.0" \
  -H "X-API-Key: $API_KEY"
```

## Caveats

- `GET /bundles/:name` returns the latest version by creation time.
- If `X-Bundle-Version` is omitted, the upload defaults to `latest`.
- Downloads are served as gzip tarballs and include an `ETag`.
