// Copyright 2026 ptrvsrg.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/container-storage-interface/spec/lib/go/csi"
	grpcprom "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/ptrvsrg/csi-driver-ipfs/pkg/driver"
	"github.com/ptrvsrg/csi-driver-ipfs/pkg/driver/store"
	"github.com/ptrvsrg/csi-driver-ipfs/pkg/grpc/middleware"
	"github.com/ptrvsrg/csi-driver-ipfs/pkg/ipfs"
	"github.com/ptrvsrg/csi-driver-ipfs/pkg/kubernetes"
	"github.com/ptrvsrg/csi-driver-ipfs/pkg/logging"
	"github.com/ptrvsrg/csi-driver-ipfs/pkg/utils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"k8s.io/client-go/tools/clientcmd"
)

// Default Kubernetes client QPS and Burst (same as client-go defaults when not set).
const (
	defaultKubeQPS   = 50.0
	defaultKubeBurst = 100
)

var (
	endpoint    = flag.String("endpoint", "unix:///csi/csi.sock", "CSI endpoint")
	nodeID      = flag.String("nodeid", "", "Node ID")
	ipfsAPI     = flag.String("ipfs-api", "/ip4/0.0.0.0/tcp/5001", "IPFS API endpoint")
	ipfsMFSRoot = flag.String("ipfs-mfs-root", "/csi-volumes", "IPFS MFS root path")
	driverName  = flag.String("drivername", driver.DefaultDriverName, "CSI driver name")
	mountDir    = flag.String("mount-dir", "/var/lib/csi-ipfs", "Base mount directory for volumes")
	logLevel    = flag.String("log-level", "info", "Log level: trace, debug, info, warn, error")
	logFormat   = flag.String("log-format", "text", "Log format: json, text")
	grpcReflect = flag.Bool("grpc-reflection", false, "Enable gRPC reflection (disable in production)")
	pinFailure  = flag.String("pin-failure-policy", "strict", "CID pin failure policy: strict, best-effort")
	version     = flag.Bool("version", false, "Print version and exit")
	kubeConfig  = flag.String(
		kubernetes.KubeConfigFlagName,
		os.Getenv(clientcmd.RecommendedConfigPathEnvVar),
		"Path to kubeconfig (empty: use KUBECONFIG env or in-cluster config)",
	)
	kubeQPS   = flag.Float64("kube-qps", defaultKubeQPS, "Kubernetes API client QPS (queries per second) rate limit")
	kubeBurst = flag.Int("kube-burst", defaultKubeBurst, "Kubernetes API client burst rate limit")
)

var (
	driverVersion = "dev"
	gitCommit     = "unknown"
	buildDate     = "unknown"
)

