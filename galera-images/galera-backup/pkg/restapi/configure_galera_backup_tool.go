// This file is safe to edit. Once it exists it will not be overwritten

package restapi

import (
	"crypto/tls"
	"fmt"
	"galera-backup/pkg/galera-handlers"
	"galera-backup/pkg/restapi/operations"
	"galera-backup/pkg/version"
	"github.com/go-openapi/errors"
	"github.com/go-openapi/runtime"
	"github.com/go-openapi/swag"
	"net/http"
	"os"
	runtime2 "runtime"
)

//go:generate swagger generate server --target ../../pkg --name GaleraBackupTool --spec ../../swagger/swagger.yaml

func configureFlags(api *operations.GaleraBackupToolAPI) {
	type Help struct {
		ShowHelp func() `short:"v" long:"version" description:"Show version"`
	}

	var help = Help{
		ShowHelp: func() {
			fmt.Println("galera-backup Version:", version.Version)
			fmt.Println("Git SHA:", version.GitSHA)
			fmt.Println("Build Date:", version.Date)
			fmt.Println("Go Version:", runtime2.Version())
			fmt.Printf("Go OS/Arch: %s/%s\n", runtime2.GOOS, runtime2.GOARCH)
			os.Exit(0)
		},
	}

	api.CommandLineOptionsGroups = []swag.CommandLineOptionsGroup{
		swag.CommandLineOptionsGroup{
			ShortDescription: "Additional Options",
			Options: &help,
		},
	}
}

func configureAPI(api *operations.GaleraBackupToolAPI) http.Handler {
	// configure the api here
	api.ServeError = errors.ServeError

	// Set your custom logger if needed. Default one is log.Printf
	// Expected interface func(string, ...interface{})
	//
	// Example:
	// api.Logger = log.Printf

	api.JSONConsumer = runtime.JSONConsumer()
	api.JSONProducer = runtime.JSONProducer()

	api.AddS3BackupHandler = operations.AddS3BackupHandlerFunc(galera_handlers.S3Backup())
	api.GetS3BackupHandler = operations.GetS3BackupHandlerFunc(galera_handlers.S3Restore())
	api.GetStateByNameHandler = operations.GetStateByNameHandlerFunc(galera_handlers.State())
	api.GetProbeHandler = operations.GetProbeHandlerFunc(galera_handlers.Probe())

	api.PreServerShutdown = func() {}
	api.ServerShutdown = func() {}

	return setupGlobalMiddleware(api.Serve(setupMiddlewares))
}

// The TLS configuration before HTTPS server starts.
func configureTLS(tlsConfig *tls.Config) {
	// Make all necessary changes to the TLS configuration here.
}

// As soon as server is initialized but not run yet, this function will be called.
// If you need to modify a config, store server instance to stop it individually later, this is the place.
// This function can be called multiple times, depending on the number of serving schemes.
// scheme value will be set accordingly: "http", "https" or "unix"
func configureServer(s *http.Server, scheme, addr string) {
}

// The middleware configuration is for the handler executors. These do not apply to the swagger.json document.
// The middleware executes after routing but before authentication, binding and validation
func setupMiddlewares(handler http.Handler) http.Handler {
	return handler
}

// The middleware configuration happens before anything, this middleware also applies to serving the swagger.json document.
// So this is a good place to plug in a panic handling middleware, logging and metrics
func setupGlobalMiddleware(handler http.Handler) http.Handler {
	return handler
}
