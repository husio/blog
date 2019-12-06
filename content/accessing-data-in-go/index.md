---
title: "Accessing data in Go"
date: 2018-09-02
tags: [Go]
comments: [reddit, https://www.reddit.com/r/golang/comments/9cbxwg/accessing_data_in_go/]
---

When writing a web application, we have to decide how to access data. Where to
get it from, how to store it, how to manipulate it. Storage engines can vary,
from being a single SQLite file to cache server or even an external service
exposing an API.

There are many ways this topic can be addressed. I will explain how a simple
and straightforward solution can be evolved into a more sophisticated one.


For the purpose of this article, let's assume that our storage engine is an SQL
database with an `items` table. Our task is to build an endpoint, which returns
a list of all _items_ in the database. _Item_ is an entity with a name and an
ID. It can be represented by the structure below.


```go
type Item struct {
	ID   int64
	Name string
}
```

## First iteration

Let's start with a basic HTTP handler. To avoid global variables, let's use
dependency injection. `ItemListHandler` takes as a parameter what's necessary
for the endpoint to complete our task -- a database connection and a template.
In return we are getting an HTTP handler function.


```go
func ItemListHandler(
	db *sql.DB,
	tmpl *template.Template,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// handler's code below
	}
}
```

To list all _items_, we must first query the database. Once we will read all
returned rows, we can use the collected entries to render the template and send
the result back.


```go
rows, err := db.QueryContext(r.Context(), `SELECT id, name FROM items`)
if err != nil {
	http.Error(w, "Server Error", http.StatusInternalServerError)
	return
}
defer rows.Close()

var items []*Item
for rows.Next() {
	var it Item
	if err := rows.Scan(&it.ID, &it.Name); err != nil {
		http.Error(w, "Server Error", http.StatusInternalServerError)
		return
	}
	items = append(items, &it)
}
if err := rows.Err(); err != nil {
	http.Error(w, "Server Error", http.StatusInternalServerError)
	return
}

_ = tmpl.Execute(w, items)
```

_(To simplify the example, returned error pages are very basic, we do not log
errors and we are assuming that template rendering never fails.)_

There are many issues with the approach presented above.

1. Every time we want to get the list of _items_, we must directly interact
   with the database. We must know about the database structure and in case of
   schema changes, we must locate all those places and update them.

2. Everything is implemented in a single place. Because we directly access the
   database, to test this code, a database must be available, it's schema
   prepared and test data inserted.

3. If we wanted to add a cache layer or some form of monitoring like tracing or
   metrics, we would have to add more code directly inside of the handler.
   That makes the code of the handler larger and testing harder. We can no
   longer test functionalities separately.


## Second iteration

Instead of writing all the code in an HTTP handler, let's extract a part of it
as a function. We can encapsulate fetching items and hide the database
connection from the user.

The same code that was written directly inside of the handler is now provided
by the `ListItems` method.


```go
// NewItemStore returns a store for items.
func NewItemStore(db *sql.DB) *ItemStore {
	return &ItemStore{db: db}
}

type ItemStore struct {
	db *sql.DB
}

// ListItems returns all stored items.
func (is *ItemStore) ListItems(ctx context.Context) ([]*Item, error) {
	rows, err := db.QueryContext(ctx, `SELECT id, name FROM items`)
	if err != nil {
		return nil, fmt.Errorf("cannot select items: %s", err)
	}
	defer rows.Close()

	var items []*Item
	for rows.Next() {
		var it Item
		if err := rows.Scan(&it.ID, &it.Name); err != nil {
			return nil, fmt.Errorf("cannot scan item: %s", err)
		}
		items = append(items, &it)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("scanner: %s", err)
	}
	return items, nil
}
```

Having such a _store_ available, we no longer have to directly query the
database in our handler. Instead of accepting `*sql.DB` as an argument,
`ItemListHandler` can now take `*ItemStore`. Handler's body can be simplified
to just a few lines.

```go
func ItemListHandler(
	itemStore *ItemStore,
	tmpl *template.Template,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		items, err := itemStore.ListItems(r.Context())
		if err != nil {
			http.Error(w, "Server Error", http.StatusInternalServerError)
		}
		_ = tmpl.Execute(w, items)
	}
}
```

Having this handler, we no longer have to track changes to the database schema.
All details of accessing _item_ data are now in `ItemStore`. If you need to
create or update an _item_, add `CreateItem` and `UpdateItem` methods.

## Third iteration

Using `*ItemStore` for accessing _items_ solved the first issue. Listing items
is now an easy task that takes only a few lines of code.

The last change is to use an interface instead of accepting a structure
pointer.  Let's call our interface `ItemStore`. The previous implementation
using an SQL database is renamed to `sqlItemStore`.


```go
type ItemStore interface {
	ListItems(context.Context) ([]*Item, error)
}

// NewItemStore returns a store for items that is using an SQL database
// as a storage engine.
func NewSQLItemStore(db *sql.DB) ItemStore {
	return &sqlItemStore{db: db}
}

type sqlItemStore struct {
	db *sql.DB
}

func (s *sqlItemStore) ListItems(ctx context.Context) ([]*Item, error) {
	// ...
}

func ItemListHandler(
	itemStore ItemStore,
	tmpl *template.Template,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// ...
	}
}
```

Defining interfaces together with the implementation might feel
counterintuitive in Go. In most cases, it is better to declare an interface
where it is used (not where it is implemented) to help to decouple
functionalities and avoid dependencies.

In this case we do not use an interface to encourage different `ItemStore`
implementations. Code that is used for accessing _items_ could be put in it's
own package and provide all necessary functionality -- an interface, the main
implementation using an SQL database, a mock implementation for testing and more.

### Mocking for tests

The `sqlItemStore` implementation is easy to test independently from any HTTP
handler that is using it. Any handler that is using an `ItemStore` should also
be testable without the need for any particular `ItemStore` implementation.

When testing handlers, instead of providing a real `ItemStore` implementation,
we can use a mock.

```go
type ItemStoreMock struct {
	Items []*Item
	Err   error
}

// ensure mock always implements the ItemStore
var _ ItemStore = (*ItemStoreMock)(nil)

func (mock *ItemStoreMock) ListItems(context.Context) ([]*Item, error) {
	return mock.Items, mock.Err
}
```

`ItemStoreMock` gives us full control over its API. We control what each
method returns, which means we are able to test all cases we want.

### Caching

Using an interface, allows us to wrap a store with additional functionality.
For example, we can provide a cache layer, that will be invisible to the user.
It can be added or removed without any changes to handler or store
implementations.

```go
type CacheStore interface {
	// Get loads value under given key into destValue. ErrMiss is returned
	// if key does not exist.
	Get(ctx context.Context, key string, destValue interface{}) error
	// Set value of given key.
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
}


func CacheItemStore(cache CacheStore, store ItemStore) ItemStore {
	return &cachedItemStore{
		cache: cache,
		store: store,
		ttl:   5 * time.Minute,
	}
}

type cachedItemStore struct {
	cache CacheStore
	store ItemStore
	ttl   time.Duration
}

func (c *cachedItemStore) ListItems(context.Context) ([]*Item, error) {
	var items []*Item
	switch err := c.cache.Get(ctx, "items:all", &items); err {
	case nil:
		return items, nil
	case ErrMiss:
		// all good, just not in the cache
	default:
		// log the error and continue
	}

	items, err := c.store.ListItems(ctx)

	if err == nil {
		if err := c.cache.Set(ctx, "items:all", items, c.ttl); err != nil {
			// log the error and continue
		}
	}

	return items, err
}
```

Testing of the `cachedItemStore` can be done using `ItemStoreMock` and an
in-memory cache backend.


## Conclusion

Writing data managers requires more effort, but allows to separate business
logic from storage implementation. Separation of concerns gives us more control
over data.

Thanks to using Go interfaces, we can mock and extend functionality of the
storage implementation. Integration with cache or monitoring tools is easy,
pluggable and can be tested separately.
