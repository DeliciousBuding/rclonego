"""Refresh typed OpenAPI from public Go fields and explicit transport contracts."""
from pathlib import Path
import json
import re

root = Path(__file__).resolve().parents[1]
schemas, embedded = {}, {}


def field_schema(kind):
    if kind.startswith("*"):
        result = field_schema(kind[1:])
        return {"anyOf": [result, {"type": "null"}]}
    if kind.startswith("[]"):
        return {"type": ["array", "null"], "items": field_schema(kind[2:])}
    if kind.startswith("map[string]"):
        return {"type": ["object", "null"], "additionalProperties": field_schema(kind[11:])}
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
    return {"$ref": f"#/components/schemas/{kind.rsplit('.', 1)[-1]}"}


sources = [source for package in ["core", "api", "engine", "persistence"]
           for source in (root / "internal" / package).glob("*.go")]
for source in sorted(sources):
    if source.name.endswith("_test.go"):
        continue
    for name, body in re.findall(r"type (\w+) struct \{(.*?)\n\}", source.read_text(encoding="utf-8"), re.S):
        properties = {}
        for kind, tag in re.findall(r"^\s*\w+\s+(\S+)\s+`json:\"([^\"]+)\"`", body, re.M):
            key = tag.split(",")[0]
            if key != "-":
                properties[key] = field_schema(kind)
        if properties:
            schemas[name] = {"type": "object", "properties": properties}
            embedded[name] = re.findall(r"^\s*(\w+)\s*$", body, re.M)

for name, parents in embedded.items():
    if parents:
        schemas[name] = {"allOf": [*[field_schema(parent) for parent in parents], schemas[name]]}


def object_schema(fields, required=()):
    value = {"type": "object", "properties": {name: field_schema(kind) for name, kind in fields.items()}}
    if required:
        value["required"] = list(required)
    return value

