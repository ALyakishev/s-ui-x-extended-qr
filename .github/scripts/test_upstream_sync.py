"""Offline integration tests: real Git merges/pushes, mocked GitHub PR API."""

import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

from upstream_sync import BASE, HEAD, Sync, SyncError, main


def git(directory, *args):
    return subprocess.check_output(["git", "-C", str(directory), *args], text=True,
                                   stderr=subprocess.STDOUT).strip()


class FakeGitHub(Sync):
    def __init__(self, upstream):
        super().__init__("owner/fork", str(upstream))
        self.prs = []
        self.creates = 0
        self.edits = 0
        self.calls = []
        self.messages = []
        self.fail_create = False

    def gh(self, *args):
        self.calls.append(args)
        if args[:2] == ("pr", "list"):
            return json.dumps(self.prs)
        if args[:2] == ("pr", "create"):
            if self.fail_create:
                raise SyncError("simulated API failure")
            self.creates += 1
            self.prs.append({"number": 7, "url": "https://example.invalid/pull/7", "isCrossRepository": False})
            return self.prs[-1]["url"]
        if args[:2] == ("pr", "edit"):
            self.edits += 1
            self.asserted_body = Path(args[args.index("--body-file") + 1]).read_text()
            return ""
        raise AssertionError(args)

    def report(self, message):
        self.messages.append(message)


class SyncTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        self.seed = self.root / "seed"
        self.seed.mkdir()
        git(self.seed, "init", "-b", "upstream-main")
        self.identity(self.seed)
        (self.seed / "shared.txt").write_text("\n".join(f"line {i}" for i in range(30)) + "\n")
        self.commit(self.seed, "common ancestor")
        self.upstream = self.root / "upstream.git"
        self.origin = self.root / "fork.git"
        git(self.root, "clone", "--bare", str(self.seed), str(self.upstream))
        git(self.root, "clone", "--bare", str(self.seed), str(self.origin))
        self.work = self.root / "runner"
        git(self.root, "clone", str(self.origin), str(self.work))
        self.identity(self.work)
        git(self.work, "checkout", "-b", BASE)
        self.change_line(self.work, 1, "fork TrustTunnel change")
        (self.work / "own-only.txt").write_text("native tt:// generator\n")
        (self.work / "frontend.txt").write_text("fork frontend\n")
        self.commit(self.work, "fork customizations")
        git(self.work, "push", "origin", BASE)
        self.base = git(self.origin, "rev-parse", BASE)
        self.sync = FakeGitHub(self.upstream)
        self.previous = Path.cwd()
        os.chdir(self.work)
        self.addCleanup(os.chdir, self.previous)

    def identity(self, repo):
        git(repo, "config", "user.name", "Sync test")
        git(repo, "config", "user.email", "test@example.invalid")

    def commit(self, repo, message):
        git(repo, "add", ".")
        git(repo, "-c", "core.hooksPath=/dev/null", "-c", "commit.gpgsign=false", "commit", "-m", message)

    def change_line(self, repo, index, text):
        p = repo / "shared.txt"
        lines = p.read_text().splitlines()
        lines[index] = text
        p.write_text("\n".join(lines) + "\n")

    def upstream_commit(self, name="upstream addition", conflict=False):
        if conflict:
            self.change_line(self.seed, 1, "conflicting upstream change")
        else:
            (self.seed / (name + ".txt")).write_text(name)
        self.commit(self.seed, name)
        git(self.seed, "push", str(self.upstream), "upstream-main")

    def remote_sync(self):
        return git(self.origin, "rev-parse", HEAD)

    def assert_base_unchanged(self):
        self.assertEqual(git(self.origin, "rev-parse", BASE), self.base)

    def test_no_updates_does_not_call_pr_api_or_create_branch(self):
        self.assertEqual(self.sync.run(), "no-updates")
        self.assertEqual(self.sync.calls, [])
        with self.assertRaises(subprocess.CalledProcessError):
            self.remote_sync()
        self.assert_base_unchanged()

    def test_new_updates_create_pr_and_retain_fork_changes(self):
        self.upstream_commit()
        self.assertEqual(self.sync.run(), "pr-created")
        self.assertEqual(self.sync.creates, 1)
        self.assertIn("upstream-main", self.sync.messages[0])
        tip = self.remote_sync()
        git(self.origin, "merge-base", "--is-ancestor", self.base, tip)
        git(self.origin, "merge-base", "--is-ancestor", git(self.upstream, "rev-parse", "HEAD"), tip)
        self.assertEqual(git(self.origin, "show", f"{HEAD}:own-only.txt"), "native tt:// generator")
        self.assertEqual(git(self.origin, "show", f"{HEAD}:frontend.txt"), "fork frontend")
        self.assertIn("fork TrustTunnel change", git(self.origin, "show", f"{HEAD}:shared.txt"))
        self.assert_base_unchanged()

    def test_repeated_runs_update_one_pr_without_redundant_commits(self):
        self.upstream_commit()
        self.sync.run()
        first = self.remote_sync()
        self.assertEqual(self.sync.run(), "pr-updated")
        self.assertEqual(self.remote_sync(), first)
        self.upstream_commit("second addition")
        self.assertEqual(self.sync.run(), "pr-updated")
        self.assertNotEqual(self.remote_sync(), first)
        git(self.origin, "merge-base", "--is-ancestor", first, self.remote_sync())
        self.assertEqual((self.sync.creates, self.sync.edits, len(self.sync.prs)), (1, 2, 1))
        self.assert_base_unchanged()

    def test_conflict_never_publishes_branch_or_pr(self):
        self.upstream_commit(conflict=True)
        with self.assertRaisesRegex(SyncError, "shared.txt"):
            self.sync.run()
        self.assertEqual(self.sync.creates, 0)
        with self.assertRaises(subprocess.CalledProcessError):
            self.remote_sync()
        self.assertEqual(git(self.work, "status", "--porcelain"), "")
        self.assert_base_unchanged()

    def test_squash_merged_sync_does_not_create_empty_pr_next_day(self):
        self.upstream_commit()
        self.sync.run()
        tip = self.remote_sync()
        git(self.work, "checkout", BASE)
        git(self.work, "merge", "--squash", tip)
        self.commit(self.work, "squashed sync PR")
        git(self.work, "push", "origin", BASE)
        self.base = git(self.origin, "rev-parse", BASE)
        self.sync.prs = []  # The first sync PR has been merged and closed.
        self.assertEqual(self.sync.run(), "no-updates")
        self.assertEqual(self.sync.creates, 1)
        self.assertEqual(self.remote_sync(), tip)
        self.assert_base_unchanged()

    def test_conflict_preserves_existing_sync_branch_and_pr(self):
        self.upstream_commit()
        self.sync.run()
        before = self.remote_sync()
        self.upstream_commit("conflict", conflict=True)
        with self.assertRaises(SyncError):
            self.sync.run()
        self.assertEqual(self.remote_sync(), before)
        self.assertEqual((self.sync.creates, self.sync.edits), (1, 0))
        self.assert_base_unchanged()

    def test_existing_sync_edits_and_new_base_are_preserved(self):
        self.upstream_commit()
        self.sync.run()
        (self.work / "manual-review.txt").write_text("reviewer edit")
        self.commit(self.work, "manual sync edit")
        git(self.work, "push", "origin", HEAD)
        git(self.work, "checkout", BASE)
        (self.work / "new-base.txt").write_text("new fork change")
        self.commit(self.work, "new base change")
        git(self.work, "push", "origin", BASE)
        self.base = git(self.origin, "rev-parse", BASE)
        self.upstream_commit("next update")
        self.sync.run()
        self.assertEqual(git(self.origin, "show", f"{HEAD}:manual-review.txt"), "reviewer edit")
        self.assertEqual(git(self.origin, "show", f"{HEAD}:new-base.txt"), "new fork change")
        self.assert_base_unchanged()

    def test_retry_after_api_create_failure_reuses_published_branch(self):
        self.upstream_commit()
        self.sync.fail_create = True
        with self.assertRaises(SyncError):
            self.sync.run()
        tip = self.remote_sync()
        self.sync.fail_create = False
        self.assertEqual(self.sync.run(), "pr-created")
        self.assertEqual(self.remote_sync(), tip)
        self.assertEqual(self.sync.creates, 1)

    def test_duplicate_existing_prs_stop_before_push(self):
        self.upstream_commit()
        self.sync.prs = [{"number": 1, "isCrossRepository": False}, {"number": 2, "isCrossRepository": False}]
        with self.assertRaisesRegex(SyncError, "Multiple"):
            self.sync.run()
        with self.assertRaises(subprocess.CalledProcessError):
            self.remote_sync()

    def test_missing_pat_has_no_git_side_effects(self):
        with patch.dict(os.environ, {"GITHUB_REPOSITORY": "owner/fork", "GH_TOKEN": ""}), \
                patch.object(Sync, "git", side_effect=AssertionError("Git must not run")), \
                patch.object(Sync, "report"):
            self.assertEqual(main(), 1)

    def test_auto_merge_enabled_pr_stops_before_branch_update(self):
        self.upstream_commit()
        self.sync.prs = [{"number": 7, "isCrossRepository": False,
                          "autoMergeRequest": {"enabledAt": "2026-01-01"}}]
        with self.assertRaisesRegex(SyncError, "Disable auto-merge"):
            self.sync.run()
        with self.assertRaises(subprocess.CalledProcessError):
            self.remote_sync()

    def test_external_fork_pr_with_same_branch_name_is_not_edited(self):
        self.upstream_commit()
        self.sync.prs = [{"number": 99, "url": "https://example.invalid/pull/99", "isCrossRepository": True}]
        self.assertEqual(self.sync.run(), "pr-created")
        self.assertEqual(self.sync.creates, 1)
        self.assertEqual(self.sync.edits, 0)


if __name__ == "__main__":
    unittest.main()
