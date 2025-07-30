module github.com/yair/where-its-at/pkg/collectors

go 1.21

require (
	github.com/mattn/go-sqlite3 v1.14.22
	github.com/yair/where-its-at/pkg/domain v0.0.0
)

replace github.com/yair/where-its-at/pkg/domain => ../domain
