import pathlib

import pytest


@pytest.fixture
def cache_root(tmp_path: pathlib.Path) -> pathlib.Path:
    return tmp_path / "cache"
