prompter
=======

[![Build Status](https://travis-ci.org/Songmu/prompter.png?branch=master)][travis]
[![Coverage Status](https://coveralls.io/repos/Songmu/prompter/badge.png?branch=master)][coveralls]
[![MIT License](http://img.shields.io/badge/license-MIT-blue.svg?style=flat-square)][license]
[![Go Documentation](http://img.shields.io/badge/go-documentation-blue.svg?style=flat-square)][godocs]

[travis]: https://travis-ci.org/Songmu/prompter
[coveralls]: https://coveralls.io/r/Songmu/prompter?branch=master
[license]: https://github.com/Songmu/prompter/blob/master/LICENSE
[godocs]: http://godoc.org/github.com/Songmu/prompter

## Description

utility for easy prompting

## Synopsis

	lang := prompter.Choose("Which language do you like the most?", []string{"Perl", "Golang", "Scala", "Ruby"}, "Perl")
	twitterID := prompter.Prompt("Enter your twitter ID", "")
	passwd := prompter.Password("Enter your password")
	var likeSushi bool = prompter.YN("Do you like sushi?", true)
	var likeBeer bool = prompter.YesNo("Do you like beer?", false)

## Author

[Songmu](https://github.com/Songmu)
