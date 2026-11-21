"""Tests for namespace-qualified struct inheritance (audit python-01)."""

from pulserpc import get_struct_fields, validate_type, ValidationError


STATUS_FIELD = {'name': 'status', 'type': {'userDefined': 'inc.Status'}, 'optional': False}
COUNT_FIELD = {'name': 'count', 'type': {'builtIn': 'int'}, 'optional': False}


def _structs():
    """Shape of the generated conform IDL: plain keys, qualified extends."""
    return {
        'Response': {'fields': [STATUS_FIELD]},
        'RepeatResponse': {
            'extends': 'inc.Response',
            'fields': [COUNT_FIELD],
        },
    }


def test_qualified_extends_parent_fields_are_resolved():
    fields = get_struct_fields('RepeatResponse', _structs())
    assert [f['name'] for f in fields] == ['status', 'count']


def test_validate_type_checks_inherited_required_field():
    errors = validate_type(
        {'count': 3},
        {'userDefined': 'RepeatResponse'},
        _structs(),
        {'Status': {'values': [{'name': 'ok'}, {'name': 'err'}]}},
    )
    assert len(errors) == 1
    assert isinstance(errors[0], ValidationError)
    assert errors[0].path == '.status'
    assert 'Missing required field' in errors[0].message


def test_validate_type_accepts_valid_inherited_field():
    errors = validate_type(
        {'status': 'ok', 'count': 3},
        {'userDefined': 'RepeatResponse'},
        _structs(),
        {'Status': {'values': [{'name': 'ok'}, {'name': 'err'}]}},
    )
    assert errors == []


def test_qualified_reference_resolves_to_plain_key():
    errors = validate_type(
        {}, {'userDefined': 'inc.Response'}, _structs(), {}
    )
    assert len(errors) == 1
    assert errors[0].path == '.status'


def test_unqualified_reference_resolves_to_qualified_key():
    structs = {
        'inc.Response': {'fields': [STATUS_FIELD]},
    }
    fields = get_struct_fields('Response', structs)
    assert [f['name'] for f in fields] == ['status']


def test_cyclic_extends_does_not_recurse_forever():
    structs = {
        'A': {'extends': 'B', 'fields': [COUNT_FIELD]},
        'B': {'extends': 'A', 'fields': [STATUS_FIELD]},
    }
    names = [f['name'] for f in get_struct_fields('A', structs)]
    assert sorted(names) == ['count', 'status']
