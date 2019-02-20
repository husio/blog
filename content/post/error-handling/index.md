---
title: "Error handling"
date: 2019-02-20
tags: [Go]
comments: []
---

Go is a language that does not provide exceptions. Instead, an operation can
return an error. [Errors are values](https://blog.golang.org/errors-are-values)
that implement the `error` interface.

I have worked with several errors handling patterns over the years and I would
like to summarize my experience focusing on the good solutions.

For the purpose of this post, let us imagine a very simple banking application.
Accounts are represented by their numeric ID and we only know how much money
each account holds. No account balance can get below zero. A bank service must
implement the interface below.

```go
type BankService interface {
    // NewAccount registers a new account in this bank. The account is
    // initialized with given funds.
    NewAccount(accountID int64, funds uint64) error

    // Transfer moves funds between two accounts. It fails if an operation
    // would cause the balance of the source account to go below zero.
    Transfer(from, to int64, amount uint64) error
}

```

To keep the examples short and simple an in-memory storage is used. Anything
more serious would use a database instead.

## Inline error creation

It is a common thing to create errors using `errors.New` and `fmt.Errorf` as
they are needed. When an operation fails you can handle the failure by creating
an error instance and returning it. The created error should contain
information about the cause of the failure. With that in mind let us create the
first version of a banking service.

{{< highlight go "hl_lines=14 24 26 30" >}}
func NewBank() *Bank {
    return &Bank{
        accounts: make(map[int64]uint64),
    }
}

type Bank struct {
    accounts map[int64]uint64
}

// Create new account with given funds. Account ID must be unique.
func (b *Bank) NewAccount(accountID int64, funds uint64) error {
    if _, ok := b.accounts[accountID]; ok {
        return errors.New("account exists")
    }
    b.accounts[accountID] = funds
    return nil
}

// Transfer moves funds from one account to another.
func (b *Bank) Transfer(from, to int64, amount uint64) error {
    switch fromFunds, ok := b.accounts[from]; {
    case !ok:
        return fmt.Errorf("source account %d not found", from)
    case fromFunds < amount:
        return fmt.Errorf("cannot transfer %d from %d account: insufficient funds", amount, fromFunds)
    }

    if _, ok := b.accounts[to]; !ok {
        return fmt.Errorf("destination account %d not found", to)
    }

    b.accounts[from] -= amount
    b.accounts[to] += amount
    return nil
}
{{< /highlight >}}

Above code presents a common way of dealing with errors. If a failure cannot be
dealt with then return the error. If possible provide additional information,
for example, an account ID. This is often an acceptable solution but sometimes
it might not be good enough. As soon as we use the `Bank` instance the
shortcomings are more visible.


```go
bank := NewBank()
// ...
if err := bank.Transfer(111, 222, 10); err != nil {
    // Why did the transfer fail?
}
```
If the `Transfer` call returns an error it is not possible to learn about the
reason and distinguish different cases. As a human analyzing the text message,
we can tell what went wrong. If you want your code to react differently if one
of the accounts does not exist and do something else when there are not enough
funds on the source account then you have a problem.

## Predefined errors

To provide more insights into the `Transfer` method failures one may declare
all expected errors upfront.

For each failure case declare a corresponding error instance. Compare an error
returned by the `Transfer` method with all error definitions it can return to
discover the cause.

{{< highlight go "hl_lines=9 11 15" >}}
// Transfer moves funds from one account to another.
// Upon failure returns one of
//   ErrNoSourceAccount
//   ErrNoDestinationAccount
//   ErrInsufficientFunds
func (b *Bank) Transfer(from, to int64, amount uint64) error {
    switch fromFunds, ok := b.accounts[from]; {
    case !ok:
        return ErrNoSourceAccount
    case fromFunds < amount:
        return ErrInsufficientFunds
    }

    if _, ok := b.accounts[to]; !ok {
        return ErrNoDestinationAccount
    }

    b.accounts[from] -= amount
    b.accounts[to] += amount
    return nil
}

var (
    // ErrNoSourceAccount is returned when the source account does not
    // exist.
    ErrNoSourceAccount = errors.New("no source account")

    // ErrNoDestinationAccount is returned when the destination account
    // does not exist.
    ErrNoDestinationAccount = errors.New("no destination account")

    // ErrInsufficientFunds is returned when a transfer cannot be completed
    // because there are not enough funds on the source account.
    ErrInsufficientFunds = errors.New("insufficient funds")
)
{{< /highlight >}}

This is similar to how the [`io`](https://golang.org/pkg/io/#pkg-variables)
package deals with errors.

Returning a different error instance for each error case allows us to handle
different failure cases accordingly. Test the returned error for being one of
the predefined instances.

```go
bank := NewBank()
// ...
switch err := bank.Transfer(1, 2); err {
case nil:
    println("money transferred")
case ErrNoSourceAccount:
    panic("source account does not exist")
case ErrNoDestinationAccount:
    panic("destination account does not exist")
case ErrInsufficientFunds:
    panic("not enough money")
default:
    panic("unexpected error")
}
```
This is in my opinion a step in the right direction but it is too verbose. This
patten requires too much code to be written. You can no longer create errors
when you need them. All failure cases and respective errors must be declared
upfront.

In addition, you are losing the context information that you were building
using `fmt.Errorf`. When returning `ErrInsufficientFunds` you no longer know
which account caused it. `fmt.Errorf` must no longer be used for the error
instance comparison to work.



## Error inheritance

In Python - a language with exceptions and type inheritance - [exceptions form
a hierarchy](https://docs.python.org/3/library/exceptions.html#exception-hierarchy).
Because each error is an instance of a class belonging to that class hierarchy
each exception instance can contain a custom message and be captured by its
type or any type it inherits from.

This is how a banking service could be used if implemented in Python.

```python
try:
    bank.transfer(from, to, amount)
except ErrAccountNotFound as e:
    print(e) # either source or destination account not found
except ErrInsufficientFunds:
    print("not enough money")
except Exception:
    print("unexpected condition")
```
Because in Python implementation both `ErrNoSourceAccount` and
`ErrNoDestinationAccount` would inherit from `ErrAccountNotFound`, both cases
can be handled with a single statement `except ErrAccountNotFound`.

When capturing an exception `e` refers to the exception instance containing the
detailed information that can be helpful during debugging or consumed by the
client. It can contain more information than just a human readable description.




### `Causer` interface

Inheritance is not a requirement to achieve the functionality provided by
Python exceptions. When considering an error it is enough if we are able to
tell what was the cause of it. This is not possible with errors created using
the standard library (`errors` or `fmt` packages). Instead of using the
standard library, we must create our own error implementation.

What is needed is an `Error` structure that implements the
[`error`](https://golang.org/pkg/builtin/#error) interface and a `Wrap`
function that will take an error together with an additional description.

```go
// Wrap returns an error that is having given error set as the cause.
func Wrap(err error, description string, args ...interface{}) *Error {
    return &Error{
        parent: err,
        desc:   fmt.Sprintf(description, args...),
    }
}

type Error struct {
    // Parent error if any.
    parent error
    // This error description.
    desc string
}

func (e *Error) Error() string {
    if e.parent == nil {
        return e.desc
    }
    return fmt.Sprintf("%s: %s", e.desc, e.parent)
}
```

In addition, it will provide a `Cause` method that will return the wrapped error
instance or `nil`.

```go
// Cause returns the cause of this error or nil if this is the root cause
// error.
func (e *Error) Cause() error {
    return e.parent
}
```

One more function is necessary for this to be complete. We must be able to
compare an error with another error or its cause. The `error` interface does
not provide `Cause` method so we must use type casting to determine if an error
instance implements the `causer` interface.

Instead of a function a method of the `Error` structure provides a nicer API.

```go

// Is returns true if given error or its cause is the same kind.
// If cause error provides Cause method then a comparison is made with all
// parents as well.
func (kind *Error) Is(err error) bool {
    type causer interface {
        Cause() error
    }
    for {
        if err == kind {
            return true
        }
        if e, ok := err.(causer); ok {
            err = e.Cause()
        } else {
            return false
        }
    }
}
```

Let us test the `Error`. All errors are created using the `Wrap` function which
builds an error hierarchy. It is possible to attach additional information by
including it in the description string.

```go
root := Wrap(nil, "root")
child1 := Wrap(root, "child one")
child2 := Wrap(root, "child two")

fmt.Println("child 1 is root", root.Is(child1))
// child 1 is root true

fmt.Println("child 2 is root", root.Is(child2))
// child 2 is root true

fmt.Println("root is child 1", child1.Is(root))
// root is child 1 false

fmt.Println("child 2 is child 1", child1.Is(child2))
// child 2 is child 1 false

inlinedErr := Wrap(child2, "current time: %s", time.Now())
fmt.Println("inlined child 2 is root", root.Is(inlinedErr))
// inlined child 2 is root true
fmt.Println("inlined child 2 is child 2", child2.Is(inlinedErr))
// inlined child 2 is child 2 true

fmt.Println("fmt error is root", root.Is(fmt.Errorf("fmt error")))
// fmt error is root false
```

Above `Error` implementation is a powerful solution to error handling. It is
easy to implement, does not require much code and it is portable without
creating an explicit dependency on the `causer` interface.


## Predefined errors with inheritance

If an error implements the `causer` interface we can unwind it and retrieve the
previous error instance! This means that no matter how many times we will wrap
an error, as long as all layers implement the `causer` interface we can
retrieve the parent error instance.

Back to the `Bank.Transfer` example. All error instances were wrapped before
returning and provide all the details one may expect an error to provide.

{{< highlight go "hl_lines=4 6-7 11" >}}
func (b *Bank) Transfer(from, to int64, amount uint64) error {
    switch fromFunds, ok := b.accounts[from]; {
    case !ok:
        return Wrap(ErrNoSourceAccount, "ID %d", from)
    case fromFunds < amount:
        return Wrap(ErrInsufficientFunds,
            "cannot transfer %d from %d account", amount, fromFunds)
    }

    if _, ok := b.accounts[to]; !ok {
        return Wrap(ErrNoDestinationAccount, "ID %d", to)
    }

    b.accounts[from] -= amount
    b.accounts[to] += amount
    return nil
}

var (
    // ErrAccountNotFound is return when an operation fails because the
    // requested account does not exist.
    ErrAccountNotFound = Wrap(nil, "account not found")

    // ErrNoSourceAccount is returned when the source account does not
    // exist.
    ErrNoSourceAccount = Wrap(ErrAccountNotFound, "no source")

    // ErrNoDestinationAccount is returned when the destination account
    // does not exist.
    ErrNoDestinationAccount = Wrap(ErrAccountNotFound, "no destination")

    // ErrInsufficientFunds is returned when a transfer cannot be completed
    // because there are not enough funds on the source account.
    ErrInsufficientFunds = Wrap(nil, "insufficient funds")
)
{{< /highlight >}}

Errors can be tested on any granularity level. It is valid to compare with the
high level `ErrAccountNotFound` or more precise `ErrNoSourceAccount`.

```go
bank := NewBank()
// ...
switch err := bank.Transfer(1, 2, 100); {
case err == nil:
    println("money transferred")
case ErrNoDestinationAccount.Is(err):
    panic("destination account does not exist")
case ErrInsufficientFunds.Is(err):
    panic("not enough money " + err.Error()) // err provides more details
default:
    panic("unexpected error")
}
```

## Don't Drink Too Much Cool Aid

What I have presented is a powerful pattern. You may use the `causer` interface
to extract attributes or custom error implementations that were wrapped,
attaching helpful information on each execution step. This might be great for
example during input validation, where together with an error you want to
return information about the invalid fields in a way that can be extracted
later.

You can use the `causer` interface and the `Wrap` function to declare a complex
tree of errors that are several layers deep and cover every possible case. If
you do, think again about your use case and if such granularity is helpful.
Usually, just a handful of errors declared upfront do the job better. I tend to
always inline error creation first and only if a case requires more attention
declare a previously inlined error.

Regardless of what you do try to avoid blindly importing any error package.
Consider your use cases and try to [tailor your errors implementation to suit
your needs](https://commandcenter.blogspot.com/2017/12/error-handling-in-upspin.html).
