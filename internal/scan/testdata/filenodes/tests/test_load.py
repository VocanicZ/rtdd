from pathlib import Path

ROOT = Path(__file__).parent.parent


def test_load():
    prompt = (ROOT / "prompts/x.md").read_text()
    conf = (ROOT / "conf" / "app.yaml").read_text()
    assert prompt and conf


def test_unrelated():
    assert 1 + 1 == 2
