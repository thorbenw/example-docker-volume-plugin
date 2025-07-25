Example Docker Volume Plugin
============================

This is another example driver like others present on
[GitHub](https://github.com) using the
[official Plugins Helpers](https://github.com/docker/go-plugins-helpers) like
the [example shim driver](https://github.com/docker/go-plugins-helpers/blob/main/volume/shim/shim.go),
[marcelo-ochoa/docker-volume-plugins](https://github.com/marcelo-ochoa/docker-volume-plugins),
[fntlnz/docker-volume-plugin-example](https://github.com/fntlnz/docker-volume-plugin-example),
[vieux/docker-volume-sshfs](https://github.com/vieux/docker-volume-sshfs)
and others.

# Basic features
- Version 2.0 Docker plugin
- No 3rd party package dependencies
- Supports volume processes, i.e. for each volume, a process can be maintained
  (e.g. started on volume creation, restarted on crash and terminated on volume
  removal.)

# Helpful links
- [Process Management in Go](https://hackernoon.com/everything-you-need-to-know-about-managing-go-processes)
- [Preventing shallow copies with the `noCopy` struct](https://stackoverflow.com/questions/68183168/how-to-force-compiler-error-if-struct-shallow-copy)
- [Semantic Versioning 2.0.0](https://semver.org/)
- [W3Schools Go Tutorial](https://www.w3schools.com/go/go_syntax.php)
- [Go Constructors Tutorial](https://tutorialedge.net/golang/go-constructors-tutorial/)
- [Go Wiki: Well-known struct tags](https://go.dev/wiki/Well-known-struct-tags) (e.g. [`json`](https://pkg.go.dev/encoding/json#Marshal))
- [Comprehensive Guide to Type Assertion in Go](https://medium.com/@jamal.kaksouri/mastering-type-assertion-in-go-a-comprehensive-guide-216864b4ea4d)
- [Implementing (i.e. faking) Enums in Go](https://builtin.com/software-engineering-perspectives/golang-enum)
- [Demystifying Synchronous and Asynchronous Programming in Go](https://pandazblog.hashnode.dev/synchronous-vs-asynchronous-programming-in-golang)
- [Guide to Graceful Shutdown in Go](https://medium.com/@karthianandhanit/a-guide-to-graceful-shutdown-in-go-with-goroutines-and-context-1ebe3654cac8)
- [Docker Engine managed plugin system documentation](https://docs.docker.com/engine/extend/)
- [Docker Plugins: How to write your own](https://www.inovex.de/de/blog/docker-plugins/)
