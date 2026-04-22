"""
Automated verifier tests for the ipguard middleware issue.

P* = Problem reproduction tests (should FAIL on init, PASS on final)
F* = Feature acceptance tests (should FAIL on init, PASS on final)
G* = Non-regression guard tests (should PASS on both init and final)
"""

import subprocess
import sys
import os
import re

WORKSPACE = os.environ.get("ISSUE_WORKSPACE", os.path.join(os.path.dirname(__file__), "..", "workspace"))


def run_go_test(test_name_pattern):
    """Run a specific Go test in the ipguard package and return success + output."""
    cmd = ["go", "test", "-v", "-count=1", "-run", test_name_pattern, "./middleware/ipguard/"]
    result = subprocess.run(cmd, capture_output=True, text=True, cwd=WORKSPACE)
    return result.returncode == 0, result.stdout + result.stderr


def run_go_build():
    """Verify the Go project builds successfully."""
    cmd = ["go", "build", "./..."]
    result = subprocess.run(cmd, capture_output=True, text=True, cwd=WORKSPACE)
    return result.returncode == 0, result.stdout + result.stderr


# ──────────────────────────────────────────
# G1: Project builds without errors (non-regression)
# ──────────────────────────────────────────
def test_G1_project_builds():
    """G1: The Go project should build successfully (both init and final)."""
    success, output = run_go_build()
    assert success, f"Go build failed:\n{output}"


# ──────────────────────────────────────────
# P1: ipguard package does not exist in init
# ──────────────────────────────────────────
def test_P1_ipguard_not_in_init():
    """P1: In init state, the ipguard middleware package should not exist."""
    ipguard_dir = os.path.join(WORKSPACE, "middleware", "ipguard")
    exists = os.path.isdir(ipguard_dir)
    # This test should FAIL in init (ipguard doesn't exist) and PASS in final (ipguard exists)
    # For verifier purposes: we check that the package IS present in final
    assert exists, f"ipguard package directory does not exist at {ipguard_dir}"


# ──────────────────────────────────────────
# F1: ipguard basic IP whitelist test passes
# ──────────────────────────────────────────
def test_F1_basic_ip_whitelist():
    """F1: ipguard middleware correctly allows/blocks IPs based on whitelist."""
    success, output = run_go_test("TestIPGuard_AllowedIP")
    assert success, f"Basic IP whitelist test failed:\n{output}"


# ──────────────────────────────────────────
# F2: X-Real-IP support works
# ──────────────────────────────────────────
def test_F2_x_real_ip_support():
    """F2: ipguard correctly parses X-Real-IP header for client IP extraction."""
    success, output = run_go_test("TestIPGuard_XRealIP")
    assert success, f"X-Real-IP test failed:\n{output}"


# ──────────────────────────────────────────
# F3: IPv4-mapped IPv6 normalization works
# ──────────────────────────────────────────
def test_F3_ipv4_mapped_ipv6():
    """F3: ipguard normalizes IPv4-mapped IPv6 addresses (e.g. ::ffff:1.2.3.4 → 1.2.3.4)."""
    success, output = run_go_test("TestIPGuard_NormalizeIPv4MappedIPv6")
    assert success, f"IPv4-mapped IPv6 normalization test failed:\n{output}"


# ──────────────────────────────────────────
# F4: ClientIPFromContext works
# ──────────────────────────────────────────
def test_F4_client_ip_from_context():
    """F4: ClientIPFromContext retrieves the resolved client IP from the Fiber context."""
    success, output = run_go_test("TestIPGuard_ClientIPFromContext")
    assert success, f"ClientIPFromContext test failed:\n{output}"


if __name__ == "__main__":
    import pytest
    sys.exit(pytest.main([__file__, "-v"]))
