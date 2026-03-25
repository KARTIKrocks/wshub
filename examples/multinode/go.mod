module github.com/KARTIKrocks/wshub/examples/multinode

go 1.22

require (
	github.com/KARTIKrocks/wshub v0.0.0
	github.com/KARTIKrocks/wshub/adapter/redis v0.0.0
	github.com/redis/go-redis/v9 v9.7.3
)

require (
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/gorilla/websocket v1.5.3 // indirect
)

replace (
	github.com/KARTIKrocks/wshub => ../..
	github.com/KARTIKrocks/wshub/adapter/redis => ../../adapter/redis
)
