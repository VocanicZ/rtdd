import hashlib
import pathlib

import pytest

from replay.corpus import (
    Corpus,
    CorpusNotFrozenError,
    UnknownRepoError,
    load_corpus,
)

YAML = """\
frozen_at: "2026-08-26"
criteria:
  - "pure-Python, pytest-based, no compiled extension in the import path"
  - "suite completes under 10 minutes uninstrumented on the disclosed hardware"
repos:
  - id: demo
    url: https://example.invalid/demo.git
    pin: 1111111111111111111111111111111111111111
    replay_commits: 50
    python: "3.12"
    install: ["uv pip install -e ."]
    source_globs: ["src/**/*.py"]
    test_globs: ["tests/**/*.py"]
excluded:
  - url: https://example.invalid/other.git
    reason: "requires a live PostgreSQL; suite cannot run hermetically"
"""


def _write(tmp_path: pathlib.Path, text: str = YAML, lock: str | None = None):
    y = tmp_path / "corpus.yaml"
    y.write_text(text, encoding="utf-8")
    lk = tmp_path / "corpus.lock"
    digest = hashlib.sha256(y.read_bytes()).hexdigest()
    lk.write_text((lock if lock is not None else digest) + "\n", encoding="utf-8")
    return y, lk


def test_loads_when_lock_matches(tmp_path):
    y, lk = _write(tmp_path)
    c = load_corpus(y, lk)
    assert c.ids() == ("demo",)
    assert c.repos["demo"].replay_commits == 50
    assert c.excluded[0]["reason"].startswith("requires a live PostgreSQL")


def test_refuses_when_lock_does_not_match(tmp_path):
    y, lk = _write(tmp_path, lock="0" * 64)
    with pytest.raises(CorpusNotFrozenError):
        load_corpus(y, lk)


def test_refuses_a_repo_not_in_the_frozen_list(tmp_path):
    y, lk = _write(tmp_path)
    c = load_corpus(y, lk)
    with pytest.raises(UnknownRepoError):
        c.require("some-repo-i-liked-the-look-of")


def test_refuses_an_empty_excluded_table(tmp_path):
    y, lk = _write(tmp_path, text=YAML.split("excluded:")[0] + "excluded: []\n")
    with pytest.raises(CorpusNotFrozenError):
        load_corpus(y, lk)


REPO_ROOT = pathlib.Path(__file__).resolve().parents[2]


def test_shipped_corpus_is_frozen_and_loadable():
    c = load_corpus(REPO_ROOT / "bench" / "corpus.yaml", REPO_ROOT / "bench" / "corpus.lock")
    assert c.ids()
    assert c.excluded
    assert all(len(s.pin) == 40 for s in c.repos.values()), "every pin must be a full sha"


def test_shipped_corpus_states_mechanical_criteria_and_a_reason_per_exclusion():
    c = load_corpus(REPO_ROOT / "bench" / "corpus.yaml", REPO_ROOT / "bench" / "corpus.lock")
    assert len(c.criteria) >= 3
    assert c.frozen_at
    assert all(e.get("url") and e.get("reason") for e in c.excluded)
    for spec in c.repos.values():
        assert spec.source_globs and spec.test_globs and spec.install
        assert c.require(spec.id) is spec
