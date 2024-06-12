---
title: Release Process
weight: 2
---

This is the process to follow to make a new release. This should
really be automated someday, but releases are juuuust infrequent
enough that it's less annoying to just do the work each time.

### Check the issue tracker milestone

Are you actually ready to release? Check the milestone on github and
verify that all its issues are closed. If there are open issues,
you'll have to either resolve them, or bump to the next version.

### Merge the main branch

MetalLB uses release branches to track releases. In case a new release is cut as a mirror of main, merge can
be used.
For example:

```bash
git checkout v0.9
git merge main
git push
```

#### Using cherry picks

In case only a subset of the changes are brought to the new release, cherry-pick
must be used.

```bash
git checkout v0.9
git cherry-pick -x f1f86ed658c1e8a6f90f967ed94881d61476b4c0
git push
```

Note that this will break the release notes generator. If this fix is a backport, please
consider filing a backport PR.

### Generate the release notes

A convenience generator script is added under `website/gen_relnotes.sh`. The syntax is
as follows:

```bash
website/gen_relnotes.sh <branch> <first commit> <last commit>
```

Where branch is the branch being released, first and last commit is the interval
we want to generate the release notes for.

The `GITHUB_TOKEN` environment variable must be set with a github token which has the following permissions:

Read access to:

- Contents
- Pull requests
- Commit statuses

### Finalize release notes

All release notes are always written on the `main` branch, and
copied into release branches in a later step. Point out all new
features and actions required by users. If there are very notable
bugfixes (e.g. security issues, long-term pain point resolved), point
those out as well.

Also update the documentation link so that the soon-to-be latest
release's documentation link points to `metallb.io`, and the
previous releases point to `vX-Y-Z--metallb.netlify.com`, which is the
website pinned at that tagged release.

To get a list of contributors to the release, run `git log
--format="%aN" $(git merge-base CUR-BRANCH PREV-TAG)..CUR-BRANCH | sort -u |
tr '\n' ',' | sed -e 's/,/, /g'`. `CUR-BRANCH` is `main` if you're
making a minor release (e.g. 0.9.0), or the release branch for the
current version if you're making a patch release (e.g. `v0.8` if
you're making `v0.8.4`). `PREV-TAG` is the release tag name for the
last release (e.g. if you're preparing 0.8.4, `PREV-TAG` is
`v0.8.3`. Also think about whether there were significant
contributions that weren't in the form of a commit, and include those
people as well. It's better to err on the side of being _too_
thankful!

Commit the finalized release notes.

### Clean the working directory

The release script only works if the Git working directory is
_completely_ clean: no pending modifications, no untracked files,
nothing. Make sure everything is clean, or run the release from a
fresh checkout.

The release script will abort if the working directory isn't right.

### Run the release script

Run `inv release X.Y.Z`. This will create the appropriate branches,
commits and tags in your local repository.

### Push the new artifacts

Run `git push --tags origin main vX.Y`. This will push all pending
changes both in main and the release branch, as well as the new tag
for the release.

### Protect the release branch (skip for patch releases)

For major and minor releases, the release script created a new `vX.Y`
branch. Go into github's repository settings and mark the branch
protected, including from administrators, to guard against accidental
force pushes.

### Create a new release on github

By default, new tags show up de-emphasized in the list of
releases. Create a new release attached to the tag you just
pushed. Make the description point to the release notes on the
website.

### Wait for the image repositories to update

When you pushed, CI kicked off a set of image builds for the new
tag. You need to wait for these images to be pushed live before
continuing, because the manifests for the new release point to image
tags that don't exist until CI makes them exist.

Check on Quay for a `vX.Y.Z` tag on each image, or check on
CI that the deploy has completed.

### Repoint the live website

Move the `live-website` branch to the newly created tag with `git
branch -f live-website vX.Y.Z`, then force-push the branch with `git
push -f origin live-website`. This will trigger Netlify to
redeploy [metallb.io](https://metallb.io) with
updated documentation for the new version.

### Update Slack

Update the topic in `#metallb` on Kubernetes slack.

### Brag about new release

Tweet, post to G+, slack, IRC, whatever. Make some noise, if it's
worth making noise about!
