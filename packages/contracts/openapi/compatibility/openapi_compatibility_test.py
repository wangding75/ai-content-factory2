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
    properties = {}

    # Check if the schema itself is an enum
    schema_resolved = resolve_ref_dict(schema_def, doc)
    if isinstance(schema_resolved, dict) and 'enum' in schema_resolved:
        return {
            "required": [],
            "enums": {"_self": schema_resolved['enum']},
            "properties": {}
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
                prop_info = {}
                if isinstance(prop_resolved, dict):
                    if 'type' in prop_resolved:
                        prop_info['type'] = str(prop_resolved['type'])
                    if 'format' in prop_resolved:
                        prop_info['format'] = str(prop_resolved['format'])

                    if 'enum' in prop_resolved:
                        enums[prop_name] = prop_resolved['enum']
                    elif 'type' in prop_resolved and prop_resolved['type'] == 'array' and 'items' in prop_resolved:
                        items_resolved = resolve_ref_dict(prop_resolved['items'], doc)
                        if isinstance(items_resolved, dict) and 'enum' in items_resolved:
                            enums[prop_name] = items_resolved['enum']
                properties[prop_name] = prop_info

    walk_schema(schema_def)
    return {
        "required": sorted(list(required_fields)),
        "enums": enums,
        "properties": properties
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

def check_compatibility(baseline, current):
    errors = []

    # 1. Check paths compatibility
    baseline_paths = { (p['path'], p['method']): p for p in baseline['paths'] }
    current_paths = { (p['path'], p['method']): p for p in current['paths'] }

    for (path, method), b_op in baseline_paths.items():
        if (path, method) not in current_paths:
            errors.append(f"Deleted path/method: {method.upper()} {path}")
            continue
        c_op = current_paths[(path, method)]

        # Check operationId
        if b_op['operationId'] != c_op['operationId']:
            errors.append(f"Changed operationId for {method.upper()} {path}: expected '{b_op['operationId']}', got '{c_op['operationId']}'")

        # Check success responses not deleted
        b_success = { r['status']: r for r in b_op['success_responses'] }
        c_success = { r['status']: r for r in c_op['success_responses'] }
        for status, b_resp in b_success.items():
            if status not in c_success:
                errors.append(f"Deleted success response {status} for {method.upper()} {path}")
            else:
                c_resp = c_success[status]
                if b_resp['schema'] != c_resp['schema']:
                    errors.append(f"Changed success response schema for {method.upper()} {path} ({status}): expected '{b_resp['schema']}', got '{c_resp['schema']}'")

    # 2. Check schemas compatibility
    baseline_schemas = baseline['schemas']
    current_schemas = current['schemas']

    for schema_name, b_schema in baseline_schemas.items():
        if schema_name not in current_schemas:
            errors.append(f"Deleted schema: {schema_name}")
            continue
        c_schema = current_schemas[schema_name]

        # Check required fields not expanded (no new required fields added)
        b_req = set(b_schema.get('required', []))
        c_req = set(c_schema.get('required', []))
        new_req = c_req - b_req
        if new_req:
            errors.append(f"Expanded required fields for schema '{schema_name}': added {list(new_req)}")

        # Check fields not deleted
        b_props = b_schema.get('properties', {})
        c_props = c_schema.get('properties', {})
        for prop_name, b_prop_info in b_props.items():
            if prop_name not in c_props:
                errors.append(f"Deleted field '{prop_name}' in schema '{schema_name}'")
                continue
            c_prop_info = c_props[prop_name]

            # Check type/format not changed
            if b_prop_info.get('type') != c_prop_info.get('type'):
                errors.append(f"Changed type for field '{prop_name}' in schema '{schema_name}': expected '{b_prop_info.get('type')}', got '{c_prop_info.get('type')}'")
            if b_prop_info.get('format') != c_prop_info.get('format'):
                errors.append(f"Changed format for field '{prop_name}' in schema '{schema_name}': expected '{b_prop_info.get('format')}', got '{c_prop_info.get('format')}'")

        # Check enums not deleted
        b_enums = b_schema.get('enums', {})
        c_enums = c_schema.get('enums', {})
        for prop_name, b_enum_values in b_enums.items():
            if prop_name not in c_enums:
                errors.append(f"Deleted enum for field '{prop_name}' in schema '{schema_name}'")
                continue
            c_enum_values = c_enums[prop_name]
            deleted_enum_vals = set(b_enum_values) - set(c_enum_values)
            if deleted_enum_vals:
                errors.append(f"Deleted enum values for field '{prop_name}' in schema '{schema_name}': removed {list(deleted_enum_vals)}")

    return errors

def main():
    os.makedirs(os.path.dirname(INVENTORY_JSON_PATH), exist_ok=True)

    current_inventory = generate_inventory()

    # Check if we should generate
    if len(sys.argv) > 1 and sys.argv[1] == '--generate':
        with open(INVENTORY_JSON_PATH, 'w', encoding='utf-8') as f:
            json.dump(current_inventory, f, indent=2, ensure_ascii=False)
        print(f"[PASS] Successfully generated OpenAPI inventory at {INVENTORY_JSON_PATH}")
        sys.exit(0)

    # Otherwise, perform validation
    if not os.path.exists(INVENTORY_JSON_PATH):
        # Auto generate on first run if file is missing
        with open(INVENTORY_JSON_PATH, 'w', encoding='utf-8') as f:
            json.dump(current_inventory, f, indent=2, ensure_ascii=False)
        print(f"[PASS] Initial inventory created at {INVENTORY_JSON_PATH}")
        sys.exit(0)

    with open(INVENTORY_JSON_PATH, 'r', encoding='utf-8') as f:
        stored_inventory = json.load(f)

    errors = check_compatibility(stored_inventory, current_inventory)
    if errors:
        print("[FAIL] OpenAPI compatibility check failed:")
        for err in errors:
            print(f" - {err}")
        sys.exit(1)

    print("[PASS] OpenAPI compatibility inventory matches stored file.")
    sys.exit(0)

if __name__ == '__main__':
    main()
