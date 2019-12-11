GTest
=====

[![Documentation](https://godoc.org/github.com/houqp/gtest?status.svg)](https://godoc.org/github.com/houqp/gtest)
[![goreportcard](https://goreportcard.com/badge/github.com/houqp/gtest)](https://goreportcard.com/report/github.com/houqp/gtest)
[![codecov](https://codecov.io/gh/houqp/gtest/branch/master/graphs/badge.svg?branch=master)](https://codecov.io/gh/houqp/gtest)
[![CircleCI](https://circleci.com/gh/houqp/gtest.svg?style=svg)](https://circleci.com/gh/houqp/gtest)

Lightweight Golang test framework inspired by pytest.

GTest provides the following functionalities to help reduce boilerplate in test code:

* Test grouping
* Setup, Teardown hooks for test groups
* BeforeEach, AfterEach hooks for tests
* Fixture injection

See [docs](http://godoc.org/github.com/houqp/gtest), [example_test.go](./example_test.go) and [gtest_test.go](./gtest_test.go) for examples.