# The public wire types, rather than internal Go anonymous fields.
schemas.update({
    "Error": object_schema({"error": "string", "code": "string", "message": "string", "retryable": "bool",
                           "next_action": "string", "earliest_cursor": "int64"},
                          ["error", "code", "message", "retryable"]),
    "EmptyRequest": {"type": "object", "maxProperties": 0},
    "OK": object_schema({"ok": "bool"}, ["ok"]),
    "Paused": object_schema({"paused": "bool"}, ["paused"]),
    "Jobs": object_schema({"paused": "bool", "jobs": "[]Job"}, ["paused", "jobs"]),
    "Settings": object_schema({"appearance": "string", "defaults": "TransferSettings"}, ["appearance", "defaults"]),
    "JobLog": object_schema({"log": "string"}, ["log"]),
    "JobActionRequest": object_schema({"id": "string", "action": "string", "delta": "int"}, ["id", "action"]),
    "MountActionRequest": object_schema({"id": "string"}, ["id"]),
    "MountStarted": object_schema({"id": "string", "pid": "int"}, ["id", "pid"]),
    "MountStopped": object_schema({"id": "string", "stopped": "bool"}, ["id", "stopped"]),
    "ReconnectRequest": object_schema({"name": "string"}, ["name"]),
    "AnswerRequest": object_schema({"answer": "string"}, ["answer"]),
    "EngineRequest": object_schema({"path": "string", "archive": "string"}),
    "RunRequest": object_schema({"workflow_id": "string", "idempotency_key": "string"}, ["workflow_id"]),
    "BandwidthRequest": object_schema({"id": "string", "bandwidth": "string"}, ["id", "bandwidth"]),
    "BandwidthResult": object_schema({"ok": "bool", "bandwidth": "string"}, ["ok", "bandwidth"]),
    "Health": object_schema({"ok": "bool", "time": "time.Time", "api_version": "int",
                            "mutation_header": "string", "service": "ServiceHealth"},
                           ["ok", "time", "api_version", "mutation_header", "service"]),
    "Overview": object_schema({"version": "string", "activity": "ActivityStatus", "remote_count": "int",
                              "mounts": "[]MountView", "queue_paused": "bool", "jobs": "[]Job",
                              "runtime": "RuntimeSnapshot", "remotes_error": "string"}),
    "APIInfo": object_schema({"name": "string", "version": "string", "api_version": "int", "listen": "string",
                             "mutation_header": "string", "operations": "[]string", "authentication": "string",
                             "endpoints": "map[string]string", "defaults": "TransferSettings"}),
    "AutomationRequest": object_schema({
        "id": "string", "name": "string", "kind": "string", "enabled": "bool",
        "source": "string", "destination": "string", "workflow_id": "string",
        "settings": "TransferSettings", "interval_seconds": "int64", "debounce_seconds": "int",
        "stable_seconds": "int", "rescan_seconds": "int"}, ["name", "kind", "enabled"]),
    "OpenAPIDocument": object_schema({"openapi": "string"}),
})
schemas["APIInfo"]["properties"].update({
    "safety": object_schema({"copy_never_deletes": "bool", "sync_requires_allow_delete": "bool",
                            "sync_max_delete": "int", "oauth_tokens_not_exposed": "bool"}),
    "constraints": object_schema({"queue_workers": "int", "transfers": "string", "checkers": "string",
                                 "chunk_size": "string", "buffer_size": "string", "not_before": "string",
                                 "workflow_source": "string", "workflow_order": "string",
                                 "active_tree": "string", "resource_interval_seconds": "int", "cpu_percent": "string"}),
})
schemas["JobActionRequest"]["properties"]["action"]["enum"] = ["retry", "cancel", "pause", "delete", "move", "delete-workflow"]
schemas["AutomationRequest"]["properties"]["kind"]["enum"] = ["watch", "interval"]
schemas["TokenRequest"]["properties"]["ttl_seconds"].update({"minimum": 60, "maximum": 86400, "default": 3600})
schemas["TokenRequest"]["properties"]["operations"]["items"]["enum"] = ["copy", "copyto", "sync", "check"]
schemas["TokenRequest"]["properties"]["paths"]["description"] = "Absolute local paths or explicit named remote:path roots."
schemas["ReadTokenRequest"] = object_schema({"ttl_seconds": "int"})
schemas["ReadTokenRequest"]["properties"]["ttl_seconds"].update({"minimum": 60, "maximum": 86400, "default": 3600})
schemas["TaskTokenRequest"] = object_schema({"ttl_seconds": "int", "operations": "[]string", "paths": "[]string"},
                                          ["operations", "paths"])
schemas["TaskTokenRequest"]["properties"]["ttl_seconds"] = schemas["ReadTokenRequest"]["properties"]["ttl_seconds"].copy()
schemas["TaskTokenRequest"]["properties"]["operations"].update({"minItems": 1, "maxItems": 4, "items": {
    "type": "string", "enum": ["copy", "copyto", "sync", "check"]}, "type": "array"})
schemas["TaskTokenRequest"]["properties"]["paths"].update({"type": "array", "minItems": 1, "maxItems": 16})
schemas["OpenAPIDocument"]["properties"].update({
    "info": object_schema({"title": "string", "version": "string", "description": "string"}),
    "paths": {"type": "object", "additionalProperties": {"type": "object"}},
    "components": {"type": "object", "properties": {
        "schemas": {"type": "object", "additionalProperties": {"type": "object"}},
        "securitySchemes": {"type": "object", "additionalProperties": {"type": "object"}}}},
    "security": {"type": "array", "items": {"type": "object", "additionalProperties": {"type": "array", "items": {"type": "string"}}}},
})
for name, fields in {"JobCreateRequest": ["name", "source", "destination"],
                     "WorkflowDefinition": ["name", "source", "destination"],
                     "AccountRequest": ["name", "type"]}.items():
    schemas[name]["required"] = fields
schemas["JobCreateRequest"]["properties"]["operation"].update({"enum": ["copy", "copyto", "sync", "check"], "default": "copy"})

