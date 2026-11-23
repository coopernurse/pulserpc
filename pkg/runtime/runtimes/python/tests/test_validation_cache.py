"""Focused tests for the Contract-level validation caches (perf fixes).

These pin the observable behaviour while proving the precomputed enum sets,
resolved struct fields, and resolution indexes are actually reused.
"""

import pulserpc.contract as contract_module
import pulserpc.validation as validation
from pulserpc import Contract, get_struct_fields


def _enum_contract():
    return Contract({
        'structs': [{
            'name': 'Payload',
            'fields': [
                {'name': 'status', 'type': {'userDefined': 'Status'}},
            ],
        }],
        'enums': [{
            'name': 'Status',
            'values': [{'name': 'ok'}, {'name': 'err'}],
        }],
    })


def test_enum_cache_uses_frozenset_and_preserves_error_text(monkeypatch):
    contract = _enum_contract()

    seen = []
    real_validate_enum = validation.validate_enum

    def recording(value, enum_name, allowed_values, path="", _allowed_set=None):
        seen.append(_allowed_set)
        return real_validate_enum(value, enum_name, allowed_values, path, _allowed_set)

    monkeypatch.setattr(validation, 'validate_enum', recording)

    result = contract.validate('Payload', {'status': 'bogus'})
    assert not result.valid
    assert result.error == (
        ".status: Invalid value for enum Status: 'bogus'. "
        "Allowed values: ['ok', 'err']"
    )
    assert result.invalid_fields == ['.status']
    assert len(seen) == 1
    assert isinstance(seen[0], frozenset)
    assert seen[0] == frozenset({'ok', 'err'})

    # Valid values still pass with the cached set.
    assert contract.validate('Payload', {'status': 'ok'}).valid
    assert contract.validate('Payload', {'status': 'err'}).valid


def _override_contract():
    return Contract({
        'structs': [
            {
                'name': 'Base',
                'fields': [
                    {'name': 'id', 'type': {'builtIn': 'string'}},
                    {'name': 'label', 'type': {'builtIn': 'string'}},
                ],
            },
            {
                'name': 'Child',
                'extends': 'ns.Base',  # qualified reference to a plain key
                'fields': [
                    {'name': 'id', 'type': {'builtIn': 'int'}},  # parent override
                    {'name': 'extra', 'type': {'builtIn': 'bool'}},
                ],
            },
        ],
    })


def test_inherited_struct_field_override_is_precomputed_and_honored():
    contract = _override_contract()
    fields = contract._fields['Child']
    assert [f['name'] for f in fields] == ['id', 'label', 'extra']
    assert fields[0]['type'] == {'builtIn': 'int'}

    good = {'id': 7, 'label': 'x', 'extra': True}
    assert contract.validate('Child', good).valid

    bad = dict(good, id='not-an-int')
    result = contract.validate('Child', bad)
    assert not result.valid
    assert result.invalid_fields == ['.id']


def test_get_struct_fields_is_memoized_and_returns_fresh_lists():
    structs = {
        'Base': {'fields': [{'name': 'id', 'type': {'builtIn': 'string'}}]},
        'Child': {
            'extends': 'Base',
            'fields': [
                {'name': 'id', 'type': {'builtIn': 'int'}},
                {'name': 'extra', 'type': {'builtIn': 'bool'}},
            ],
        },
    }
    first = get_struct_fields('Child', structs)
    second = get_struct_fields('Child', structs)
    assert first == second
    assert first is not second
    assert [f['name'] for f in first] == ['id', 'extra']
    assert first[0]['type'] == {'builtIn': 'int'}

    # Mutating a returned list must not poison the memoized value.
    first.append({'name': 'bogus'})
    assert [f['name'] for f in get_struct_fields('Child', structs)] == ['id', 'extra']


def _multi_param_contract():
    return Contract({
        'interfaces': [{
            'name': 'Svc',
            'methods': [{
                'name': 'multi',
                'parameters': [
                    {'name': 'a', 'type': {'userDefined': 'Item'}},
                    {'name': 'b', 'type': {'userDefined': 'Item'}},
                    {'name': 'c', 'type': {'userDefined': 'Status'}},
                ],
                'returnType': {'userDefined': 'Item'},
            }],
        }],
        'structs': [{
            'name': 'Item',
            'fields': [{'name': 'v', 'type': {'builtIn': 'string'}}],
        }],
        'enums': [{'name': 'Status', 'values': [{'name': 'ok'}]}],
    })


def test_same_resolution_index_reused_across_params(monkeypatch):
    contract = _multi_param_contract()
    seen = []
    real = contract_module.validate_type

    def recording(*args, **kwargs):
        seen.append(kwargs.get('_indexes'))
        return real(*args, **kwargs)

    monkeypatch.setattr(contract_module, 'validate_type', recording)

    contract.validate_request('Svc', 'multi', [{'v': 'x'}, {'v': 'y'}, 'ok'])
    assert len(seen) == 3
    contract.validate_response('Svc', 'multi', {'v': 'z'})
    assert len(seen) == 4
    assert all(idx is contract._indexes for idx in seen)
    assert seen[0] is seen[1] is seen[2] is seen[3]


def test_validation_does_not_reacquire_global_index_cache(monkeypatch):
    contract = _multi_param_contract()

    def forbidden(*args, **kwargs):
        raise AssertionError('global index cache touched during validation')

    monkeypatch.setattr(validation, '_get_resolution_index', forbidden)
    contract.validate_request('Svc', 'multi', [{'v': 'x'}, {'v': 'y'}, 'ok'])
    contract.validate_response('Svc', 'multi', {'v': 'z'})
