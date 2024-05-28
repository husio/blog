---
title: "Task runner in Bash"
date: 2024-05-28
tags: [Bash]
---

Every project grows to a point where a set of custom tasks must be executed on various occasions.
It is good to write those commands down, so that they don't get lost and anyone can execute them.
I used to maintain a [_Makefile_](https://makefiletutorial.com/) as a simple way to organize and share tasks with others.

As the complexity grows, and more functionality is needed, Makefile becomes more unreadable.
It feels like using the wrong tool for the job.

And indeed, there is a better tools - shell scripts.

The below [Bash](https://en.wikipedia.org/wiki/Bash_(Unix_shell)) script is a solid base to extend.


```bash
#!/usr/bin/env bash

set -euo pipefail


function task:help {
	# Print this script help.
	local tasks
	local self_path
	local desc

	printf "%s <task> [args]\n\nTasks:\n" "${0}"
	tasks=$(compgen -A function | sed -En 's/task:(.*)/\1/p')
	self_path=$(realpath "$0")
	for task in ${tasks}; do
		desc=$(grep "function task:$task {" "$self_path" -A 1 | sed -En 's/.*# (.*)/\1/p')
		printf "  %-32s\t%s\n" "$task" "$desc"
	done
}

# shellcheck disable=SC2145
"task:${@:-help}"
```

In order to register a new task, define a `task:<name>` function.
The first line, when comment, is used as that task documentation.

```bash
function task:say-hello {
  # Greet the user.
  echo "Hello $USER"
}
```

When the script is called with no arguments, it runs `help` that renders the list of available tasks with their description.

```sh
% ./run
./run <task> [args]

Tasks:
  say-hello                             Greet the user.
  help                                  Print this script help.
```