request_types = {
    ("POST", "/api/v1/jobs"): "JobCreateRequest",
    ("POST", "/api/v1/jobs/preview"): "JobCreateRequest",
    ("POST", "/api/v1/offline-copies"): "OfflineCopyRequest",
    ("PUT", "/api/v1/jobs/{id}"): "JobUpdateRequest",
    ("POST", "/api/v1/workflows"): "WorkflowRequest",
    ("POST", "/api/v1/workflow-definitions"): "WorkflowDefinition",
    ("PUT", "/api/v1/workflow-definitions"): "WorkflowDefinition",
    ("POST", "/api/v1/accounts"): "AccountRequest",
    ("PUT", "/api/v1/settings"): "SettingsRequest",
    ("PUT", "/api/v1/external"): "[]ExternalTransfer",
    ("POST", "/api/v1/runtime/bandwidth"): "BandwidthRequest",
    ("POST", "/api/v1/remotes/reconnect"): "ReconnectRequest",
    ("POST", "/api/v1/remotes/create"): "AccountRequest",
    ("PUT", "/api/v1/mounts"): "[]MountConfig",
    ("POST", "/api/v1/mounts/start"): "MountActionRequest",
    ("POST", "/api/v1/mounts/stop"): "MountActionRequest",
    ("POST", "/api/v1/jobs/action"): "JobActionRequest",
    ("POST", "/api/v1/queue/pause"): "Paused",
    ("POST", "/api/v1/runs"): "RunRequest",
    ("POST", "/api/v1/tokens/read"): "ReadTokenRequest",
    ("POST", "/api/v1/tokens/tasks"): "TaskTokenRequest",
    ("POST", "/api/v1/engine/{action}"): "EngineRequest",
    ("POST", "/api/v1/handoff/{action}"): "HandoffRequest",
    ("POST", "/api/v1/accounts/{id}/{action}"): "AnswerRequest",
    ("POST", "/api/v1/automations"): "AutomationRequest",
    ("PUT", "/api/v1/automations/{id}"): "AutomationRequest",
}
for kind in request_types.values():
    if kind in schemas:
        schemas[kind]["additionalProperties"] = False
responses = {}


def response(method, endpoint, kind, status=200):
    responses[method, endpoint] = (status, field_schema(kind))


for endpoint, kind in {
    "/api": "APIInfo", "/api/v1": "APIInfo", "/api/v1/schema": "OpenAPIDocument",
    "/api/v1/handoff": "*LegacyHandoff",
    "/api/v1/health": "Health", "/api/v1/doctor": "[]Diagnostic", "/api/v1/diagnostics": "DiagnosticReport",
    "/api/v1/events": "EventsResponse", "/api/v1/engine": "Installation", "/api/v1/providers": "[]Provider",
    "/api/v1/accounts/{id}": "AccountView", "/api/v1/settings": "Settings", "/api/v1/overview": "Overview",
    "/api/v1/remotes": "[]RemoteInfo", "/api/v1/remotes/{name}/quota": "RemoteQuota",
    "/api/v1/remotes/{name}/info": "StorageInfo", "/api/v1/storage/usage": "DirectoryUsage",
    "/api/v1/mounts": "[]MountView", "/api/v1/jobs": "Jobs",
    "/api/v1/workflow-definitions": "[]WorkflowDefinition", "/api/v1/runs": "[]Run",
    "/api/v1/runs/{id}": "RunReport", "/api/v1/runs/{id}/report": "RunReport",
    "/api/v1/jobs/{id}": "Job", "/api/v1/jobs/log": "JobLog", "/api/v1/browse": "BrowseResult",
    "/api/v1/automations": "[]Automation",
    "/api/v1/mounts/{id}/cache": "MountCacheView",
}.items():
    response("GET", endpoint, kind)
responses["GET", "/api/v1/runtime"] = (200, {"anyOf": [
    field_schema("RuntimeSnapshot"), field_schema("ResourceSnapshot"), field_schema("[]ExternalTransferView")]})
