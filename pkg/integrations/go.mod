module github.com/yair/where-its-at/pkg/integrations

go 1.23.0

toolchain go1.24.5

require github.com/yair/where-its-at/pkg/domain v0.0.0

require golang.org/x/net v0.42.0 // indirect

replace github.com/yair/where-its-at/pkg/domain => ../domain
