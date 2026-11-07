"""Refresh the checked-in OpenAPI contract from transport routes and Go fields."""
from pathlib import Path
import json
import re

root = Path(__file__).resolve().parents[1]
schemas = {}


def field_schema(kind):
    if kind.startswith("*"):
        result = field_schema(kind[1:])
        return {"anyOf": [result, {"type": "null"}]}
    if kind.startswith("[]"):
        return {"type": "array", "items": field_schema(kind[2:])}
    if kind.startswith("map[string]"):
        return {"type": "object", "additionalProperties": field_schema(kind[11:])}
    if kind == "time.Time":
        return {"type": "string", "format": "date-time"}
    if kind == "string":
        return {"type": "string"}
    if kind == "bool":
        return {"type": "boolean"}
    if kind.startswith("int") or kind.startswith("uint"):
        return {"type": "integer"}
    if kind.startswith("float"):
        return {"type": "number"}
    return {"$ref": f"#/components/schemas/{kind}"}


for source in sorted((root / "internal/core").glob("*.go")):
    if source.name.endswith("_test.go"):
        continue
    for name, body in re.findall(r"type (\w+) struct \{(.*?)\n\}", source.read_text(encoding="utf-8"), re.S):
        properties = {}
        for kind, tag in re.findall(r"\w+\s+(\S+)\s+`json:\"([^\"]+)\"`", body):
            key = tag.split(",")[0]
            if key != "-":
                properties[key] = field_schema(kind)
        if properties:
            schemas[name] = {"type": "object", "properties": properties}

# The public wire types, rather than internal Go anonymous fields.
schemas.update({
    "Error": {"type": "object", "required": ["error", "code", "message", "retryable"],
              "properties": {key: {"type": "boolean" if key == "retryable" else "string"}
                             for key in ["error", "code", "message", "retryable", "next_action"]}},
    "Engine": {"type": "object", "properties": {key: {"type": "string"} for key in
               ["version", "path", "source", "sha256", "platform"]}},
    "Event": {"type": "object", "properties": {"seq": {"type": "integer"},
              **{key: {"type": "string"} for key in ["occurred_at", "subject", "kind"]}}},
})
for name, fields in {"JobCreateRequest": ["name", "source", "destination"],
                     "WorkflowDefinition": ["name", "source", "destination"],
                     "AccountRequest": ["name", "type"]}.items():
    schemas[name]["required"] = fields

request_types = {
    ("POST", "/api/v1/jobs"): "JobCreateRequest",
    ("POST", "/api/v1/jobs/preview"): "JobCreateRequest",
    ("PUT", "/api/v1/jobs/{id}"): "JobUpdateRequest",
    ("POST", "/api/v1/workflows"): "WorkflowRequest",
    ("POST", "/api/v1/workflow-definitions"): "WorkflowDefinition",
    ("PUT", "/api/v1/workflow-definitions"): "WorkflowDefinition",
    ("POST", "/api/v1/accounts"): "AccountRequest",
    ("PUT", "/api/v1/settings"): "SettingsRequest",
}
response_types = {
    "/api/v1/engine": "Engine", "/api/v1/jobs/{id}": "Job",
    "/api/v1/jobs/preview": "Preview", "/api/v1/accounts/{id}": "AccountView",
    "/api/v1/runs/{id}/report": "RunReport", "/api/v1/runs/{id}": "RunReport",
}
descriptions = {
    "/api/v1/jobs/preview": "Run without changing files. Preview IDs expire after ten minutes.",
    "/api/v1/jobs": "Submit copy, copyto, sync or check. Sync requires allow_delete and a matching preview_id.",
    "/api/v1/engine/{action}": "Install the pinned official release (optional archive) or select an executable (path).",
    "/api/v1/accounts": "Start an isolated configuration session. Google Drive needs a user-owned client_id.",
    "/api/v1/accounts/{id}/{action}": "Answer the current question with {answer: string}, or cancel the draft.",
    "/api/v1/tokens/read": "Create a session-only read token; ttl_seconds is 60 to 86400.",
    "/api/v1/events": "Cursor is a sequence number. The newest 10000 state events are retained.",
}
paths = {}
transport = (root / "internal/api/server.go").read_text(encoding="utf-8")
for method, path in re.findall(r'mux\.HandleFunc\("(\w+) ([^"]+)"', transport):
    operation = {
        "operationId": method.lower() + "_" + re.sub(r"\W+", "_", path).strip("_"),
        "summary": descriptions.get(path, f"{method} {path}"),
        "parameters": [{"name": name, "in": "path", "required": True, "schema": {"type": "string"}}
                       for name in re.findall(r"\{(\w+)\}", path)],
        "responses": {"200" if method == "GET" else "202": {
            "description": "Successful operation",
            "content": {"application/json": {"schema": field_schema(response_types[path])
                        if path in response_types else {}}}},
            "default": {"description": "Request rejected",
                        "content": {"application/json": {"schema": field_schema("Error")}}}},
    }
    if method != "GET":
        operation["parameters"].append({"name": "X-RcloneGo-Client", "in": "header", "required": True,
                                         "schema": {"type": "string", "const": "1"}})
        operation["requestBody"] = {"required": True, "content": {"application/json": {
            "schema": field_schema(request_types[method, path]) if (method, path) in request_types else {}}}}
    if method == "DELETE":
        operation.pop("requestBody", None)
    if path == "/api/v1/health":
        operation["security"] = []
    if path == "/api/v1/events":
        operation["parameters"] += [
            {"name": "after", "in": "query", "schema": {"type": "integer", "minimum": 0}},
            {"name": "limit", "in": "query", "schema": {"type": "integer", "minimum": 1, "maximum": 500}}]
    paths.setdefault(path, {})[method.lower()] = operation

# Remove dangling schema references for internal anonymous/generic fields.
def verify_references(value):
    if isinstance(value, dict):
        reference = value.get("$ref", "")
        if reference and reference.rsplit("/", 1)[-1] not in schemas:
            raise SystemExit(f"unknown schema: {reference}")
        for child in value.values():
            verify_references(child)
    elif isinstance(value, list):
        for child in value:
            verify_references(child)


document = {"openapi": "3.1.0", "info": {"title": "RcloneGo Local API", "version": "1",
            "description": "Loopback API. Owner tokens grant trusted local-user access. Delegated tokens are read-only."},
            "security": [{"localBearer": []}], "paths": paths,
            "components": {"securitySchemes": {"localBearer": {"type": "http", "scheme": "bearer"}},
                           "schemas": schemas}}
verify_references(document)
(root / "internal/api/openapi.json").write_text(json.dumps(document, indent=2) + "\n", encoding="utf-8")
print(f"Updated contract: {len(paths)} paths, {len(schemas)} schemas")
