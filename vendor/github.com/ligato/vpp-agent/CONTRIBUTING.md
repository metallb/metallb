# Contributing

Contributions to VPP-Agent are welcome. We use the standard pull request
model. You can either pick an open issue and assign it to yourself or open
a new issue and discuss your feature.

In any case, before submitting your pull request please check the 
[Coding style](CODINGSTYLE.md) and cover the newly added code with tests 
and documentation.

The tool used for managing third-party dependencies is
[dep](https://github.com/golang/dep). After adding or updating a
dependency in `Gopkg.toml` run `make dep-install` to download the specified
dependencies into the vendor folder.
To update all of the project's dependencies run `make dep-update`.
