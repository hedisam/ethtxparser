package main

import (
	"context"
	"errors"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"

	restapi "github.com/hedisam/ethtxparser/api/rest"
	"github.com/hedisam/ethtxparser/internal/custompromauto"
	"github.com/hedisam/ethtxparser/internal/eth"
	"github.com/hedisam/ethtxparser/internal/index"
	"github.com/hedisam/ethtxparser/internal/store/memdb"
)

type Options struct {
	ServerAddr             string
	NodeAddr               string
	PollInterval           time.Duration
	ReorgConfirmationDepth uint
	Verbose                bool
}

func main() {
	var opts Options
	flag.StringVar(&opts.ServerAddr, "server-addr", "localhost:8080", "Server addr to serve the http server on")
	flag.StringVar(&opts.NodeAddr, "node-addr", "https://ethereum-rpc.publicnode.com", "The Ethereum node to connect to")
	flag.DurationVar(&opts.PollInterval, "poll-interval", time.Second*10, "ETH node polling interval. Recommend no less than 6 seconds")
	flag.UintVar(&opts.ReorgConfirmationDepth, "reorg-confirmation-depth", 3, "Number of blocks to check for reorganisation to mark a block confirmed. Cannot be less than 1")
	flag.BoolVar(&opts.Verbose, "v", false, "Verbose output")
	flag.Parse()

	logger := logrus.New()
	ensureValidOpts(logger, opts)

	if opts.Verbose {
		logger.SetLevel(logrus.DebugLevel)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	txStore := memdb.NewTxStore()
	subscriptionStore := memdb.NewSubscriptionStore()

	httpClient := &http.Client{Timeout: time.Second * 10}
	ethClient := eth.New(logger, httpClient, opts.NodeAddr)
	blocksStream := ethClient.Stream(ctx, opts.PollInterval)
	confirmedBlocksStream := eth.ReorgFilter(ctx, logger, blocksStream, opts.ReorgConfirmationDepth)

	idx := index.New(logger, txStore, subscriptionStore)
	go idx.Start(ctx, confirmedBlocksStream)

	restServer := restapi.NewServer(logger, txStore, subscriptionStore)
	mux := http.NewServeMux()
	restapi.RegisterFunc(logger, mux, http.MethodGet, "/api/v1/blocks/current", restServer.GetCurrentBlock)
	restapi.RegisterFunc(logger, mux, http.MethodGet, "/api/v1/transactions/{address}", restServer.ListTransactions)
	restapi.RegisterFunc(logger, mux, http.MethodPut, "/api/v1/subscriptions/{address}", restServer.Subscribe)
	restapi.RegisterFunc(logger, mux, http.MethodGet, "/api/v1/subscriptions/", restServer.ListSubscriptions)

	// use a custom prom registry to avoid recording the default http handler metrics
	mux.Handle("/metrics", promhttp.HandlerFor(custompromauto.Registry(), promhttp.HandlerOpts{}))

	mustListenAndServe(ctx, logger, opts.ServerAddr, mux)
}

func mustListenAndServe(ctx context.Context, logger *logrus.Logger, addr string, handler http.Handler) {
	srv := &http.Server{
		Addr:    addr,
		Handler: handler,
	}

	go func() {
		logger.WithField("addr", addr).Info("Serving server...")
		err := srv.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.WithError(err).Fatal("Server failed with error")
		}
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	logger.Info("Shutting down server...")
	err := srv.Shutdown(shutdownCtx)
	if err != nil && !errors.Is(err, context.DeadlineExceeded) {
		logger.WithError(err).Error("Failed to shutdown server gracefully")
	}
}

func ensureValidOpts(logger *logrus.Logger, opts Options) {
	if opts.ServerAddr == "" {
		logger.Error("--server-addr is required")
		flag.Usage()
		os.Exit(1)
	}
	if opts.NodeAddr == "" {
		logger.Error("--node-addr is required")
		flag.Usage()
		os.Exit(1)
	}
	if opts.PollInterval < time.Second*3 {
		logger.Error("--poll-interval is too small, it cannot be less than 3 seconds")
		flag.Usage()
		os.Exit(1)
	}
	if opts.ReorgConfirmationDepth < 1 {
		logger.Error("--reorg-confirmation-depth is too small, it cannot be less than 1")
		flag.Usage()
		os.Exit(1)
	}
}
