import pytest

from replay.hardware import (
    CI_ENV_VARS,
    CIWallClockRefused,
    Hardware,
    detect_ci,
    probe,
    require_wallclock,
)


def test_detects_common_ci_environments():
    assert detect_ci({"GITHUB_ACTIONS": "true"}) == "GITHUB_ACTIONS"
    assert detect_ci({"CI": "1"}) == "CI"
    assert detect_ci({"BUILDKITE": "true"}) == "BUILDKITE"
    assert detect_ci({"CI": "false"}) is None
    assert detect_ci({"HOME": "/home/x"}) is None


def test_every_documented_ci_env_var_is_detected():
    for name in CI_ENV_VARS:
        assert detect_ci({name: "true"}) == name


def test_probe_records_the_machine_and_flags_ci():
    hw = probe({"GITHUB_ACTIONS": "true"})
    assert hw.ci == "GITHUB_ACTIONS"
    assert hw.cpu_count >= 1
    assert hw.python_version
    assert not hw.wallclock_allowed()


def test_probe_records_cpu_model_memory_and_os_even_under_ci():
    hw = probe({"GITHUB_ACTIONS": "true"})
    assert hw.cpu_model
    assert hw.mem_total_kb >= 0
    assert hw.platform
    d = hw.to_dict()
    assert d["cpu_model"] == hw.cpu_model
    assert d["fingerprint"] == hw.fingerprint()


def test_require_wallclock_refuses_on_ci_and_allows_on_a_disclosed_machine():
    ci = probe({"GITHUB_ACTIONS": "true"})
    with pytest.raises(CIWallClockRefused):
        require_wallclock(ci)
    local = probe({})
    assert local.ci is None
    require_wallclock(local)


def test_fingerprint_is_stable_and_excludes_ci_flag():
    a = Hardware("Xeon", 8, 1024, "Linux-6", "3.12.4", None)
    b = Hardware("Xeon", 8, 1024, "Linux-6", "3.12.4", "CI")
    assert a.fingerprint() == b.fingerprint()
