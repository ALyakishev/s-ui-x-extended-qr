"""Merge upstream for review; never execute upstream code or merge a PR."""

import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile


BASE = "s-ui-x-extended"
HEAD = "sync/upstream"
UPSTREAM = "https://github.com/deposist/s-ui-x-extended.git"


class SyncError(RuntimeError):
    pass


class Sync:
    def __init__(self, repository, upstream=UPSTREAM):
        self.repository = repository
        self.upstream = upstream

    def command(self, *args, check=True):
        result = subprocess.run(args, text=True, capture_output=True)
        if check and result.returncode:
            raise SyncError(f"{args[0]} failed: {result.stderr.strip()}")
        return result

    def git(self, *args, check=True):
        return self.command("git", "-c", "core.hooksPath=/dev/null",
                            "-c", "commit.gpgsign=false", *args, check=check)

    def gh(self, *args):
        return self.command("gh", *args, "--repo", self.repository).stdout

    def report(self, message):
        print(message)
        if path := os.environ.get("GITHUB_STEP_SUMMARY"):
            with open(path, "a", encoding="utf-8") as summary:
                summary.write(message + "\n\n")

    def ancestor(self, older, newer):
        result = self.git("merge-base", "--is-ancestor", older, newer, check=False)
        if result.returncode not in (0, 1):
            raise SyncError("Cannot establish Git ancestry; refusing to sync")
        return result.returncode == 0

    def pull_requests(self):
        prs = json.loads(self.gh("pr", "list", "--state", "open", "--base", BASE,
                                 "--head", HEAD, "--json", "number,url,isCrossRepository,autoMergeRequest"))
        # A contributor fork may have a branch with the same name. Never edit
        # that PR or mistake it for our own sync branch's review.
        prs = [pr for pr in prs if pr.get("isCrossRepository") is False]
        if len(prs) > 1:
            raise SyncError("Multiple open sync PRs found; manual review required")
        if prs and prs[0].get("autoMergeRequest") is not None:
            raise SyncError("Disable auto-merge on the existing sync PR before syncing; refusing to update it")
        return prs

    def merge(self, ref):
        result = self.git("merge", "--no-ff", "--no-edit", ref, check=False)
        if result.returncode:
            conflicts = self.git("diff", "--name-only", "--diff-filter=U").stdout.splitlines()
            self.git("merge", "--abort", check=False)
            detail = ", ".join(json.dumps(p) for p in conflicts) or result.stderr.strip()
            raise SyncError(f"Merge failed for {ref}: {detail}. Remote branch and PR were not changed; resolve manually.")

    def run(self):
        if self.git("status", "--porcelain").stdout.strip():
            raise SyncError("Working tree must be clean")
        self.git("fetch", "--no-tags", "origin", f"refs/heads/{BASE}:refs/remotes/origin/{BASE}")
        base = self.git("rev-parse", f"refs/remotes/origin/{BASE}").stdout.strip()
        remote = self.git("remote", "get-url", "upstream", check=False)
        if remote.returncode:
            self.git("remote", "add", "upstream", self.upstream)
        elif remote.stdout.strip() != self.upstream:
            raise SyncError("Unexpected upstream remote URL")
        default = self.git("ls-remote", "--symref", "upstream", "HEAD").stdout
        match = re.search(r"^ref: refs/heads/(.+)\tHEAD$", default, re.MULTILINE)
        if not match:
            raise SyncError("Cannot determine upstream default branch")
        branch = match.group(1)
        self.git("check-ref-format", f"refs/heads/{branch}")
        self.git("fetch", "--no-tags", "upstream",
                 f"refs/heads/{branch}:refs/remotes/upstream/default")
        upstream = self.git("rev-parse", "refs/remotes/upstream/default").stdout.strip()
        self.report(f"Base: `{BASE}` at `{base}`. Upstream default: `{branch}` at `{upstream}`.")
        if self.ancestor(upstream, base):
            self.report("No upstream updates. No branch or PR was created or modified.")
            return "no-updates"
        # Check API access/duplicates before making any remote changes.
        self.pull_requests()
        remote_head = self.git("ls-remote", "--exit-code", "--heads", "origin",
                               f"refs/heads/{HEAD}", check=False)
        if remote_head.returncode not in (0, 2):
            raise SyncError("Cannot inspect remote sync branch")
        start = base
        if remote_head.returncode == 0:
            self.git("fetch", "--no-tags", "origin",
                     f"refs/heads/{HEAD}:refs/remotes/origin/{HEAD}")
            start = self.git("rev-parse", f"refs/remotes/origin/{HEAD}").stdout.strip()
        self.git("checkout", "-B", HEAD, start)
        # Normal merges retain fork and existing sync-branch commits. No ours/
        # theirs strategy, reset to upstream, rebase, or force push is used.
        self.merge(base)
        self.merge(upstream)
        difference = self.git("diff", "--quiet", base, "HEAD", check=False)
        if difference.returncode == 0:
            # A prior sync PR may have been squash-merged. Its upstream SHA is
            # then not an ancestor of base, despite all changes being present.
            self.report("No content changes after merge (possibly already squash-merged). No remote branch or PR was changed.")
            return "no-updates"
        if difference.returncode != 1:
            raise SyncError("Cannot compare merged content with base")
        tip = self.git("rev-parse", "HEAD").stdout.strip()
        if tip != start or remote_head.returncode == 2:
            self.git("push", "origin", f"HEAD:refs/heads/{HEAD}")
        # Re-query after push, so reruns also recover from a prior create failure.
        prs = self.pull_requests()
        body = (f"Merge upstream default branch `{branch}` into `{BASE}`.\n\n"
                f"Upstream: {UPSTREAM.removesuffix('.git')}/commit/{upstream}\n"
                f"Fork base: `{base}`\nSync tip: `{tip}`\n\n"
                "Normal Git merges preserve fork changes; conflicts stop the workflow. "
                "Review TrustTunnel, native tt:// links, frontend and CI before merging. "
                "This workflow never enables auto-merge.\n")
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "body.md"
            path.write_text(body, encoding="utf-8")
            if prs:
                self.gh("pr", "edit", str(prs[0]["number"]), "--title", "chore: sync upstream",
                        "--body-file", str(path))
                url = prs[0]["url"]
            else:
                url = self.gh("pr", "create", "--base", BASE, "--head", HEAD,
                              "--title", "chore: sync upstream", "--body-file", str(path)).strip()
        self.report(f"Review required: {url}. CI is triggered by the PAT's push/PR events. No automatic merge.")
        return "pr-updated" if prs else "pr-created"


def main():
    sync = Sync(os.environ["GITHUB_REPOSITORY"])
    try:
        if not os.environ.get("GH_TOKEN"):
            raise SyncError("Set UPSTREAM_SYNC_TOKEN in the branch-restricted upstream-sync environment; no GITHUB_TOKEN fallback is allowed")
        sync.git("config", "user.name", "github-actions[bot]")
        sync.git("config", "user.email", "41898282+github-actions[bot]@users.noreply.github.com")
        # Only the GitHub credential helper reads GH_TOKEN; no token in remotes,
        # command arguments, files, commit messages or logs.
        sync.git("config", "credential.helper", "")
        sync.git("config", "credential.https://github.com.helper", "!gh auth git-credential")
        sync.run()
    except SyncError as error:
        sync.report(f"Sync stopped: {error}")
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
