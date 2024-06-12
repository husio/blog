---
title: "The Go standard library routing improvements"
date: 2024-06-12
tags: [Go]
---

Go 1.22 ships with [router Enhancements](https://go.dev/blog/routing-enhancements).
The [`net/http.ServeMux`](https://godocs.io/net/http#ServeMux) can now match requests by method, host and a simple path wildcard.

With the new ServeMux, it is no longer necessary to [struggle](https://benhoyt.com/writings/go-routing/) to find the best routing method. For most cases, standard library should be the best choice.
And with the next release, you can align your declarations with [any number of spaces](https://github.com/golang/go/commit/7b583fd1a1aeda98daa5a9d485b35786c031e941).

```go
func run() {
	rt := http.NewServeMux()

	rt.Handle(`POST /users`, &demoHandler{info: "create user"})
	rt.Handle(`GET  /users/{name}`, &demoHandler{info: "show user"})
	rt.Handle(`GET  /users/{name}/profile`, &demoHandler{info: "show user profile"})

	_ = http.ListenAndServe("localhost:8000", rt)
}

type demoHandler struct {
	info string
}

func (h *demoHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, h.info, r.PathValue("name"))
}

```


```sh
% curl localhost:8000/users
Method Not Allowed
% curl localhost:8000/users -X POST
create user
% curl localhost:8000/users/andy
show user andy
% curl localhost:8000/users/andy/profile
show user profile andy
% curl localhost:8000/users/andy/profile -X POST
Method Not Allowed
```
