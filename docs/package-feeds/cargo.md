# Cargo

## Endpoints

- `GET /api/v1/cargo/api/v1/crates`
- `GET /api/v1/cargo/api/v1/crates/:crate`
- `GET /api/v1/cargo/api/v1/crates/:crate/:version/download`
- `PUT /api/v1/cargo/api/v1/crates/new`
- `DELETE /api/v1/cargo/api/v1/crates/:crate/:version/yank`

## Auth

Use `Authorization: Bearer <token>` or `X-API-Key`.

## Publish

The upload request is multipart form data with a `crate` file field. The file must be named `name-version.crate`.

```bash
curl -X PUT "http://localhost:8080/api/v1/cargo/api/v1/crates/new" \
  -H "X-API-Key: $API_KEY" \
  -F "crate=@lodestone-cargo-1.0.0.crate"
```

## Search / download

```bash
curl "http://localhost:8080/api/v1/cargo/api/v1/crates?q=lodestone" \
  -H "X-API-Key: $API_KEY"

curl -O "http://localhost:8080/api/v1/cargo/api/v1/crates/lodestone-cargo/1.0.0/download" \
  -H "X-API-Key: $API_KEY"
```

## Caveats

- `yank` currently deletes the version instead of marking it yanked.
- Search only filters by the `q` value against package name.
