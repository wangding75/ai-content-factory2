import os
import sys
import json
import yaml

OPENAPI_YAML_PATH = "packages/contracts/openapi/openapi.yaml"
INVENTORY_JSON_PATH = "packages/contracts/openapi/compatibility/openapi_inventory.json"

def load_openapi():
    with open(OPENAPI_YAML_PATH, "r", encoding="utf-8") as f:
        return yaml.safe_load(f)

def resolve_ref(ref, doc):
    if not ref.startswith('#/'):
        return {}
    parts = ref[2:].split('/')
    curr = doc
    for p in parts:
        if isinstance(curr, dict) and p in curr:
            curr = curr[p]
        else:
            return {}
    return curr

def resolve_ref_dict(node, doc, path_history=None):
    if path_history is None:
        path_history = set()
    if not isinstance(node, dict):
        return node
    if '$ref' in node:
        ref = node['$ref']
        if ref in path_history:
            return node
        path_history.add(ref)
        resolved = resolve_ref(ref, doc)
        return resolve_ref_dict(resolved, doc, path_history)
    return node

def is_error_envelope_ref(schema_ref, doc, path_history=None):
    if not schema_ref:
        return False
    if schema_ref.endswith('/ErrorEnvelope'):
        return True
    if path_history is None:
        path_history = set()
    if schema_ref in path_history:
        return False
    path_history.add(schema_ref)
    try:
        resolved = resolve_ref(schema_ref, doc)
        if isinstance(resolved, dict):
            if 'allOf' in resolved:
                for sub in resolved['allOf']:
                    if isinstance(sub, dict) and '$ref' in sub:
                        if is_error_envelope_ref(sub['$ref'], doc, path_history.copy()):
                            return True
            if 'oneOf' in resolved:
                for sub in resolved['oneOf']:
                    if isinstance(sub, dict) and '$ref' in sub:
                        if is_error_envelope_ref(sub['$ref'], doc, path_history.copy()):
                            return True
    except Exception:
        pass
    return False

def check_expected_version_in_schema(schema, doc, path_history=None):
    if path_history is None:
        path_history = set()
    if isinstance(schema, dict) and '$ref' in schema:
        ref = schema['$ref']
        if ref in path_history:
            return None
        path_history.add(ref)
        resolved = resolve_ref(ref, doc)
        return check_expected_version_in_schema(resolved, doc, path_history)

    if not isinstance(schema, dict):
        return None

    if 'allOf' in schema:
        for sub in schema['allOf']:
            res = check_expected_version_in_schema(sub, doc, path_history.copy())
            if res:
                return res

    if 'properties' in schema:
        for prop_name in schema['properties']:
            if prop_name.startswith('expected') and prop_name.endswith('Version'):
                return prop_name
            if prop_name == 'expected_version':
                return prop_name
            if prop_name == 'expected_usage_version':
                return prop_name

    return None

def extract_path_details(path, method, operation, doc):
    operation_id = operation.get('operationId')

    # Merge path-level and operation-level parameters
    parameters = []
    if 'parameters' in doc['paths'][path]:
        parameters.extend(doc['paths'][path]['parameters'])
    if 'parameters' in operation:
        parameters.extend(operation['parameters'])

    resolved_params = []
    for param in parameters:
        resolved = resolve_ref_dict(param, doc)
        if resolved:
            resolved_params.append(resolved)

    # Check Idempotency-Key header
    has_idempotency_key = False
    for param in resolved_params:
        if param.get('in') == 'header' and param.get('name', '').lower() == 'idempotency-key':
            has_idempotency_key = True
            break

    # Check limit & offset query params (pagination)
    has_limit = False
    has_offset = False
    for param in resolved_params:
        if param.get('in') == 'query':
            if param.get('name') == 'limit':
                has_limit = True
            elif param.get('name') == 'offset':
                has_offset = True
    has_pagination = has_limit and has_offset

    # Check expected version in query/path/header params
    expected_version = None
    for param in resolved_params:
        pname = param.get('name', '')
        if pname == 'expected_version' or (pname.startswith('expected') and pname.endswith('Version')):
            expected_version = pname
            break

    # Check expected version in requestBody schema (if not found in parameters)
    if not expected_version and 'requestBody' in operation:
        req_body = resolve_ref_dict(operation['requestBody'], doc)
        if 'content' in req_body and 'application/json' in req_body['content']:
            json_schema = req_body['content']['application/json'].get('schema')
            if json_schema:
                expected_version = check_expected_version_in_schema(json_schema, doc)

    # Extract success response schemas and check ErrorEnvelope for errors
    success_responses = []
    error_responses = []
    has_error_envelope = False

    if 'responses' in operation:
        for status, resp_def in operation['responses'].items():
            resolved_resp = resolve_ref_dict(resp_def, doc)
            schema_name = None
            if 'content' in resolved_resp and 'application/json' in resolved_resp['content']:
                json_schema = resolved_resp['content']['application/json'].get('schema')
                if json_schema:
                    if isinstance(json_schema, dict) and '$ref' in json_schema:
                        schema_name = json_schema['$ref'].rsplit('/', 1)[-1]
                    else:
                        schema_name = "inline"

            try:
                status_int = int(status)
                if 200 <= status_int < 300:
                    success_responses.append({
                        "status": status_int,
                        "schema": schema_name
                    })
                elif status_int >= 400:
                    is_env = False
                    if isinstance(resp_def, dict) and '$ref' in resp_def:
                        # Resolved response schema
                        if 'content' in resolved_resp and 'application/json' in resolved_resp['content']:
                            json_schema = resolved_resp['content']['application/json'].get('schema')
                            if json_schema and isinstance(json_schema, dict) and '$ref' in json_schema:
                                if is_error_envelope_ref(json_schema['$ref'], doc):
                                    is_env = True
                    elif schema_name and schema_name != "inline":
                        if is_error_envelope_ref(resolved_resp['content']['application/json']['schema']['$ref'], doc):
                            is_env = True

                    if is_env:
                        has_error_envelope = True
                    error_responses.append({
                        "status": status_int,
                        "schema": schema_name,
                        "is_error_envelope": is_env
                    })
            except ValueError:
                # E.g. 'default' response
                pass

    return {
        "path": path,
        "method": method,
        "operationId": operation_id,
        "idempotency_key": has_idempotency_key,
        "pagination": has_pagination,
        "expected_version": expected_version,
        "success_responses": success_responses,
        "error_responses": error_responses,
        "has_error_envelope": has_error_envelope
    }

