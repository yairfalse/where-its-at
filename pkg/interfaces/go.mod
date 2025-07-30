module github.com/yair/where-its-at/pkg/interfaces

go 1.21

require (
	github.com/gorilla/mux v1.8.1
	github.com/yair/where-its-at/pkg/domain v0.0.0
	github.com/yair/where-its-at/pkg/integrations v0.0.0
)

replace github.com/yair/where-its-at/pkg/domain => ../domain

replace github.com/yair/where-its-at/pkg/collectors => ../collectors

replace github.com/yair/where-its-at/pkg/integrations => ../integrations
