package main

import (
	"flag"
	"os"
	"time"

	"github.com/golang/glog"
	"github.com/pliurh/sriov-network-operator/pkg/daemon"
	"github.com/pliurh/sriov-network-operator/pkg/version"
	"github.com/spf13/cobra"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	snclientset "github.com/pliurh/sriov-network-operator/pkg/client/clientset/versioned"
)

var (
	startCmd = &cobra.Command{
		Use:   "start",
		Short: "Starts Machine Config Daemon",
		Long:  "",
		Run:   runStartCmd,
	}

	startOpts struct {
		kubeconfig             string
		nodeName               string
	}
)

func init() {
	rootCmd.AddCommand(startCmd)
	startCmd.PersistentFlags().StringVar(&startOpts.kubeconfig, "kubeconfig", "", "Kubeconfig file to access a remote cluster (testing only)")
	startCmd.PersistentFlags().StringVar(&startOpts.nodeName, "node-name", "", "kubernetes node name daemon is managing.")
}

func runStartCmd(cmd *cobra.Command, args []string) {
	flag.Set("logtostderr", "true")
	flag.Parse()

	// To help debugging, immediately log version
	glog.Infof("Version: %+v", version.Version)

	if startOpts.nodeName == "" {
		name, ok := os.LookupEnv("NODE_NAME")
		if !ok || name == "" {
			glog.Fatalf("node-name is required")
		}
		startOpts.nodeName = name
	}

	// This channel is used to ensure all spawned goroutines exit when we exit.
	stopCh := make(chan struct{})
	defer close(stopCh)

	// This channel is used to signal Run() something failed and to jump ship.
	// It's purely a chan<- in the Daemon struct for goroutines to write to, and
	// a <-chan in Run() for the main thread to listen on.
	exitCh := make(chan error)
	defer close(exitCh)

	// var dn *daemon.Daemon
	// var ctx *common.ControllerContext

	// cb, err := common.NewClientBuilder(startOpts.kubeconfig)
	// if err != nil {
	// 	glog.Fatalf("failed to initialize ClientBuilder: %v", err)
	// }
	// ctx = common.CreateControllerContext(cb, stopCh, componentName)
	// // create the daemon instance. this also initializes kube client items
	// // which need to come from the container and not the chroot.
	// dn, err = daemon.New(
	// 	startOpts.nodeName,
	// 	exitCh,
	// 	stopCh,
	// )
	// if err != nil {
	// 	glog.Fatalf("failed to initialize daemon: %v", err)
	// }


	var config *rest.Config
	var err error
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig != "" {
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	} else {
		// creates the in-cluster config
		config, err = rest.InClusterConfig()
		if err != nil {
			panic(err.Error())
		}
	}
	clientset := snclientset.NewForConfigOrDie(config)
	glog.Info("starting node writer")
	nodeWriter := daemon.NewNodeStateStatusWriter(clientset,startOpts.nodeName)
	go nodeWriter.Run(stopCh)

	for {
		time.Sleep(300)
	}
	glog.Info("Starting SriovNetworkConfigDaemon")
	defer glog.Info("Shutting down SriovConfigDaemon")



	// err = dn.Run(stopCh, exitCh)
	// if err != nil {
	// 	glog.Fatalf("failed to run: %v", err)
	// }
}