def extract_schema_details(schema_name, doc):
    schema_def = doc['components']['schemas'][schema_name]
    required_fields = set()
    enums = {}

    # Check if the schema itself is an enum
    schema_resolved = resolve_ref_dict(schema_def, doc)
    if isinstance(schema_resolved, dict) and 'enum' in schema_resolved:
        return {
            "required": [],
            "enums": {"_self": schema_resolved['enum']}
        }

    def walk_schema(node, path_history=None):
        if path_history is None:
            path_history = set()
        if not isinstance(node, dict):
            return

        if '$ref' in node:
            ref = node['$ref']
            if ref in path_history:
                return
            path_history.add(ref)
            resolved = resolve_ref(ref, doc)
            walk_schema(resolved, path_history)
            return

        if 'allOf' in node:
            for sub in node['allOf']:
                walk_schema(sub, path_history.copy())

        if 'required' in node:
            for req in node['required']:
                required_fields.add(req)

        if 'properties' in node:
            for prop_name, prop_def in node['properties'].items():
                prop_resolved = resolve_ref_dict(prop_def, doc)
                if isinstance(prop_resolved, dict):
                    if 'enum' in prop_resolved:
                        enums[prop_name] = prop_resolved['enum']
                    elif 'type' in prop_resolved and prop_resolved['type'] == 'array' and 'items' in prop_resolved:
                        items_resolved = resolve_ref_dict(prop_resolved['items'], doc)
                        if isinstance(items_resolved, dict) and 'enum' in items_resolved:
                            enums[prop_name] = items_resolved['enum']

    walk_schema(schema_def)
    return {
        "required": sorted(list(required_fields)),
        "enums": enums
    }

def generate_inventory():
    doc = load_openapi()

    # Collect paths
    paths_list = []
    for path, path_item in doc.get('paths', {}).items():
        for method, operation in path_item.items():
            if method in {'get', 'post', 'put', 'delete', 'options', 'head', 'patch', 'trace'}:
                paths_list.append(extract_path_details(path, method, operation, doc))

    # Collect schemas
    schemas_dict = {}
    for schema_name in doc.get('components', {}).get('schemas', {}):
        schemas_dict[schema_name] = extract_schema_details(schema_name, doc)

    return {
        "paths": sorted(paths_list, key=lambda x: (x['path'], x['method'])),
        "schemas": schemas_dict
    }

def main():
    # Ensure directory exists
    os.makedirs(os.path.dirname(INVENTORY_JSON_PATH), exist_ok=True)

    inventory = generate_inventory()

    # Check if we should generate
    if len(sys.argv) > 1 and sys.argv[1] == '--generate':
        with open(INVENTORY_JSON_PATH, 'w', encoding='utf-8') as f:
            json.dump(inventory, f, indent=2, ensure_ascii=False)
        print(f"[PASS] Successfully generated OpenAPI inventory at {INVENTORY_JSON_PATH}")
        sys.exit(0)

    # Otherwise, perform validation
    if not os.path.exists(INVENTORY_JSON_PATH):
        # Auto generate on first run if file is missing
        with open(INVENTORY_JSON_PATH, 'w', encoding='utf-8') as f:
            json.dump(inventory, f, indent=2, ensure_ascii=False)
        print(f"[PASS] Initial inventory created at {INVENTORY_JSON_PATH}")
        sys.exit(0)

    with open(INVENTORY_JSON_PATH, 'r', encoding='utf-8') as f:
        stored_inventory = json.load(f)

    # Check equality by serialized string comparison to be exact and preserve order
    current_serialized = json.dumps(inventory, indent=2, ensure_ascii=False)
    stored_serialized = json.dumps(stored_inventory, indent=2, ensure_ascii=False)

    if current_serialized != stored_serialized:
        print("[FAIL] OpenAPI compatibility inventory mismatch!")
        print("Please run this script with '--generate' to update the compatibility inventory.")
        sys.exit(1)

    print("[PASS] OpenAPI compatibility inventory matches stored file.")
    sys.exit(0)

if __name__ == '__main__':
    main()
