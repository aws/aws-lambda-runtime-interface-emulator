# Contributing Guidelines

Thank you for your interest in contributing to our project. Whether it's a bug report, new feature, correction, or additional
documentation, we greatly value feedback and contributions from our community.

Please read through this document before submitting any issues or pull requests to ensure we have all the necessary
information to effectively respond to your bug report or contribution.

**Please note:** this repository is generated from an internal AWS source of truth, which means we are unable to merge
pull requests into it directly. We very much still want your input -- see
[How we handle contributions](#how-we-handle-contributions) for what happens to an issue or pull request you open, and
how you get credit for a change we adopt.


## Reporting Bugs/Feature Requests

We welcome you to use the GitHub issue tracker to report bugs or suggest features. Issues are the most effective way to
reach us: a feature request that describes your use case can be picked up and implemented, whereas a pull request cannot
be merged as-is.

When filing an issue, please check existing open, or recently closed, issues to make sure somebody else hasn't already
reported the issue. Please try to include as much information as you can. Details like these are incredibly useful:

* A reproducible test case or series of steps
* The version of our code being used
* Any modifications you've made relevant to the bug
* Anything unusual about your environment or deployment


## How we handle contributions

This repository contains the source code and examples for the Runtime Interface Emulator. The code here is generated from
an internal AWS repository, which is the source of truth, and changes flow outward from there. Because of that we cannot
merge a pull request into this repository, even one we agree with.

That does not mean we don't want it. Here is what each kind of contribution gets you:

* **Feature requests and bug reports are welcome, and are the most useful thing you can send us.** Open an issue
  describing the behaviour you want and the use case behind it. Our priority is maintaining fidelity with AWS Lambda's
  Runtime Interface in the cloud, so a clear use case is what lets us weigh a change against that.
* **Pull requests are welcome as a reference.** We read them, and a working patch is often the clearest way to explain a
  proposal. It will not be merged directly. If we adopt your solution, we make the corresponding change in the internal
  repository, and it reaches this repository through the next sync.
* **If we adopt your change, we credit you in the release notes** for the version that ships it.

So that we can act on a pull request, please:

1. Open an issue first to discuss any significant work -- we would hate for your time to be wasted on something we
   cannot take.
2. Work against the latest source on the *main* branch.
3. Check existing open, and recently closed, pull requests and issues to make sure someone else hasn't raised it already.
4. Focus on the specific change you are proposing. If you also reformat all the code, it will be hard for us to see what
   you are actually suggesting.
5. Ensure local tests pass through `make integ-tests-and-compile`.
6. Pay attention to any automated CI failures reported in the pull request, and stay involved in the conversation.

GitHub provides additional documentation on [forking a repository](https://help.github.com/articles/fork-a-repo/) and
[creating a pull request](https://help.github.com/articles/creating-a-pull-request/).


## Finding contributions to work on
Looking at the existing issues is a great way to find something to contribute on. As our projects, by default, use the default GitHub issue labels (enhancement/bug/duplicate/help wanted/invalid/question/wontfix), looking at any 'help wanted' issues is a great place to start.


## Code of Conduct
This project has adopted the [Amazon Open Source Code of Conduct](https://aws.github.io/code-of-conduct).
For more information see the [Code of Conduct FAQ](https://aws.github.io/code-of-conduct-faq) or contact
opensource-codeofconduct@amazon.com with any additional questions or comments.


## Security issue notifications
If you discover a potential security issue in this project we ask that you notify AWS/Amazon Security via our [vulnerability reporting page](http://aws.amazon.com/security/vulnerability-reporting/). Please do **not** create a public github issue.


## Licensing

See the [LICENSE](LICENSE) file for our project's licensing. We will ask you to confirm the licensing of your
contribution, including for a pull request we adopt rather than merge.
