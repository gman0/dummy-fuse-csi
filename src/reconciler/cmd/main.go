package main

import (
	"context"
	"flag"
	"time"

	"dummy-fuse-reconciler/internal/reconciler"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var (
	fNodeName  = flag.String("node-name", "", "name of this node")
	fCSIDriver = flag.String("csi-name", "", "name of the CSI driver")
	// TODO: restart limit? We need to make sure we don't end up in
	// a loop if node plugin keeps crashing.
)

func main() {
	flag.Parse()

	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}

	c, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	ctx := context.TODO()

	opts := reconciler.Opts{
		NodeName:           *fNodeName,
		CSIDriverName:      *fCSIDriver,
		ReconciliationTime: time.Now(),
	}

	if err := reconciler.Run(ctx, c, &opts); err != nil {
		panic(err.Error())
	}
}
