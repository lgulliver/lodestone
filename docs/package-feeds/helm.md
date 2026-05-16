# Helm

## Endpoints

- `GET /api/v1/helm/index.yaml`
- `GET /api/v1/helm/:chart/:version/:filename`
- `POST /api/v1/helm/api/charts`
- `DELETE /api/v1/helm/api/charts/:chart/:version`

## Auth

Use `Authorization: Bearer <token>` or `X-API-Key`.

## Publish

```bash
curl -X POST "http://localhost:8080/api/v1/helm/api/charts" \
  -H "X-API-Key: $API_KEY" \
  -F "chart=@mychart-1.2.3.tgz"
```

## List / download

```bash
curl "http://localhost:8080/api/v1/helm/index.yaml" \
  -H "X-API-Key: $API_KEY"

curl -O "http://localhost:8080/api/v1/helm/mychart/1.2.3/mychart-1.2.3.tgz" \
  -H "X-API-Key: $API_KEY"
```

## Caveats

- `index.yaml` is currently rendered as JSON with an `application/x-yaml` content type.
- `index.yaml` uses relative chart URLs.
- The generated timestamp in the index is currently a fixed placeholder.
