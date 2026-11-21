"""Cached qualified-name resolution (review finding 2).

The old implementation called ``sorted(definitions)`` up to twice per lookup on
tables that are fixed for a Contract's lifetime.  These tests pin the resolution
semantics to the original algorithm, prove the index is built once, and fail if
lookups regress to repeated sorting.
"""

import builtins

from pulserpc import find_enum, find_struct, get_struct_fields, validate_type
from pulserpc.rpctypes import (
    NameResolutionIndex,
    _get_resolution_index,
    resolve_qualified_name,
)


def slow_resolve(name, definitions):
    """The pre-index algorithm, kept as an oracle for equivalence checks."""
    if name in definitions:
        return name
    for key in sorted(definitions):
        if key.endswith('.' + name):
            return key
    short_name = name.rsplit('.', 1)[-1]
    for key in sorted(definitions):
        if key.rsplit('.', 1)[-1] == short_name:
            return key
    return None


def test_resolution_matches_original_algorithm_for_every_path():
    definitions = {
        'Response': {},
        'inc.Response': {},
        'sub.inc.Doc': {},
        'a.Doc': {},
        'b.Doc': {},
        'Status': {},
    }
    names = [
        'Response',             # exact key wins
        'inc.Response',         # exact key wins over suffix/short fallback
        'sub.inc.Response',     # qualified suffix match
        'Doc',                  # ambiguous short name -> first sorted key
        'inc.Doc',              # qualified name resolved via short name
        'b.Doc',                # exact key wins for one ambiguous candidate
        'Status',               # exact key wins
        'nope.Status',          # short-name fallback
        'sub.inc.Doc',          # exact key wins
        'missing',              # no match at all
    ]
    for name in names:
        assert resolve_qualified_name(name, definitions) == slow_resolve(
            name, definitions
        ), name


def test_exact_key_wins_over_suffix_and_short_name():
    definitions = {'Response': 1, 'inc.Response': 2, 'x.Response': 3}
    assert resolve_qualified_name('Response', definitions) == 'Response'
    assert resolve_qualified_name('x.Response', definitions) == 'x.Response'


def test_ambiguous_names_are_recorded_but_resolve_deterministically():
    definitions = {'b.Doc': 1, 'a.Doc': 2, 'c.Doc': 3}
    index = NameResolutionIndex(definitions)
    assert 'Doc' in index.ambiguous
    for _ in range(10):
        assert resolve_qualified_name('Doc', definitions) == 'a.Doc'
        assert find_struct('Doc', definitions) == 2


def test_find_helpers_and_inheritance_use_the_indexed_path():
    structs = {
        'Response': {'fields': [
            {'name': 'status', 'type': {'builtIn': 'string'}, 'optional': False},
        ]},
        'inc.Repeat': {
            'extends': 'inc.Response',
            'fields': [
                {'name': 'count', 'type': {'builtIn': 'int'}, 'optional': False},
            ],
        },
    }
    enums = {'inc.Status': {'values': [{'name': 'ok'}]}}

    assert find_struct('Repeat', structs) is structs['inc.Repeat']
    assert find_enum('Status', enums) is enums['inc.Status']
    assert [f['name'] for f in get_struct_fields('Repeat', structs)] == [
        'status', 'count'
    ]

    errors = validate_type({}, {'userDefined': 'inc.Repeat'}, structs, enums)
    assert [e.path for e in errors] == ['.status', '.count']

    # The same cached index backs both the direct helpers and validation.
    assert _get_resolution_index(structs) is _get_resolution_index(structs)


def test_index_rebuilds_when_definitions_are_mutated():
    definitions = {'a.One': {}}
    assert resolve_qualified_name('One', definitions) == 'a.One'
    definitions['b.Two'] = {}
    assert resolve_qualified_name('Two', definitions) == 'b.Two'
    assert resolve_qualified_name('One', definitions) == 'a.One'


def test_repeated_lookups_do_not_re_sort_the_definitions(monkeypatch):
    """Regression guard: lookups must not call sorted() on every lookup."""
    definitions = {f'ns{i:03d}.Type{i:03d}': {} for i in range(50)}
    real_sorted = sorted
    sort_calls = []

    def counting_sorted(iterable, *args, **kwargs):
        if iterable is definitions:
            sort_calls.append(1)
        return real_sorted(iterable, *args, **kwargs)

    monkeypatch.setattr(builtins, 'sorted', counting_sorted)

    assert resolve_qualified_name('Type007', definitions) == 'ns007.Type007'
    assert len(sort_calls) == 1  # the index is built exactly once

    before = len(sort_calls)
    for _ in range(1000):
        assert resolve_qualified_name('Type049', definitions) == 'ns049.Type049'
    assert len(sort_calls) == before
