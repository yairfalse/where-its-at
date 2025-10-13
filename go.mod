module github.com/yair/where-its-at

go 1.24.0

toolchain go1.24.5

require (
	github.com/gorilla/mux v1.8.1
	github.com/mattn/go-sqlite3 v1.14.32
	github.com/yair/where-its-at/pkg/collectors v0.0.0
	github.com/yair/where-its-at/pkg/config v0.0.0
	github.com/yair/where-its-at/pkg/integrations v0.0.0
	github.com/yair/where-its-at/pkg/interfaces v0.0.0
)

require (
	github.com/yair/where-its-at/pkg/domain v0.0.0 // indirect
	golang.org/x/net v0.46.0 // indirect
)

replace github.com/yair/where-its-at/pkg/domain => ./pkg/domain

replace github.com/yair/where-its-at/pkg/collectors => ./pkg/collectors

replace github.com/yair/where-its-at/pkg/config => ./pkg/config

replace github.com/yair/where-its-at/pkg/integrations => ./pkg/integrations

replace github.com/yair/where-its-at/pkg/interfaces => ./pkg/interfaces
