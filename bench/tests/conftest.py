from __future__ import annotations

import pathlib
import sys

import pytest

sys.path.insert(0, str(pathlib.Path(__file__).resolve().parents[1]))

from tests.synthrepo import SynthRepo, build_synth_repo  # noqa: E402


@pytest.fixture
def synth(tmp_path: pathlib.Path) -> SynthRepo:
    return build_synth_repo(tmp_path / "synth")


@pytest.fixture
def cache_root(tmp_path: pathlib.Path) -> pathlib.Path:
    return tmp_path / "cache"
