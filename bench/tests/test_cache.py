import json
import pathlib

from replay.cache import DEFAULT_ROOT, Cache


def test_key_includes_config_digest_so_a_config_change_misses(cache_root):
    a = Cache(cache_root, "cfg-a")
    b = Cache(cache_root, "cfg-b")
    assert a.key("repo", "sha", "natural", "rtdd") != b.key("repo", "sha", "natural", "rtdd")
    assert a.key("repo", "sha", "natural", "rtdd") == a.key("repo", "sha", "natural", "rtdd")


def test_key_distinguishes_every_coordinate_of_the_tuple(cache_root):
    c = Cache(cache_root, "cfg")
    base = c.key("repo", "sha", "natural", "rtdd")
    assert base != c.key("other", "sha", "natural", "rtdd")
    assert base != c.key("repo", "sha2", "natural", "rtdd")
    assert base != c.key("repo", "sha", "seeded", "rtdd")
    assert base != c.key("repo", "sha", "natural", "full")


def test_key_is_not_confusable_by_concatenating_adjacent_parts(cache_root):
    c = Cache(cache_root, "cfg")
    assert c.key("ab", "c") != c.key("a", "bc")


def test_a_config_change_is_a_miss_not_a_silent_mix_of_old_and_new_numbers(cache_root):
    old = Cache(cache_root, "cfg-a")
    old.put_json(old.key("repo", "sha", "natural", "full"), {"n": 1})

    new = Cache(cache_root, "cfg-b")
    assert new.get_json(new.key("repo", "sha", "natural", "full")) is None


def test_json_round_trip_and_build_is_called_once(cache_root):
    c = Cache(cache_root, "cfg")
    calls = []

    def build():
        calls.append(1)
        return {"tests": ["a::b"], "n": 2}

    k = c.key("repo", "sha", "natural", "full")
    assert c.json_or_build(k, build) == {"tests": ["a::b"], "n": 2}
    assert c.json_or_build(k, build) == {"tests": ["a::b"], "n": 2}
    assert calls == [1]
    assert c.stats() == {"hits": 1, "misses": 1}


def test_json_survives_a_fresh_cache_object_on_the_same_root(cache_root):
    k = Cache(cache_root, "cfg").key("repo", "sha", "natural", "full")
    Cache(cache_root, "cfg").put_json(k, {"n": 3})
    assert Cache(cache_root, "cfg").get_json(k) == {"n": 3}


def test_artifact_round_trip(cache_root, tmp_path):
    c = Cache(cache_root, "cfg")
    src = tmp_path / "state"
    (src / "sub").mkdir(parents=True)
    (src / "map.jsonl").write_text('{"t":"x"}\n', encoding="utf-8")
    (src / "sub" / "meta.json").write_text("{}", encoding="utf-8")

    k = c.key("repo", "sha", "seed", "rtdd")
    assert c.artifact(k) is None
    c.store_artifact(k, src)
    assert c.artifact(k) is not None

    dest = tmp_path / "restored"
    assert c.restore_artifact(k, dest) is True
    assert (dest / "map.jsonl").read_text() == '{"t":"x"}\n'
    assert (dest / "sub" / "meta.json").read_text() == "{}"


def test_a_restored_artifact_is_byte_identical_to_what_was_stored(cache_root, tmp_path):
    c = Cache(cache_root, "cfg")
    src = tmp_path / "state"
    (src / "deep" / "deeper").mkdir(parents=True)
    (src / "empty").mkdir()
    (src / ".coverage").write_bytes(bytes(range(256)))
    (src / "deep" / "deeper" / "map.jsonl").write_text("a\nb\n", encoding="utf-8")
    (src / "deep" / "unicode.txt").write_text("café ☕\n", encoding="utf-8")

    k = c.key("repo", "sha", "natural", "rtdd")
    c.store_artifact(k, src)
    dest = tmp_path / "restored"
    assert c.restore_artifact(k, dest) is True

    assert _tree(dest) == _tree(src)


def test_restoring_a_missing_artifact_is_a_miss_not_a_crash(cache_root, tmp_path):
    c = Cache(cache_root, "cfg")
    k = c.key("repo", "sha", "natural", "rtdd")
    assert c.restore_artifact(k, tmp_path / "dest") is False
    assert c.stats() == {"hits": 0, "misses": 1}


def test_artifact_hit_and_miss_are_counted(cache_root, tmp_path):
    c = Cache(cache_root, "cfg")
    src = tmp_path / "state"
    src.mkdir()
    (src / "f").write_text("x", encoding="utf-8")
    k = c.key("repo", "sha", "natural", "rtdd")

    assert c.restore_artifact(k, tmp_path / "a") is False
    c.store_artifact(k, src)
    assert c.restore_artifact(k, tmp_path / "b") is True
    assert c.stats() == {"hits": 1, "misses": 1}


