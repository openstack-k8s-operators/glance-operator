# Contributing to the glance operator

Thank you for taking the time to contribute!

The following is a set of guidelines for contributing to the [glance-operator
hosted in GitHub](https://github.com/openstack-k8s-operators/glance-operator).
hosted in GitHub. Feel free to propose changes to this document or any other
in a pull request.

## What should I know before I get started?

The glance-operator is not a large open source project, but it's part
of the [OpenStack podification effort](https://github.com/openstack-k8s-operators)
which is of some size.

There are some requirements to deploy a working Glance service within OpenShift
using the glance-operator. For simplicity's sake all the documentation will
assume that the [openstack-operator](https://github.com/openstack-k8s-operators/openstack-operator)
will be used to deploy necessary services, though this doesn't mean that
there are no other ways to do it.

Before working on your first code contribution it would be a good idea to
complete the [getting started](README.md#getting-started) section first to
get familiar with the deploying of the services and to get a feeling of
the behavior of a working OpenStack deployment.

Please refer to the [OpenStack documentation](https://docs.openstack.org) to
learn about OpenStack itself.

### Design decisions

There is some [global podification
documentation](https://github.com/openstack-k8s-operators/docs) relevant for
all the operators, but there are also some global design decisions that have
not been spelled out yet anywhere else, so they are included in the
[glance-operator design decisions document](docs/design-decisions.md).

### How to contribute?

This project is [using pull
request](https://docs.github.com/en/pull-requests/collaborating-with-pull-requests/proposing-changes-to-your-work-with-pull-requests/about-pull-requests)
to submit changes, but it is not currently using the [issues
section](https://github.com/openstack-k8s-operators/glance-operator/issues) or
any other GitHub feature to track the project's bugs or features and instead it
is expected that whoever finds an issue will fix it or will find someone else
to work on it.

### Testing changes

Developers submitting PRs are expected to have run the code and manually
verified the functionality before submitting the PR, with the exception
of documentation only and CI only PRs.

There are multiple ways to test code changes, but in general terms they can be
split into 2 categories: running things locally or in a container in the
OpenShift cluster.

Running things locally is considerably faster but it doesn't use the real ACLs
(RBACs) the glance-operator would use on a normal deployment. On the other hand
running things in OpenShift is slower (because we have to build a new
container) but runs as close as a real deployment as we can.

Each of the approaches has its own advantages and disadvantages, so different
variants of both approaches will be covered for readers to choose the one they
prefer.

The following list of articles assumes that podified OpenStack has been
deployed using the openstack-operator as described in the [Getting started
section](README.md#getting-started):

- [Run operator locally](docs/local.md)

### Debugging

When working on the glance-operator there will be times where things won't work
as expected and we'll need to debug things.

In the [debugging article](https://github.com/openstack-k8s-operators/dev-docs/blob/main/debugging.md)
we present some ideas to help you figure things out.

### Pull Requests

While the pull request flow is used for submitting new issues and for
reviewing, the git repository should **always** be the source of truth, so all
decisions made through the reviewing and design phases should be properly
reflected in the final code, code comments, documentation, and git commit
message that is merged.  It should not be necessary to go to a PR in GitHub and
go through the comments to know what a commit is meant to do or why it is doing
it.

*One-liner* commit messages should be avoided like the plague.  Please refer to
the [commit messages guide line](#commit-messages) for good practices on commit
messages.

The glance-operator project will not squash all pull request commits into a
single commit but will look to preserve all submitted individual commits
instead using a merge strategy instead.  This means that we can have both
single commit and as multi-commit PRs, and both have their places. It's all
about how and when to split changes.

#### Good practice for Dependency Management

When submitting a PR that has dependencies in other repositories these
dependencies should be stated in the PR or the commits using the `Depends-On:`
tag.

There are 4 ways to state a dependency with another projects PR:
- `Depends-On: lib-common=88`
- `Depends-On: lib-common#88`
- `Depends-On: openstack-k8s-operators/lib-common#88`
- `Depends-On: https://github.com/openstack-k8s-operators/lib-common/88`

#### Structural split of changes

The general rule for how to split code changes into commits is that we should
aim to have a single "logical change" per commit.

As the [OpenStack Git Commit Good
Practice](https://wiki.openstack.org/wiki/GitCommitMessages) explains, there
are multiple reasons why this rule is important:

- The smaller the amount of code being changed in a commit, the quicker &
  easier it is to review & identify potential flaws in that specific code
  change.
- If a change is found to be flawed later, it may be necessary to revert the
  broken commit. This is much easier to do if there are not other unrelated
  code changes entangled with the original commit.
- When troubleshooting problems using Git's bisect capability, small well
  defined changes will aid in isolating exactly where the code problem was
  introduced.
- When browsing history using Git annotate/blame, small well defined changes
  also aid in isolating exactly where & why a piece of code came from.

In the [OpenStack Git Commit Good Practice
page](https://wiki.openstack.org/wiki/GitCommitMessages) we can also find two
great sections explaining [things to avoid when creating
commits](https://wiki.openstack.org/wiki/GitCommitMessages#Things_to_avoid_when_creating_commits)
and [examples of bad
practice](https://wiki.openstack.org/wiki/GitCommitMessages#Examples_of_bad_practice)
that contributors to this repository should take into consideration.

#### PR Images

Once a PR merges it will trigger an image rebuild and publish, so how can we
tell when the new image is ready?

For that we'll go to the [project's
actions](https://github.com/openstack-k8s-operators/glance-operator/actions)
and in there [we select the `Glance Operator image builder`
job](https://github.com/openstack-k8s-operators/glance-operator/actions/workflows/build-glance-operator.yaml)
where we'll see the job that is building the new operator container image.

In there how the job is building 3 images: Operator, Bundle, and Index, and if
you go into each one of them and expand the `Push **** To quay.io` step you can
find the actual image location.

For example:

```
Successfully pushed "glance-operator-index:9f3d1ec26ba8939710f146f3e6a1d81f5077be8a" to "quay.io/***/glance-operator-index:9f3d1ec26ba8939710f146f3e6a1d81f5077be8a"
```

## Style Guides

### Go Style Guide

While the project has not formally defined a Go Style Guide for the project it
uses [Gofmt](https://pkg.go.dev/cmd/gofmt) for automated code formatting.

This tool is automatically called when building the glance-operator binary
using the `Makefile` or when it is manually invoked with:

```sh
make fmt
```

Pull Requests are expected to have passed the formatting tool before committing
the code and submitting the PR. [pre-commit](https://github.com/openstack-k8s-operators/glance-operator/blob/main/.pre-commit-config.yaml)
and other related git hook can be used to check locally if patch is going to
pass related checks (job).

### Commit Messages

As mentioned before, commit messages are a very important part of a pull
request, and their contents must be carefully crafted.

Commit messages should:

- Provide a brief description of the change in the first line.
- Insert a single blank line after the first line.
- Provide a detailed description of the change in the following lines, breaking
  paragraphs where needed.
- Lines should be wrapped at 72 characters.

Once again the OpenStack documentation goes over the [important things to
consider when writing the commit
message](https://wiki.openstack.org/wiki/GitCommitMessages#Information_in_commit_messages),
so please take a good look.  The short version is:

- Do not assume the reviewer understands what the original problem was.
- Do not assume the reviewer has access to external web services/site.
- Do not assume the code is self-evident/self-documenting.
- Describe why a change is being made.
- Read the commit message to see if it hints at improved code structure.
- The first commit line is the most important.
- Describe any limitations of the current code.
- Do not assume the reviewer has knowledge of the tests executed on the change
- Do not include patch set-specific comments.
