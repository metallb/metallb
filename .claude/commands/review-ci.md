---
description: Review CI status for current branch with PR details and job results
argument-hint: [run-identifier]
---

## Synopsis
```
/review-ci
/review-ci <github-url>
/review-ci <commit-id>
/review-ci pick
```

## Description

Review CI status for the current branch.

The command accepts:
- No argument: Run with latest completed run (whatever that is)
- GitHub URL: Extract run ID from the GitHub Actions URL (skip user/branch/PR lookup)
- Commit ID: Find the run associated with that commit (skip user/branch/PR lookup)
- "pick": Show list of runs and let user select

For empty argument (latest run):
  1. Get current user: `gh api user --jq .login` and store in variable
  2. Get current branch: `git rev-parse --abbrev-ref HEAD` and store in variable
  3. Get latest run with: `gh run list -u <user> -b <branch> --limit 1 --json status,databaseId,conclusion,displayTitle,workflowName` and store workflow name
  4. Check the status field:
     - If status is `in_progress` or `queued`: Show which jobs are in progress and which have failed, then use AskUserQuestion to ask if user wants to continue with this run or pick from last 10 runs
     - If status is `completed`: Use this run and continue

For URL or commit ID argument:
  - Skip user/branch/PR lookup steps
  - Get run details to extract commit hash and run ID
  - Use first 9 characters of commit hash for folder name
  - Save artifacts to `aistuff/<commit>/` (not `aistuff/<branch-name>/<commit>/`)

## Implementation

1. Find PR for current branch (skip if URL or commit ID provided):
   - Get current branch: `git rev-parse --abbrev-ref HEAD`
   - Find PR: `gh pr list --head <branch> --json number --jq '.[0].number'`
2. Display PR details (title, author, labels) using `gh pr view <pr-number> --json number,title,url,headRefName,author,labels` (skip if URL or commit ID provided)
3. Show CI check results for the selected run
4. If failed jobs exist: Use AskUserQuestion to present failed jobs as selectable options (show job name and conclusion)
5. Fetch logs and artifacts for the selected failed job:
   - Determine folder: `aistuff/<branch-name>/<commit>/` if branch available, otherwise `aistuff/<commit>/`
   - Commit is first 9 characters of commit hash
   - Check if folder artifacts exist
   - If exists: Skip download and use cached artifacts
   - If not exists:
     - Download job logs with `gh api repos/{owner}/{repo}/actions/jobs/{job-id}/logs`
     - Download artifacts with `gh run download <run-id> --dir <folder>/artifacts`
6. Analyze the failure:
   - Read `.github/workflows/<workflow-name>` (from Description step 3) to understand workflow structure
   - Read failed job logs from `<folder>/` to identify the failures
   - Use `metallb-ci-artifacts` skill to analyze artifacts in `<folder>/artifacts/` and provide available files (e.g., node logs, pod logs, etc.) for the next analysis step
   - Identify the root cause based on findings
   - Present a plan to the user with findings and next steps
   - Use AskUserQuestion to ask user for feedback: proceed with the plan, modify it, or choose different analysis:
     - Option 1: Proceed with the proposed plan
     - Option 2: Modify the plan (user provides feedback)
   - Present the findings
   - Follow up ask user what they want next:
     - Option 1: Get statistics (how many times this job failed in main branch)
     - Option 2: Propose possible solutions
     - Option 3: Show how to run this e2e test locally (setup, invoke, and focus on failed test)

## Output Format

Run the command with minimal output during execution. Only print a comprehensive summary at the end with:

### Branch & PR Info
- PR number, title (with URL if available)
- Current branch and commit of the CI run
- Failed job name with run URL and job URL
- Root cause


## Notes

- **Caching**: Only cache downloaded artifacts in `aistuff/<branch-name>/<commit>/artifacts/` to avoid re-downloading. Do not cache PR info, checks, or run lists.
- **Skills Required**: `gh-cli` - Use `man gh-run` to review available options

## metallb-ci-artifacts skill

This skill needs to be created to analyze MetalLB CI artifacts structure and provide available log files.

Expected artifact structure for e2e tests:
- `tree aistuff/<branch>/<commit>/artifacts/kind_logs_<test-name>/ -L1` shows:
  - `kind-control-plane/` - Control plane node logs
  - `kind-worker/` - Worker node logs
  - `kind-worker2/` - Worker node 2 logs
  - `kind-version.txt` - Kind version used
  - Per e2e test that failed: folder named after the test (e.g., `removes-all-pools/`) containing:
    - MetalLB CRD lists: `*List.log` (BFDProfileList, BGPAdvertisementList, IPAddressPoolList, etc.)
    - Pod logs: `metallb-system_*_pods_logs.log` (controller, speaker, operator pods)
    - Pod specs: `metallb-system_*_pods_specs.log`
    - Cluster state: `events.log`, `nodes.log`
