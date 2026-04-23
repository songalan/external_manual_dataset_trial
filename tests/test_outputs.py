#!/usr/bin/env python3
"""
Automated verifier for dbmate status command enhancements.

Test categories:
  P* - Problem reproduction tests (init should fail / lack feature)
  F* - Feature acceptance tests (final should pass)
  G* - Non-regression tests (both init/final should pass)
"""

import os
import re
import subprocess
import sys
import json


ISSUE_DIR = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
INIT_DIR = os.path.join(ISSUE_DIR, "init")
FINAL_DIR = os.path.join(ISSUE_DIR, "final")
WORKSPACE_DIR = os.path.join(ISSUE_DIR, "workspace")


def run_cmd(cmd, cwd=None, timeout=300):
    """Run a shell command and return (returncode, stdout, stderr)."""
    result = subprocess.run(
        cmd, shell=True, cwd=cwd, capture_output=True, text=True, timeout=timeout
    )
    return result.returncode, result.stdout, result.stderr


def read_file(filepath):
    """Read a file and return its content."""
    with open(filepath, "r", encoding="utf-8", errors="replace") as f:
        return f.read()


# ──────────────────────────────────────────
# P1: Problem reproduction - init lacks new status features
# ──────────────────────────────────────────
def test_P1_init_lacks_pagination_and_filter():
    """P1: init code should NOT contain pagination/filter/color functions in db.go"""
    db_go = read_file(os.path.join(INIT_DIR, "pkg", "dbmate", "db.go"))
    main_go = read_file(os.path.join(INIT_DIR, "main.go"))

    # These functions should NOT exist in init
    missing_functions = []
    for func_name in ["PaginateDashboardResult", "LimitDashboardResult", "FilterDashboardResult"]:
        if f"func {func_name}" in db_go:
            missing_functions.append(func_name)

    # These CLI flags should NOT exist in init main.go
    missing_flags = []
    for flag_name in ["--page", "--page-size", "--limit", "--filter", "--pretty", "--color", "--no-color"]:
        # Check in flag definitions
        if flag_name in main_go:
            missing_flags.append(flag_name)

    assert len(missing_functions) == 0, (
        f"P1 FAIL: init db.go should NOT contain these functions: {missing_functions}"
    )
    # At least pagination-related flags should be absent
    pagination_absent = "--page" not in main_go and "--page-size" not in main_go
    assert pagination_absent, (
        "P1 FAIL: init main.go should NOT contain --page or --page-size flags"
    )
    print("P1 PASS: init code correctly lacks pagination/filter/color features")


# ──────────────────────────────────────────
# F1: Feature acceptance - final code builds successfully
# ──────────────────────────────────────────
def test_F1_final_builds():
    """F1: final code should compile successfully with go build"""
    rc, stdout, stderr = run_cmd("go build ./...", cwd=FINAL_DIR, timeout=300)
    assert rc == 0, (
        f"F1 FAIL: go build failed in final\nstdout: {stdout}\nstderr: {stderr}"
    )
    print("F1 PASS: final code builds successfully")


# ──────────────────────────────────────────
# F2: Feature acceptance - final code contains new functions and flags
# ──────────────────────────────────────────
def test_F2_final_contains_new_features():
    """F2: final code should contain pagination/limit/filter/color functions and CLI flags"""
    db_go = read_file(os.path.join(FINAL_DIR, "pkg", "dbmate", "db.go"))
    main_go = read_file(os.path.join(FINAL_DIR, "main.go"))

    # Check for new functions in db.go
    required_functions = [
        "PaginateDashboardResult",
        "LimitDashboardResult",
        "FilterDashboardResult",
    ]
    for func_name in required_functions:
        assert f"func {func_name}" in db_go, (
            f"F2 FAIL: final db.go missing function: {func_name}"
        )

    # Check for new CLI flags in main.go
    required_flags = ["--page", "--page-size", "--limit", "--filter", "--pretty", "--color", "--no-color"]
    for flag in required_flags:
        assert flag in main_go, (
            f"F2 FAIL: final main.go missing flag: {flag}"
        )

    # Check PrintDashboardTable has color parameter
    assert "useColor" in db_go, (
        "F2 FAIL: final db.go PrintDashboardTable missing useColor parameter"
    )

    # Check PrintDashboardJSON has indent parameter
    assert "indent string" in db_go or "indent" in db_go, (
        "F2 FAIL: final db.go PrintDashboardJSON missing indent parameter"
    )

    print("F2 PASS: final code contains all new features")


# ──────────────────────────────────────────
# F3: Feature acceptance - dashboard tests pass in final
# ──────────────────────────────────────────
def test_F3_final_dashboard_tests_pass():
    """F3: final code dashboard tests should pass"""
    rc, stdout, stderr = run_cmd(
        "go test -v -run TestPaginate -run TestLimit -run TestFilter -run TestPrintDashboard ./pkg/dbmate/...",
        cwd=FINAL_DIR,
        timeout=300,
    )
    assert rc == 0, (
        f"F3 FAIL: dashboard tests failed in final\nstdout: {stdout}\nstderr: {stderr}"
    )
    print("F3 PASS: final dashboard tests pass")


# ──────────────────────────────────────────
# G1: Non-regression - go vet passes in final
# ──────────────────────────────────────────
def test_G1_final_go_vet():
    """G1: final code should pass go vet"""
    rc, stdout, stderr = run_cmd("go vet ./...", cwd=FINAL_DIR, timeout=300)
    assert rc == 0, (
        f"G1 FAIL: go vet found issues in final\nstdout: {stdout}\nstderr: {stderr}"
    )
    print("G1 PASS: final code passes go vet")


if __name__ == "__main__":
    tests = [
        ("P1", test_P1_init_lacks_pagination_and_filter),
        ("F1", test_F1_final_builds),
        ("F2", test_F2_final_contains_new_features),
        ("F3", test_F3_final_dashboard_tests_pass),
        ("G1", test_G1_final_go_vet),
    ]

    passed = 0
    failed = 0
    for name, test_fn in tests:
        try:
            test_fn()
            passed += 1
        except AssertionError as e:
            print(str(e), file=sys.stderr)
            failed += 1
        except Exception as e:
            print(f"{name} ERROR: {e}", file=sys.stderr)
            failed += 1

    print(f"\n{'='*50}")
    print(f"Results: {passed} passed, {failed} failed out of {len(tests)} tests")
    sys.exit(0 if failed == 0 else 1)