def test_storing_an_artifact_twice_replaces_the_old_bytes(cache_root, tmp_path):
    c = Cache(cache_root, "cfg")
    k = c.key("repo", "sha", "natural", "rtdd")

    first = tmp_path / "first"
    first.mkdir()
    (first / "only-in-first").write_text("1", encoding="utf-8")
    c.store_artifact(k, first)

    second = tmp_path / "second"
    second.mkdir()
    (second / "only-in-second").write_text("2", encoding="utf-8")
    c.store_artifact(k, second)

    dest = tmp_path / "dest"
    assert c.restore_artifact(k, dest) is True
    assert _tree(dest) == _tree(second)


def test_a_corrupt_json_entry_is_a_miss_not_a_crash(cache_root):
    c = Cache(cache_root, "cfg")
    k = c.key("repo", "sha", "natural", "full")
    c.put_json(k, {"n": 1})

    path = c._json_path(k)
    path.write_text('{"n": 1', encoding="utf-8")

    assert c.get_json(k) is None
    assert c.json_or_build(k, lambda: {"n": 2}) == {"n": 2}
    assert c.get_json(k) == {"n": 2}


def test_a_truncated_or_wrong_shaped_json_entry_is_a_miss(cache_root):
    c = Cache(cache_root, "cfg")
    k = c.key("repo", "sha", "natural", "full")
    c.put_json(k, {"n": 1})

    c._json_path(k).write_text("", encoding="utf-8")
    assert c.get_json(k) is None

    c._json_path(k).write_text("[1, 2]", encoding="utf-8")
    assert c.get_json(k) is None


def test_a_partially_written_artifact_is_a_miss_not_a_crash(cache_root, tmp_path):
    c = Cache(cache_root, "cfg")
    src = tmp_path / "state"
    src.mkdir()
    (src / "f").write_text("x", encoding="utf-8")
    k = c.key("repo", "sha", "natural", "rtdd")
    c.store_artifact(k, src)

    # Simulate a crash between writing the payload and marking it complete.
    c._complete_marker(k).unlink()

    assert c.artifact(k) is None
    assert c.restore_artifact(k, tmp_path / "dest") is False


def test_a_partially_written_artifact_is_replaced_by_the_next_store(cache_root, tmp_path):
    c = Cache(cache_root, "cfg")
    k = c.key("repo", "sha", "natural", "rtdd")

    half = tmp_path / "half"
    half.mkdir()
    (half / "stale").write_text("stale", encoding="utf-8")
    c.store_artifact(k, half)
    c._complete_marker(k).unlink()

    good = tmp_path / "good"
    good.mkdir()
    (good / "fresh").write_text("fresh", encoding="utf-8")
    c.store_artifact(k, good)

    dest = tmp_path / "dest"
    assert c.restore_artifact(k, dest) is True
    assert _tree(dest) == _tree(good)


def test_a_leftover_staging_directory_is_never_mistaken_for_an_entry(cache_root, tmp_path):
    c = Cache(cache_root, "cfg")
    src = tmp_path / "state"
    src.mkdir()
    (src / "f").write_text("x", encoding="utf-8")
    k = c.key("repo", "sha", "natural", "rtdd")
    c.store_artifact(k, src)

    strays = [p for p in c._artifact_path(k).parent.iterdir() if p.name.startswith(".staging")]
    assert strays == []


def test_the_cache_root_defaults_to_bench_cache_and_is_gitignored():
    bench = pathlib.Path(__file__).resolve().parent.parent
    assert DEFAULT_ROOT == bench / "cache"
    ignored = (bench / ".gitignore").read_text(encoding="utf-8").split()
    assert "cache/" in ignored


def test_the_cache_writes_only_under_its_own_root(cache_root, tmp_path):
    c = Cache(cache_root, "cfg")
    src = tmp_path / "state"
    src.mkdir()
    (src / "f").write_text("x", encoding="utf-8")
    k = c.key("repo", "sha", "natural", "rtdd")
    c.put_json(k, {"n": 1})
    c.store_artifact(k, src)

    assert c._json_path(k).is_relative_to(cache_root)
    assert c._artifact_path(k).is_relative_to(cache_root)


def _tree(root: pathlib.Path) -> dict[str, bytes | None]:
    out: dict[str, bytes | None] = {}
    for p in sorted(root.rglob("*")):
        rel = p.relative_to(root).as_posix()
        out[rel] = None if p.is_dir() else p.read_bytes()
    return out
