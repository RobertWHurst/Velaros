module github.com/RobertWHurst/scramjet

go 1.21.4

require (
	github.com/RobertWHurst/navaros v1.0.0
	github.com/coder/websocket v1.8.12
	github.com/grafana/regexp v0.0.0-20240518133315-a468a5bfb3bc
)

require github.com/davecgh/go-spew v1.1.1

require (
	github.com/google/uuid v1.6.0 // indirect
	github.com/klauspost/compress v1.17.2 // indirect
	github.com/nats-io/nats.go v1.37.0 // indirect
	github.com/nats-io/nkeys v0.4.7 // indirect
	github.com/nats-io/nuid v1.0.1 // indirect
	golang.org/x/crypto v0.18.0 // indirect
	golang.org/x/sys v0.16.0 // indirect
)

replace github.com/RobertWHurst/navaros => ../Navaros
