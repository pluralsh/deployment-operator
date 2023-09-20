package provisioner

import (
	"context"
	"encoding/json"
	"time"

	"google.golang.org/grpc"

	"github.com/pluralsh/deployment/common/log"
)

func apiLogger(ctx context.Context, api string,
	req, resp interface{},
	grpcConn *grpc.ClientConn,
	apiCall grpc.UnaryInvoker,
	opts ...grpc.CallOption) error {

	if jsonReq, err := json.MarshalIndent(req, "", " "); err != nil {
		log.Logger.Infow("Request", "api", api, "req", string(jsonReq))
	}

	start := time.Now()
	err := apiCall(ctx, api, req, resp, grpcConn, opts...)
	end := time.Now()

	if jsonRes, err := json.MarshalIndent(resp, "", " "); err != nil {
		log.Logger.Infow("Response", "api", api, "elapsed", end.Sub(start), "resp", jsonRes)
	}

	return err
}