var (
	noopCreate = func(any) {}
	noopUpdate = func(any, any) {}
	noopRemove = func(any) {}
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	defer cancel()

	flag.Parse()

	if *version {
		fmt.Printf("csi-driver-ipfs %s (commit: %s, built: %s)\n", driverVersion, gitCommit, buildDate)
		os.Exit(0)
	}

	logging.SetupLogger(
		utils.FromPtr(logLevel),
		utils.FromPtr(logFormat),
	)
	setupLogger := logging.NewComponentLogger("setup")

	if *nodeID == "" {
		setupLogger.Warn().Msg("node ID not specified, using hostname")
		hostname, err := os.Hostname()
		if err != nil {
			setupLogger.Fatal().Err(err).Msg("get hostname")
		}
		*nodeID = hostname
	}

	setupLogger.Info().
		Str("version", driverVersion).
		Str("build-date", buildDate).
		Str("git-commit", gitCommit).
		Str("node-id", utils.FromPtr(nodeID)).
		Str("driver-name", utils.FromPtr(driverName)).
		Str("ipfs-api", utils.FromPtr(ipfsAPI)).
		Str("endpoint", utils.FromPtr(endpoint)).
		Msg("starting IPFS CSI driver")

	setupLogger.Debug().Str("ipfs-api", utils.FromPtr(ipfsAPI)).Msg("create IPFS client")
	ipfsClient, err := ipfs.NewClient(
		logging.NewComponentLogger("ipfs-client"),
		utils.FromPtr(ipfsAPI),
	)
	if err != nil {
		setupLogger.Fatal().Err(err).Msg("create IPFS client")
	}

	restConfig, err := kubernetes.GetKubeConfig(
		utils.FromPtr(kubeConfig),
	)
	if err != nil {
		setupLogger.Fatal().Err(err).Msg("get kube config")
	}

	// Apply QPS and Burst from flags (defaults used when flags not set).
	restConfig.QPS = float32(*kubeQPS)
	restConfig.Burst = *kubeBurst

	setupLogger.Debug().Msg("create kubernetes core client")
	kubeClient, err := kubernetes.NewKubeClient(restConfig)
	if err != nil {
		setupLogger.Fatal().Err(err).Msg("create kubernetes core client")
	}

	setupLogger.Debug().Msg("create kubernetes snapshot client")
	snapshotClient, err := kubernetes.NewSnapshotClient(restConfig)
	if err != nil {
		setupLogger.Warn().Err(err).Msg("kubernetes snapshot client not available")
		snapshotClient = nil
	}
	if snapshotClient != nil && !kubernetes.IsVolumeSnapshotAPIAvailable(restConfig) {
		setupLogger.Warn().Msg(
			"snapshot.storage.k8s.io/v1 API not registered; snapshot informer disabled " +
				"(install VolumeSnapshot CRDs to enable snapshots)",
		)
		snapshotClient = nil
	}

	setupLogger.Info().Msg("starting Kubernetes informers (PersistentVolume, VolumeSnapshotContent when API available)")
	im := kubernetes.NewInformerManager(kubeClient, snapshotClient)

	if err := im.AddPVListener(noopCreate, noopUpdate, noopRemove); err != nil {
		setupLogger.Fatal().Err(err).Msg("add PersistentVolume listener")
	}
	if err := im.AddVSCListener(noopCreate, noopUpdate, noopRemove); err != nil {
		setupLogger.Fatal().Err(err).Msg("add VolumeSnapshotContent listener")
	}

	if err := im.Start(ctx, setupLogger); err != nil {
		setupLogger.Fatal().Err(err).Msg("start Kubernetes informers")
	}

	volumeStore := store.NewVolumeStoreFromPVLister(im.GetPVLister())
	snapshotStore := store.NewSnapshotStoreFromVSCLister(im.GetVSCLister())
	if snapshotStore == nil {
		setupLogger.Debug().Msg("snapshot store not available")
	}

	cfg := &driver.Config{
		DriverName:       *driverName,
		NodeID:           *nodeID,
		Version:          driverVersion,
		MountDir:         *mountDir,
		MFSRoot:          *ipfsMFSRoot,
		PinFailurePolicy: *pinFailure,
	}
	csiDriver, err := driver.NewDriver(
		logging.NewComponentLogger("csi-driver"),
		cfg,
		ipfsClient,
		volumeStore,
		snapshotStore,
	)
	if err != nil {
		setupLogger.Fatal().Err(err).Msg("create CSI driver")
	}

	setupLogger.Debug().Msg("create gRPC server")
	scheme, addr, err := parseListenAddress(*endpoint)
	if err != nil {
		setupLogger.Fatal().Err(err).Msg("parse endpoint")
	}
	listener, err := net.Listen(scheme, addr)
	if err != nil {
		setupLogger.Fatal().Err(err).Msg("create listener")
	}

	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(
			middleware.LoggingUnaryServerInterceptor(logging.NewComponentLogger("grpc-server")),
		),
	)
	grpcprom.Register(grpcServer)
	if *grpcReflect {
		reflection.Register(grpcServer)
		setupLogger.Info().Msg("gRPC reflection enabled")
	}
	csi.RegisterIdentityServer(grpcServer, csiDriver)
	csi.RegisterControllerServer(grpcServer, csiDriver)
	csi.RegisterNodeServer(grpcServer, csiDriver)

	go func() {
		setupLogger.Info().Str("endpoint", fmt.Sprintf("%s://%s", scheme, addr)).Msg("gRPC server listening")
		if err := grpcServer.Serve(listener); err != nil {
			setupLogger.Fatal().Err(err).Msg("gRPC server")
		}
	}()

	defer func() {
		setupLogger.Info().Msg("stopping gRPC server")
		grpcServer.GracefulStop()
	}()

	<-ctx.Done()
}

// parseListenAddress parses endpoint as a URL and returns the listen scheme and address for net.Listen.
//
// endpoint must use scheme "unix" (path in URL path) or "tcp" (host:port in host). For unix,
// an existing socket file at addr is removed when present so Listen can recreate it.
//
// It returns scheme and addr suitable for net.Listen(scheme, addr), or a non-nil err if the URL
// is invalid or the scheme is unsupported.
func parseListenAddress(endpoint string) (scheme, addr string, err error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", "", fmt.Errorf("parse endpoint: %w", err)
	}

	switch u.Scheme {
	case "unix":
		addr = filepath.Clean(u.Path)
		fi, statErr := os.Lstat(addr)
		if statErr == nil && (fi.Mode()&os.ModeSocket) == 0 {
			return "", "", fmt.Errorf("refusing to remove non-socket path: %s", addr)
		}

		if removeErr := os.Remove(addr); removeErr != nil && !os.IsNotExist(removeErr) {
			return "", "", fmt.Errorf("remove unix socket: %w", removeErr)
		}

		return u.Scheme, addr, nil
	case "tcp":
		return u.Scheme, u.Host, nil
	default:
		return "", "", fmt.Errorf("unsupported protocol scheme: %s", u.Scheme)
	}
}
