---
title: "Testing in Go"
date: 2020-01-30
tags: [Go, Testing]
toc: true
---

This is a collection of testing techniques and patterns that I have learned
throughout my career of being a Go programmer.


## `testing` package basics

The Go standard library comes with the
[`testing`](https://golang.org/pkg/testing/) package which provides a solid
base for writing tests.

Each test should be a separate function. A test function must accept a single
argument of type [`*testing.T`](https://golang.org/pkg/testing/#T).

A test for a functoin `isEven` could look like this:

```go
func TestIsEven(t *testing.T) {
    if !isEven(2) {
        t.Fatal("2 is even")
    }
    if isEven(1) {
        t.Fatal("1 is odd")
    }
}
```

Run your test by using the [`go
test`](https://golang.org/cmd/go/#hdr-Test_packages) command, for example

```sh
# Test this directory
$ go test .

# Test the whole project recursively.
$ go test a-package.com/path/...
```


### Failing and messages

Each test accepts one argument, a `T` instance. `T` provides methods that
allow to print information and control the flow of a test.

Use `t.Log` and `t.Logf` methods to write a message.

Use `t.Error` and `t.Errorf` methods to write a message and mark the test as
failed.

Use `t.Fatal` and `t.Fatalf` methods to write a message, mark the test as
failed and instantly terminate that test execution.


#### Write good error messages

A good error message is concise and short. Sprinkle each result with a bit of
context.

```go
if isEven(1) {
    t.Fatal("1 is an odd number")
}
if want, got := 42, compute(); want != got {
    t.Fatalf("want %d, got %d", want, got)
}
```
By declaring `got` and `want` I am sure that what is tested for is what I
print. If the `compute` function was changed and in the new implementation `want`
should be `33` I cannot make the mistake of not updating the error message.
Both `got` and `want` are scoped to the `if` statement only.

When writing a table test, declaring an expected value might not be necessary.
The expected value can be easily found in the test declaration.


### Skipping a test

Some tests should run only under special circumstances. For example, you want
to run a test only if a database is available. `t.Skip` and `t.Skipf` methods
allow to cancel (skip) the currently running test without failing it.

```go
func TestDatabaseIntegration(t *testing.T) {
    db, err := connectToDatabase("test-database")
    if err != nil {
        t.Skipf("cannot connect to database: %s", err)
    }
    defer db.Close()

    // ...
}
```


### Test helpers


Often times many tests require similar dependencies, for example running a
service or preparing a state. Instead of repeating the preparation code extract
each functionality to a separate function.


### Test helpers: Setting up dependencies

If you are testing code that depends on an external database, this is how the
beginning of a test function might look like:

```go
func TestDatabaseIntegration(t *testing.T) {
    db, err := connectToDatabase("test-database")
    if err != nil {
        t.Skipf("cannot connect to database: %s", err)
    }
    defer db.Close()
    if err := db.Ping(); err != nil {
        t.Fatalf("cannot ping database: %s", err)
    }

    for i, migration := range databaseMigrations {
        if err := db.ApplyMigration(migration); err != nil {
            t.Fatalf("cannot apply %d migration: %s", i, err)
        }
    }

    mycollection := NewCollection(db)

    // The actual test starts below.
    // ...
}
```

A solution to code repetition can be to create a function that will encapsulate
certain functionality. The whole setup and teardown process for a test can be
extracted.


```go
func TestDatabaseIntegration(t *testing.T) {
    mycollection, cleanup := ensureMyCollection(t, "test-database")
    defer cleanup()

    // The actual test starts below.
    // ...
}

func ensureMyCollection(t testing.TB), dbName string (MyCollection, func(){} {
    t.Helper()

    db, err := connectToDatabase(dbName)
    if err != nil {
        t.Skipf("cannot connect to database: %s", err)
    }

    if err := db.Ping(); err != nil {
        db.Close()
        t.Fatalf("cannot ping database: %s", err)
    }

    for i, migration := range databaseMigrations {
        if err := db.ApplyMigration(migration); err != nil {
            db.Close()
            t.Fatalf("cannot apply %d migration: %s", i, err)
        }
    }
    collection := NewCollection(db)
    cleanup := func() {
        db.Close()
    }
    return collection, cleanup
}
```

With the above solution, `ensureMyCollection` can be used by many test
functions to ensure that a collection using a database as a backend is
available. A helper function hides the for the test logic irrelevant part of
setting up an environment and ensuring all components are provided.

A helper function accepts [`testing.TB`](https://golang.org/pkg/testing/#TB)
interface instead of `t *testing.T`. That makes it useful for both test and
[benchmark functions](https://golang.org/pkg/testing/#hdr-Benchmarks).

A helper function does not return an error. Instead, it directly terminates the
test by calling `t.Fatal`.

At the beginning of the helper function the
[`t.Helper()`](https://golang.org/pkg/testing/#T.Helper) method is called. This
marks this function and when it fails the stack information and error will be
more helpful.

`ensureMyCollection` returns a cleanup function. This is a convenient way of
cleaning up all created resources. The user of this helper must call it once
the returned resource is not needed anymore. The cleanup function should not
return anything nor fail the test.


### Blackbox package testing

> Test files that declare a package with the suffix "_test" will be compiled as
> a separate package, and then linked and run with the main test binary.
> -- [golang.org](https://golang.org/cmd/go/#hdr-Test_packages)

Test files for your package are located in the same directory as the code they
test. Your tests can belong to the same package as the rest of the code. It is
also possible to enforce a black-box test for your package. Your test files can
be in the same directory as your package code and use a different package name.

```go
package xxx_test
```

Using a different test package name enforces that only the public interface of
the tested package is accessible. This is for example [how
`strings`](https://golang.org/src/strings/compare_test.go) and [`bytes`
packages](https://golang.org/src/bytes/reader_test.go) are tested.



### Third party test helper packages

I do not use any additional packages for testing. I am of an opinion that
[assert functions are not as helpful as one may
think](https://golang.org/doc/faq#testing_framework). Introducing an external
package requires learning a new API.

Someone else wrote [a great
summary](https://danmux.com/posts/the_cult_of_go_test/) on the topic.

Complex comparisons can usually be done using
[`reflect.DeepEqual`](#reflectdeepequal) function.


## `reflect.DeepEqual`

Those values that cannot be compared with `==`, most of the time can be
compared with [`reflect.DeepEqual`](https://golang.org/pkg/reflect/#DeepEqual).


## Table tests

When testing a functionality a single input is often not enough to ensure
correctness. Repeating the same operation for many cases can be implemented
using [table tests](https://github.com/golang/go/wiki/TableDrivenTests).

Use a map with strings as keys to provide a description of each test case.

```go
func TestDiv(t *testing.T) {
    cases := map[string]struct{
        A int
        B int
        WantRes int
        WantErr error
    }{
        "two positive numbers": {
            A: 4,
            B: 2,
            WantRes: 2,
        },
        "divide by zero": {
            A: 4,
            B: 0,
            WantErr: errors.ErrZeroDivision,
        },
    }

    for testName, tc := range cases {
        t.Run(testName, func(t *testing.T) {
            res, err := Div(tc.A, tc.B)
            if !errors.Is(err, tc.WantErr) {
                t.Fatalf("unexpected error: %q", err)
            }
            if res != tc.WantRes {
                t.Fatalf("unlexpected result: %d", res)
            }
        })
    }
}
```

When declaring a test case, always use field names. This increases the
readability and you have to provide only non zero values.

```go
cases := map[string]struct{
    DB *Database
    Req *Request
    WantRes int
    WantErr error
}{
    // BAD
    {nil, myrequest, 32, nil},

    // GOOD
    {
        Req: myrequest,
        WantRes: 32,
    },
}
```

## Mocking

Write your code to accept interfaces. Using interfaces allows you to test a
single layer of a functionality at a time.

For example, if you are writing an application that is storing data in an SQL
database, instead of accessing the database directly through a `*sql.DB`
instance [use a wrapper](/blog/accessing-data-in-go/#mocking-for-tests). Using
a data access abstraction allows for mocking.

When writing a mock you do not have to implement all methods. For the compiler
it is enough to include the interface in the mock declaration. Implement only
methods that you intend to call.

```go
type Collection interface {
    One(id uint64) (*Entity, error)
    List() ([]*Entity, error)
    Add(Entity) error
    Delete(id uint64) error
}

type CollectionMock struct {
    Collection
    Err error
}

func (c *CollectionMock) Add(Entity) error {
    return c.Err
}
```

`CollectionMock` implements the `Collection` interface, but using any other
method than `Add` will panic. See [the full
example](https://play.golang.org/p/GVc2tOJoAHX).


### Your code should provide a mock

When writing a package that is used by others provide test implementations of
your interfaces.

This approach is taken by the standard library. For example,
[`httptest.ResponseRecorder`](https://golang.org/pkg/net/http/httptest/#ResponseRecorder)
allows to test your HTTP handler without using a real `http.ResponseWriter`.


## Test flags

You can add your own flags to the `go test` command in order to customize your
tests. Use the [`flag`](https://golang.org/pkg/flag/) package and declare your
flags globally.

```go
var dbFl = flag.String("db", "", "Use given database DSN.")
```

## Environment variables

Instead of `flag` you can control your tests using environment variables. If
you follow the [12 factor app](https://12factor.net/config) principles then
your application is already utilizing environment variables for the
configuration.

```go
var dbDSN = os.Getenv("DATABASE_DSN")
```


## Fixtures

If your test requires fixtures `/testdata` is the directory you should consider
keeping them in.

>  The go tool will ignore a directory named "testdata", making it available to
>  hold ancillary data needed by the tests.
> -- [golang.org](https://golang.org/cmd/go/#hdr-Test_packages)

When running tests each test function is executed with its working directory
set to the source directory of the tested package. That means that when
accessing files in `/testdata` you can safely use relative path

```go
fd, err := os.Open(filepath.Join("testdata", "some-fixture.json"))
```

## Golden files

[Golden files](https://softwareengineering.stackexchange.com/q/358786) are a
great way to validate and keep track of a test output. Together with a version
control system they are much easier to maintain than strings hard coded in
functions.


```go
var goldFl = flag.Bool("gold", false, "Write result to golden files instead of comparing with them.")


func TestExample(t *testing.T) {
    // Test logic.
    result := ...

    if *goldFl {
        writeGoldenFile(t, result)
    }

    compareWithGoldenFile(t, result)
}
```

This technique comes in very helpful combined with [table tests](#table-tests).


## Integration tests

For a well written application [integration
testing](https://en.wikipedia.org/wiki/Integration_testing) should not require
more work than usual testing. For each external resource provide a single
function to [setup and teardown the
resource](#test-helpers-setting-up-dependencies).


## Build constraints

You can use a [build
constraint](https://golang.org/pkg/go/build/#hdr-Build_Constraints) to
conditionally build code in a file.


```sh
$ head -n 1 app_intergration_test.go
// +build integration
```

To run tests including those tagged as `integration` use `-tag` flag.

```sh
$ go test -tag integration .
```




## Setup/teardown

When using the `testing` package, it is possible to overwrite the [`test
main`](https://golang.org/pkg/testing/#hdr-Main) function.

Using a custom test main function allows to execute code before and after
executing all discovered tests. This can be running an external dependency like
a database instance or building a binary that tested functionality might depend
on.

```go
func TestMain(m *testing.M) {
    // Setup code.

    // defer Teardown code.

    os.Exit(m.Run())
}
```


## `-race`

Run tests with `-race` flag to enable data race detection.

This functionality is not available on [musl](https://www.musl-libc.org/) based systems.

## Testing FAQ

Check the [FAQ at golang.org](https://golang.org/doc/faq#Packages_Testing).
