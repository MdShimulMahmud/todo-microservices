module technonext/todo-app/api-gateway

go 1.24.0

toolchain go1.24.9

require (
	github.com/gorilla/handlers v1.5.1
	github.com/gorilla/mux v1.8.0
	github.com/gorilla/schema v1.2.0
	github.com/technonext/todo-app/proto v0.0.0
	google.golang.org/grpc v1.76.0
)

require (
	github.com/felixge/httpsnoop v1.0.1 // indirect
	golang.org/x/net v0.42.0 // indirect
	golang.org/x/sys v0.34.0 // indirect
	golang.org/x/text v0.27.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250804133106-a7a43d27e69b // indirect
	google.golang.org/protobuf v1.36.10 // indirect
)

replace github.com/technonext/todo-app/proto => ../proto
