package main

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gnolang/gno/contribs/gnoweb/handlers"
	"github.com/gnolang/gno/contribs/gnoweb/services"
	"github.com/gnolang/gno/contribs/gnoweb/session"
	"github.com/gnolang/gno/gno.land/pkg/log"
	"go.uber.org/zap/zapcore"
)

func main() {
	zapLogger := log.NewZapConsoleLogger(os.Stdout, zapcore.DebugLevel)
	logger := log.ZapLoggerToSlog(zapLogger)
	logger.Info("logger initilized")

	// s, err := db.NewCountStore(os.Getenv("TABLE_NAME"), os.Getenv("AWS_REGION"))
	// if err != nil {
	// 	log.Error("failed to create store", slog.Any("error", err))
	// 	os.Exit(1)
	// }

	cs := services.NewCount(logger)
	h := handlers.New(logger, cs)

	// var secureFlag = true
	// if os.Getenv("SECURE_FLAG") == "false" {
	// 	secureFlag = false
	// }

	// Add session middleware.
	sh := session.NewMiddleware(h, session.WithSecure(false))

	server := &http.Server{
		Addr:         "127.0.0.1:9000",
		Handler:      sh,
		ReadTimeout:  time.Second * 10,
		WriteTimeout: time.Second * 10,
	}

	fmt.Printf("Listening on %v\n", server.Addr)
	server.ListenAndServe()
}
