# Package Feed Guides

Concise, route-accurate usage pages for Lodestone feeds.

## Shared auth

All package routes use the auth middleware. In practice that means one of:

- `Authorization: Bearer <JWT>` or `Authorization: Bearer <API key>`
- `X-API-Key: <key>`
- `X-NuGet-ApiKey: <key>`
- `?api_key=<key>`

If a feed is disabled in registry settings, requests return `503`.

## Guides

- [npm](npm.md)
- [Maven](maven.md)
- [Cargo](cargo.md)
- [Go](go.md)
- [Helm](helm.md)
- [RubyGems](rubygems.md)
- [OPA](opa.md)
- [OCI](oci.md)

## Existing guide

- [NuGet](../NUGET.md)
