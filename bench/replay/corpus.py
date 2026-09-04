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
    #: What was actually measured at the pin, on named hardware. `audit` reads this;
    #: a repo without it cannot be shown to meet the criteria it was admitted under.
    measured: dict = dataclasses.field(default_factory=dict)


@dataclasses.dataclass(frozen=True)
class Corpus:
    frozen_at: str
    criteria: tuple[str, ...]
    repos: dict[str, RepoSpec]
    excluded: tuple[dict, ...]
    digest: str
    version: int = 1
    #: The machine-readable half of the criteria. Empty means no criterion is decidable.
    admission: dict = dataclasses.field(default_factory=dict)

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


@dataclasses.dataclass(frozen=True)
class Finding:
    """One admitted repo failing one criterion it was admitted under."""

    repo_id: str
    criterion: str
    detail: str

    def __str__(self) -> str:
        return f"{self.repo_id}: {self.criterion} — {self.detail}"


def _digest(yaml_path: pathlib.Path) -> str:
    return hashlib.sha256(yaml_path.read_bytes()).hexdigest()


def archive_path(yaml_path: pathlib.Path, version: int) -> pathlib.Path:
    """Where a superseded corpus version is kept, verbatim, for reproduction."""
    return yaml_path.parent / "corpus.d" / f"v{version}.yaml"


def _parse_lock(text: str) -> dict[int, str]:
    """`<version> <digest>` per line. A bare digest is the pre-versioning v1 lock."""
    out: dict[int, str] = {}
    for raw in text.splitlines():
        line = raw.split("#", 1)[0].strip()
        if not line:
            continue
        parts = line.split()
        if len(parts) == 1 and len(parts[0]) == 64:
            out[1] = parts[0]
        elif len(parts) == 2:
            try:
                out[int(parts[0])] = parts[1]
            except ValueError:
                raise CorpusNotFrozenError(f"corpus.lock has an unreadable line: {raw!r}") from None
        else:
            raise CorpusNotFrozenError(f"corpus.lock has an unreadable line: {raw!r}")
    if not out:
        raise CorpusNotFrozenError("corpus.lock is empty; the corpus must be frozen before use")
    return out


def _declared_version(raw: dict) -> int:
    return int(raw.get("corpus_version", 1))


def freeze(yaml_path: pathlib.Path, lock_path: pathlib.Path) -> str:
    """Record this corpus version's digest, keeping every prior version's.

    Re-freezing must never orphan a published result, so the lock is a history and
    not a single line: an old `corpus_digest` stays verifiable against the version
    it was produced under. Archive the superseded file at :func:`archive_path`
    before editing `corpus.yaml`, or that version becomes unreproducible.
    """
    d = _digest(yaml_path)
    raw = yaml.safe_load(yaml_path.read_text(encoding="utf-8")) or {}
    version = _declared_version(raw)
    known = _parse_lock(lock_path.read_text(encoding="utf-8")) if lock_path.exists() else {}
    known[version] = d
    body = "".join(f"{v} {known[v]}\n" for v in sorted(known))
    lock_path.write_text(
        "# corpus.lock — one line per frozen corpus version: <version> <sha256 of that\n"
        "# version's yaml>. History, not a pointer: a result published under an older\n"
        "# digest stays verifiable against the version it was produced under.\n" + body,
        encoding="utf-8",
    )
    return d


def load_corpus(
    yaml_path: pathlib.Path, lock_path: pathlib.Path, version: int | None = None
) -> Corpus:
    """Load the current corpus, or a superseded one by `version`.

    Reproducing a published result means loading the corpus whose digest its
    `config.json` stamps — that is what `version` is for.
    """
    if not lock_path.exists():
        raise CorpusNotFrozenError(f"{lock_path} is missing; the corpus must be frozen before use")
    locks = _parse_lock(lock_path.read_text(encoding="utf-8"))
    path = yaml_path
    if version is not None:
        current = _declared_version(yaml.safe_load(yaml_path.read_text(encoding="utf-8")) or {})
        if version != current:
            path = archive_path(yaml_path, version)
            if not path.exists():
                raise CorpusNotFrozenError(
                    f"corpus version {version} is not archived at {path}; only versions "
                    f"kept verbatim there can reproduce the results published under them"
                )
    actual = _digest(path)
    raw = yaml.safe_load(path.read_text(encoding="utf-8")) or {}
    declared = _declared_version(raw)
    expected = locks.get(declared)
    if expected is None:
        raise CorpusNotFrozenError(
            f"{path} declares corpus_version {declared}, which {lock_path} does not record "
            f"(it knows {sorted(locks)}); re-freeze before use"
        )
    if actual != expected:
        raise CorpusNotFrozenError(
            f"{path} has changed since it was frozen "
            f"(lock={expected[:12]}, actual={actual[:12]}). Re-freezing invalidates every "
            f"published result derived from the old corpus."
        )
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
            measured=dict(r.get("measured") or {}),
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
        version=declared,
        admission=dict(raw.get("admission") or {}),
    )


def audit(corpus: Corpus) -> tuple[Finding, ...]:
    """Check every admitted repo against the measurable admission criteria.

    v1 stated seven criteria and enforced none: they were prose in a YAML header, so
    `sqlfluff` was admitted 2.4x over the same budget that excluded `pandas` and
    nothing noticed. This is what notices. It decides only the criteria that carry a
    threshold in `admission:` and a measurement in the repo's `measured:` block — an
    unmeasured repo is a finding, because unverifiable is not the same as passing.
    """
    a = corpus.admission
    findings: list[Finding] = []
    if not a:
        findings.append(
            Finding(
                "(corpus)",
                "admission",
                "no machine-readable `admission:` thresholds; no criterion is decidable",
            )
        )
        return tuple(findings)
    for repo_id in corpus.ids():
        spec = corpus.repos[repo_id]
        m = spec.measured
        if not m:
            findings.append(
                Finding(
                    repo_id,
                    "measurement",
                    "no `measured:` block; the repo cannot be shown to meet its own bar",
                )
            )
            continue
        floor = a.get("min_collected_tests")
        if floor is not None:
            n = m.get("collected_tests")
            if n is None:
                findings.append(Finding(repo_id, "collected tests", "not measured"))
            elif n < floor:
                findings.append(
                    Finding(repo_id, "collected tests", f"{n} collected, floor is {floor}")
                )
        ceiling = a.get("max_uninstrumented_full_suite_seconds")
        if ceiling is not None:
            secs = m.get("uninstrumented_full_suite_seconds")
            if secs is None:
                findings.append(Finding(repo_id, "uninstrumented full suite", "not measured"))
            elif secs >= ceiling:
                findings.append(
                    Finding(
                        repo_id,
                        "uninstrumented full suite",
                        f"{secs} s, budget is {ceiling} s ({secs / ceiling:.1f}x over)",
                    )
                )
        max_ext = a.get("max_compiled_extensions_on_import_path")
        if max_ext is not None:
            ext = m.get("compiled_extensions_on_import_path")
            if ext is None:
                findings.append(Finding(repo_id, "compiled extensions", "not measured"))
            elif ext > max_ext:
                findings.append(
                    Finding(repo_id, "compiled extensions", f"{ext} on the import path")
                )
        reachable = m.get("replay_commits_ceiling")
        if reachable is not None and spec.replay_commits > reachable:
            findings.append(
                Finding(
                    repo_id,
                    "replay depth",
                    f"declares {spec.replay_commits} commits, measured ceiling is {reachable}",
                )
            )
    return tuple(findings)
