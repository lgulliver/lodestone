# Maven

## Endpoints

- `GET /api/v1/maven/*path`
- `HEAD /api/v1/maven/*path`
- `PUT /api/v1/maven/*path`
- `DELETE /api/v1/maven/*path`

Use the standard Maven path shape:
`group/id/artifact/version/artifact-version.jar`

## Auth

Use `Authorization: Bearer <token>` or `X-API-Key`.

## Publish

```bash
curl -X PUT "http://localhost:8080/api/v1/maven/com/example/app/1.0.0/app-1.0.0.jar" \
  -H "X-API-Key: $API_KEY" \
  --data-binary @app-1.0.0.jar
```

## Download

```bash
curl -O "http://localhost:8080/api/v1/maven/com/example/app/1.0.0/app-1.0.0.jar" \
  -H "X-API-Key: $API_KEY"
```

## Caveats

- Upload is verified; download/HEAD can still miss because the current path-to-package mapping does not round-trip cleanly.
- Paths shorter than four segments are rejected.
