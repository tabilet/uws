# Feature 1: OpenAPI Operation Binding

← [Home](index.md) | [Next: Six Structural Constructs →](02-Six-Structural-Constructs.md)

---

Every executable operation in UWS binds to an existing OpenAPI operation by reference. UWS never duplicates the HTTP method, path, request schema, response schema, server, or security scheme — those live in the OpenAPI document.

## Three Mutually Exclusive Shapes

Every valid UWS operation matches exactly one of three shapes:

| Shape | `sourceDescription` | `openapiOperationId` | `openapiOperationRef` | `x-uws-operation-profile` |
|---|---|---|---|---|
| OpenAPI-bound by operationId | REQUIRED | REQUIRED | MUST NOT be set | OPTIONAL |
| OpenAPI-bound by JSON Pointer | REQUIRED | MUST NOT be set | REQUIRED | OPTIONAL |
| Extension-owned | MUST NOT be set | MUST NOT be set | MUST NOT be set | REQUIRED |

A document that mixes fields from two shapes, or omits the binding fields of every shape, is invalid.

## Source Descriptions

`sourceDescriptions[]` declares every OpenAPI document the workflow uses. Every source entry must have a unique `name` matching `^[A-Za-z0-9_-]+$`, a `url`, and optionally `type: openapi`.

```yaml
sourceDescriptions:
  - name: petstore_api
    url: ./petstore.yaml
    type: openapi
  - name: gmail_api
    url: https://raw.githubusercontent.com/example/gmail-openapi/main/openapi.yaml
    type: openapi
  - name: stripe_api
    url: ./stripe.openapi.json
    type: openapi
```

`sourceDescriptions` is REQUIRED whenever any operation declares `sourceDescription`. A document where every operation is extension-owned may omit it.

## Binding by `operationId`

The most common form: name the source description and the OpenAPI `operationId`. UWS resolves the full operation — method, path, server, security — from the OpenAPI document.

```json
{
  "operationId": "list_pets",
  "sourceDescription": "petstore_api",
  "openapiOperationId": "listPets",
  "request": {
    "query": { "limit": 10 }
  },
  "outputs": {
    "firstPet": "$response.body#/0"
  }
}
```

Note what is absent: no HTTP method (`GET`), no path (`/pets`), no response schema. Those live in `petstore.yaml`. UWS only says which operation to call and how to use its result.

**Two operations against two different APIs in one document:**

```yaml
sourceDescriptions:
  - name: weather_api
    url: ./weather.openapi.yaml
    type: openapi
  - name: gmail_api
    url: ./gmail.openapi.yaml
    type: openapi

operations:
  - operationId: get_weather
    sourceDescription: weather_api
    openapiOperationId: getCurrentWeather
    request:
      query:
        q: Los Angeles
    outputs:
      summary: $response.body.summary

  - operationId: send_report
    sourceDescription: gmail_api
    openapiOperationId: sendMessage
    dependsOn: [get_weather]
    request:
      body:
        userId: me
        text: $steps.fetch.outputs.summary
```

Each operation points at a different source. UWS orchestrates them; neither OpenAPI document needs to know about the other.

## Binding by JSON Pointer

When an OpenAPI document does not assign a stable `operationId`, use a JSON Pointer fragment (`openapiOperationRef`) resolved against the named source:

```json
{
  "operationId": "get_pet_by_id",
  "sourceDescription": "petstore_api",
  "openapiOperationRef": "#/paths/~1pets~1{petId}/get"
}
```

The pointer MUST begin with `#/`. Slashes in path segments are escaped as `~1` per RFC 6901. `openapiOperationRef` and `openapiOperationId` MUST NOT be set together on the same operation.

## Extension-Owned Operations

Operations without an OpenAPI binding are owned by a named runtime profile. `x-uws-operation-profile` names the profile; additional `x-*` fields carry profile-specific configuration.

```yaml
operationId: build_email
x-uws-operation-profile: udon
x-udon-runtime:
  type: fnct
  function: mail_raw
  args:
    from: bot@example.com
    to: user@example.com
    subject: Daily weather report
    body: $steps.get_weather.outputs.summary
```

The validator accepts this as intentionally runtime-owned. See [Extension Profiles](08-Extension-Profiles.md) for more.

## Request Binding

The `request` object maps values into the OpenAPI operation's parameters and body. Keys map to OpenAPI parameter locations:

```yaml
request:
  path:
    petId: $steps.list.outputs.firstId
  query:
    includeDetails: true
    format: json
  header:
    X-Request-Id: trace-abc-123
    Accept-Language: en-US
  cookie:
    session: $variables.session_token
  body:
    status: available
    tags:
      - featured
```

Only `path`, `query`, `header`, `cookie`, `body`, and `^x-` extension keys are permitted at the top level of `request`. Any other key is rejected:

```
# This fails validation:
request:
  params:        # ← invalid key — use "query" or "path"
    limit: 10
```

## Outputs

`outputs` maps friendly names to runtime expressions evaluated after the operation runs:

```yaml
operationId: search_products
sourceDescription: catalog_api
openapiOperationId: searchProducts
request:
  query:
    q: $variables.search_term
outputs:
  total:    $response.body.total
  firstId:  $response.body#/items/0/id
  status:   $response.statusCode
  location: $response.headers.Location
```

These outputs are available to downstream steps as `$steps.<stepId>.outputs.<name>`.

## What the Validator Rejects

The following are all invalid — each produces a structured error:

```yaml
# Shape 1: both binding selectors set at once
- operationId: bad_op
  sourceDescription: api
  openapiOperationId: getOp
  openapiOperationRef: "#/paths/~1foo/get"
  # error: cannot specify both openapiOperationId and openapiOperationRef

# Shape 2: sourceDescription set but no selector
- operationId: bad_op
  sourceDescription: api
  # error: requires exactly one of openapiOperationId or openapiOperationRef

# Shape 3: no binding at all, no profile
- operationId: bad_op
  # error: requires an OpenAPI binding or x-uws-operation-profile

# Shape 4: extension profile but also an OpenAPI binding field
- operationId: bad_op
  sourceDescription: api
  x-uws-operation-profile: udon
  # error: extension-owned operations must not set sourceDescription
```

---

← [Home](index.md) | [Next: Six Structural Constructs →](02-Six-Structural-Constructs.md)