for method, endpoint, kind, status in [
    ("POST", "/api/v1/tokens/read", "TokenResponse", 201),
    ("POST", "/api/v1/tokens/tasks", "TokenResponse", 201),
    ("POST", "/api/v1/tokens/{id}/revoke", "OK", 200),
    ("POST", "/api/v1/engine/{action}", "Installation", 200),
    ("POST", "/api/v1/handoff/{action}", "LegacyHandoff", 200),
    ("POST", "/api/v1/accounts", "AccountView", 202),
    ("POST", "/api/v1/accounts/{id}/{action}", "OK", 202),
    ("PUT", "/api/v1/settings", "Settings", 200),
    ("PUT", "/api/v1/external", "[]ExternalTransfer", 200),
    ("POST", "/api/v1/runtime/bandwidth", "BandwidthResult", 200),
    ("POST", "/api/v1/remotes/reconnect", "AccountView", 202),
    ("POST", "/api/v1/remotes/create", "AccountView", 202),
    ("PUT", "/api/v1/mounts", "[]MountConfig", 200),
    ("POST", "/api/v1/mounts/start", "MountStarted", 200),
    ("POST", "/api/v1/mounts/stop", "MountStopped", 200),
    ("POST", "/api/v1/workflow-definitions", "WorkflowDefinition", 201),
    ("PUT", "/api/v1/workflow-definitions", "WorkflowDefinition", 201),
    ("DELETE", "/api/v1/workflow-definitions/{id}", "OK", 200),
    ("POST", "/api/v1/runs", "Run", 202),
    ("POST", "/api/v1/runs/{id}/{action}", "OK", 200),
    ("POST", "/api/v1/jobs", "Job", 202),
    ("POST", "/api/v1/jobs/preview", "Preview", 200),
    ("PUT", "/api/v1/jobs/{id}", "OK", 200),
    ("POST", "/api/v1/workflows", "[]Job", 202),
    ("POST", "/api/v1/jobs/action", "OK", 200),
    ("POST", "/api/v1/queue/pause", "Paused", 200),
    ("POST", "/api/v1/queue/recover", "ServiceHealth", 200),
    ("POST", "/api/v1/offline-copies", "Job", 202),
    ("POST", "/api/v1/logs/cleanup", "OK", 200),
    ("POST", "/api/v1/automations", "Automation", 201),
    ("PUT", "/api/v1/automations/{id}", "Automation", 201),
    ("DELETE", "/api/v1/automations/{id}", "OK", 200),
    ("POST", "/api/v1/automations/{id}/trigger", "OK", 200),
]:
    response(method, endpoint, kind, status)
