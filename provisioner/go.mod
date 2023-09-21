module github.com/pluralsh/deployment-operator/provisioner

go 1.21

require (
	github.com/pkg/errors v0.9.1
	github.com/pluralsh/deployment-operator/common v0.0.0-00010101000000-000000000000
	google.golang.org/grpc v1.58.1
	google.golang.org/protobuf v1.31.0
)

require (
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.26.0 // indirect
	golang.org/x/net v0.15.0 // indirect
	golang.org/x/sys v0.12.0 // indirect
	golang.org/x/text v0.13.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20230711160842-782d3b101e98 // indirect
)

// Local workspace modules
replace github.com/pluralsh/deployment-operator/common => ./../common
