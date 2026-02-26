package maestro

import (
	"context"
	"fmt"
	"time"

	"github.com/openshift/hypershift/cmd/cluster/core"
	maestroapply "github.com/openshift-online/maestro/pkg/client/apply"
)

// DestroyViaMaestro deletes the ManifestWork from Maestro. Returns (true, nil) if
// Maestro destroy was performed, (false, nil) if Maestro opts were not set, or
// (true, err) if Maestro destroy failed.
func DestroyViaMaestro(ctx context.Context, opts *core.DestroyOptions) (done bool, err error) {
	if opts.MaestroServer == "" {
		return false, nil
	}
	if opts.MaestroConsumer == "" {
		return true, fmt.Errorf("--maestro-consumer is required when using --maestro-server")
	}
	grpcServer := opts.MaestroGRPCServer
	if grpcServer == "" {
		grpcServer = "127.0.0.1:8090"
	}
	workName := opts.Name + "-work"
	applyOpts := maestroapply.Options{
		MaestroServer:        opts.MaestroServer,
		GRPCServer:           grpcServer,
		ConsumerName:         opts.MaestroConsumer,
		WorkName:             workName,
		InsecureSkipVerify:   opts.MaestroInsecureTLS,
		ServerHealthTimeout:  20 * time.Second,
	}
	if err := maestroapply.DeleteManifestWork(ctx, applyOpts); err != nil {
		return true, err
	}
	opts.Log.Info("Deleted ManifestWork from Maestro", "work", workName, "consumer", opts.MaestroConsumer)
	return true, nil
}