descriptions = {
    "/api/v1/jobs/preview": "Run without changing files. Preview IDs expire after ten minutes.",
    "/api/v1/jobs": "Submit copy, copyto, sync or check. Sync requires allow_delete and a matching preview_id.",
    "/api/v1/engine/{action}": "Install the pinned official release (optional archive) or select an executable (path).",
    "/api/v1/accounts": "Start an isolated configuration session. Google Drive needs a user-owned client_id.",
    "/api/v1/accounts/{id}/{action}": "Answer the current question with {answer: string}, or cancel the draft.",
    "/api/v1/tokens/read": "Create a session-only read token; ttl_seconds is 60 to 86400.",
    "/api/v1/events": "Cursor is a sequence number. The newest 10000 state events are retained.",
    "/api/v1/events/watch": "SSE stream. Event IDs are cursors; reconnect with after or Last-Event-ID. Expired cursors return 410.",
    "/api/v1/tokens/tasks": "Owner-issued session grant for specific transfer operations and local/remote path roots.",
    "/api/v1/automations": "Opt-in watch (one-way copy) and interval (saved workflow) triggers use the shared task queue.",
}
paths = {}
transport = (root / "internal/api/server.go").read_text(encoding="utf-8")
for method, path in re.findall(r'mux\.HandleFunc\("(\w+) ([^"]+)"', transport):
    stream = path == "/api/v1/events/watch"
    if stream:
        status, response_schema = 200, {"type": "string", "description": "SSE cursor, event and error frames; event data is Event JSON."}
    elif (method, path) in responses:
        status, response_schema = responses[method, path]
    else:
        raise SystemExit(f"Missing response contract: {method} {path}")
    operation = {
        "operationId": method.lower() + "_" + re.sub(r"\W+", "_", path).strip("_"),
        "summary": descriptions.get(path, f"{method} {path}"),
        "parameters": [{"name": name, "in": "path", "required": True, "schema": {"type": "string"}}
                       for name in re.findall(r"\{(\w+)\}", path)],
        "responses": {str(status): {
            "description": "Successful operation",
            "content": {"text/event-stream" if stream else "application/json": {"schema": response_schema}}},
            "default": {"description": "Request rejected",
                        "content": {"application/json": {"schema": field_schema("Error")}}}},
    }
    if method != "GET":
        operation["parameters"].append({"name": "X-RcloneGo-Client", "in": "header", "required": True,
                                         "schema": {"type": "string", "const": "1"}})
        if (method, path) in request_types:
            operation["requestBody"] = {"required": True, "content": {"application/json": {
                "schema": field_schema(request_types[method, path])}}}
    if method == "DELETE":
        operation.pop("requestBody", None)
    if path == "/api/v1/health":
        operation["security"] = []
    if path in ("/api/v1/events", "/api/v1/events/watch"):
        operation["parameters"] += [
            {"name": "after", "in": "query", "schema": {"type": "integer", "minimum": 0}},
            {"name": "limit", "in": "query", "schema": {"type": "integer", "minimum": 1, "maximum": 500}}]
        operation["responses"]["410"] = {"description": "Cursor expired. Refresh state before restarting the event stream.",
                                        "content": {"application/json": {"schema": field_schema("Error")}}}
    if stream:
        operation["parameters"].append({"name": "Last-Event-ID", "in": "header", "schema": {"type": "integer", "minimum": 0}})
    if path == "/api/v1/browse":
        operation["parameters"] += [
            {"name": "path", "in": "query", "required": True, "schema": {"type": "string"}},
            {"name": "offset", "in": "query", "schema": {"type": "integer", "minimum": 0, "maximum": 9999}},
            {"name": "limit", "in": "query", "schema": {"type": "integer", "minimum": 1, "maximum": 500}},
            {"name": "directories_only", "in": "query", "schema": {"type": "boolean", "default": False}},
        ]
    if path == "/api/v1/storage/usage":
        operation["parameters"].append(
            {"name": "path", "in": "query", "required": True, "schema": {"type": "string"}})
    if path == "/api/v1/runs" and method == "GET":
        operation["parameters"] += [
            {"name": "offset", "in": "query", "schema": {"type": "integer", "minimum": 0}},
            {"name": "limit", "in": "query", "schema": {"type": "integer", "minimum": 1, "maximum": 500}},
        ]
    if path == "/api/v1/runtime":
        operation["parameters"].append({"name": "view", "in": "query", "schema": {"type": "string", "enum": ["resources", "transfers"]}})
    if path == "/api/v1/jobs/log":
        operation["parameters"].append({"name": "id", "in": "query", "required": True, "schema": {"type": "string"}})
    if method == "POST" and path in ("/api/v1/jobs", "/api/v1/runs"):
        operation["parameters"].append({"name": "Idempotency-Key", "in": "header", "schema": {"type": "string"}})
    if path == "/api/v1/accounts/{id}/{action}":
        operation["responses"]["200"] = operation["responses"]["202"]
        operation["requestBody"]["required"] = False
    if path == "/api/v1/handoff/{action}":
        operation["responses"]["200"]["content"]["application/json"]["schema"] = {
            "anyOf": [field_schema("HandoffPreview"), field_schema("LegacyHandoff")]}
        operation["requestBody"]["required"] = False
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
            "description": "Loopback API. Owner access, expiring read tokens, and revocable operation/path-scoped task grants."},
            "security": [{"localBearer": []}], "paths": paths,
            "components": {"securitySchemes": {"localBearer": {"type": "http", "scheme": "bearer"}},
                           "schemas": schemas}}
verify_references(document)
(root / "internal/api/openapi.json").write_text(json.dumps(document, indent=2) + "\n", encoding="utf-8")
print(f"Updated contract: {len(paths)} paths, {len(schemas)} schemas")
