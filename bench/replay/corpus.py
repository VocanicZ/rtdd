from __future__ import annotations

import dataclasses
import hashlib
import pathlib

import yaml


class CorpusError(RuntimeError):
    pass


class UnknownRepoError(CorpusError):
    pass


class CorpusNotFrozenError(CorpusError):
    pass


@dataclasses.dataclass(frozen=True)
class RepoSpec:
    id: str
    url: str
    pin: str
    replay_commits: int
    python: str
    install: tuple[str, ...]
    source_globs: tuple[str, ...]
    test_globs: tuple[str, ...]


@dataclasses.dataclass(frozen=True)
class Corpus:
    frozen_at: str
    criteria: tuple[str, ...]
    repos: dict[str, RepoSpec]
    excluded: tuple[dict, ...]
    digest: str

    def require(self, repo_id: str) -> RepoSpec:
        try:
            return self.repos[repo_id]
        except KeyError:
            raise UnknownRepoError(
                f"{repo_id!r} is not in the frozen corpus "
                f"({', '.join(sorted(self.repos))}). Add it to bench/corpus.yaml and "
                f"re-freeze before results exist, never after."
            ) from None

    def ids(self) -> tuple[str, ...]:
        return tuple(sorted(self.repos))


def _digest(yaml_path: pathlib.Path) -> str:
    return hashlib.sha256(yaml_path.read_bytes()).hexdigest()


def freeze(yaml_path: pathlib.Path, lock_path: pathlib.Path) -> str:
    d = _digest(yaml_path)
    lock_path.write_text(d + "\n", encoding="utf-8")
    return d


def load_corpus(yaml_path: pathlib.Path, lock_path: pathlib.Path) -> Corpus:
    if not lock_path.exists():
        raise CorpusNotFrozenError(f"{lock_path} is missing; the corpus must be frozen before use")
    actual = _digest(yaml_path)
    expected = lock_path.read_text(encoding="utf-8").strip()
    if actual != expected:
        raise CorpusNotFrozenError(
            f"{yaml_path} has changed since it was frozen "
            f"(lock={expected[:12]}, actual={actual[:12]}). Re-freezing invalidates every "
            f"published result derived from the old corpus."
        )
    raw = yaml.safe_load(yaml_path.read_text(encoding="utf-8")) or {}
    excluded = tuple(raw.get("excluded") or ())
    if not excluded:
        raise CorpusNotFrozenError(
            "corpus.yaml has an empty `excluded:` table. Every attempted-and-rejected repo "
            "must be listed with a reason; exclusions are where cherry-picking hides."
        )
    repos: dict[str, RepoSpec] = {}
    for r in raw.get("repos") or ():
        spec = RepoSpec(
            id=r["id"],
            url=r["url"],
            pin=r["pin"],
            replay_commits=int(r["replay_commits"]),
            python=str(r["python"]),
            install=tuple(r.get("install") or ()),
            source_globs=tuple(r.get("source_globs") or ("**/*.py",)),
            test_globs=tuple(
                r.get("test_globs") or ("tests/**/*.py", "**/test_*.py", "**/*_test.py")
            ),
        )
        if spec.id in repos:
            raise CorpusError(f"duplicate repo id {spec.id!r}")
        repos[spec.id] = spec
    if not repos:
        raise CorpusError("corpus.yaml lists no repos")
    return Corpus(
        frozen_at=str(raw.get("frozen_at", "")),
        criteria=tuple(raw.get("criteria") or ()),
        repos=repos,
        excluded=excluded,
        digest=actual,
    )
