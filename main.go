package main

import (
	"context"
	"fmt"
	golog "log"
	"os"
	"os/signal"
	"time"

	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"github.com/hashicorp/go-multierror"
	flag "github.com/spf13/pflag"
	"go.uber.org/zap"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type flags struct {
	idendityID     string
	subscriptionID string
}

func (f *flags) validate() error {
	var error *multierror.Error
	if f.idendityID == "" {
		error = multierror.Append(error, fmt.Errorf("identityID cannot be empty"))
	}
	if f.subscriptionID == "" {
		error = multierror.Append(error, fmt.Errorf("subscriptionID cannot be empty"))
	}
	return error.ErrorOrNil()
}

func main() {
	var fl flags
	flag.StringVarP(&fl.idendityID, "identity", "i", "", "ID of identity used to authenticate to cluster resources and snapshot storage.")
	flag.StringVarP(&fl.subscriptionID, "subscription", "s", "", "SubscriptionID of cluster resources and snapshot storage.")

	flag.Parse()

	log, err := newLogger()
	if err != nil {
		golog.Fatalln(err)
	}

	if err := fl.validate(); err != nil {
		log.Error(err, "failed to validate flags")
		os.Exit(1)
	}

	int := make(chan os.Signal, 1)
	signal.Notify(int, os.Interrupt)
	ctx, cancel := context.WithCancel(context.Background())
	go func() { <-int; cancel() }()

	if err := run(ctx, cancel, log, &fl); err != nil {
		log.Error(err, "error running serving, shutting down")
		os.Exit(1)
	}
}

func run(ctx context.Context, cancel context.CancelFunc, log logr.Logger, fl *flags) error {
	auth, err := auth.NewAuthorizerFromEnvironment()
	// msiConfig := NewMSIConfig(fl.idendityID)

	// if token, err := msiConfig.ServicePrincipalToken(); err != nil {
	// 	return fmt.Errorf("failed to acquire auth token, likely auth configuration problem. %w", err)
	// } else {
	// 	if err := token.EnsureFreshWithContext(ctx); err != nil {
	// 		return fmt.Errorf("failed to ensure fresh token, likely identity configuration issue. %w", err)
	// 	}
	// }

	// auth, err := msiConfig.Authorizer()
	if err != nil {
		return fmt.Errorf("failed to get authorizer. %w", err)
	}

	// Fetch kubeconfig
	kubeconfig, err := ctrl.GetConfig()
	if err != nil {
		return fmt.Errorf("failed to get kubeconfig. %w", err)
	}

	kubeclient, err := client.New(kubeconfig, client.Options{})
	if err != nil {
		return fmt.Errorf("failed to create kubeclient. %w", err)
	}

	bakery := &Bakery{
		Kube:  kubeclient,
		Auth:  auth,
		Cloud: NewClients(fl.subscriptionID, auth),
	}

	server := newHTTPServer("0.0.0.0:8080", bakery, log)

	done := make(chan struct{})
	go func() {
		if err := server.Start(); err != nil {
			log.Error(err, "failure running server")
		}
		close(done)
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	<-c

	ctx, cancel = context.WithTimeout(context.Background(), time.Second*15)
	defer cancel()
	server.Shutdown(ctx)

	return nil
}

func newLogger() (logr.Logger, error) {
	zapLog, err := zap.NewDevelopment()
	if err != nil {
		return zapr.NewLogger(nil), fmt.Errorf("failed to create logger. %w", err)
	}
	return zapr.NewLogger(zapLog), nil
}
