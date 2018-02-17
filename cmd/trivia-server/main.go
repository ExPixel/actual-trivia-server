package main

import (
	"context"
	"database/sql"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"github.com/expixel/actual-trivia-server/eplog"
	"github.com/expixel/actual-trivia-server/trivia/api/auth"
	"github.com/expixel/actual-trivia-server/trivia/api/profile"
	"github.com/expixel/actual-trivia-server/trivia/postgres/migrations"

	"github.com/expixel/actual-trivia-server/trivia/postgres"
	_ "github.com/lib/pq"
)

func withLogging(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		handler.ServeHTTP(w, r)
		dur := time.Since(start)
		eplog.Debug("http", "%s %s (%s)", strings.ToUpper(r.Method), r.RequestURI, dur.String())
	})
}

var logLevelFlag = flag.String("level", "info", "Sets the log minimum log level. Should be one onf 'debug', 'info', 'warning', 'error'.")

func setLogLevelFromFlag() {
	flg := strings.ToLower(*logLevelFlag)
	switch flg {
	case "debug":
		eplog.SetMinLevel(eplog.LogLevelDebug)
	case "info":
		eplog.SetMinLevel(eplog.LogLevelInfo)
	case "warning":
		eplog.SetMinLevel(eplog.LogLevelWarning)
	case "warn":
		eplog.SetMinLevel(eplog.LogLevelWarning)
	case "error":
		eplog.SetMinLevel(eplog.LogLevelError)
	default:
		eplog.SetMinLevel(eplog.LogLevelInfo)
	}
}

func main() {
	flag.Parse()

	fileLogHandler, err := eplog.NewDefaultFileHandler("trivia-log.log")
	if err != nil {
		log.Fatal("Failed to create file log handler for path: ", "trivia-log.log")
	}
	logHandler := eplog.MergeLogHandlers(
		eplog.NewDefaultStdoutHandler(),
		fileLogHandler,
	)
	eplog.SetHandler(logHandler)
	setLogLevelFromFlag()

	go eplog.Start()

	eplog.Info("app", "starting server...")
	config := loadConfig()

	connStr := createSQLConnectionString(config)
	eplog.Debug("postgres connection string =  `%s`", connStr)
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		eplog.Error("app", "error occurred while opening db connection: %s", err)
		eplog.Stop()
		eplog.WaitForStop()
		os.Exit(1)
		return
	}

	authPepper256, ok := getStringValue(config.Auth.Pepper256)
	if ok {
		auth.SetAESKeyHex(authPepper256)
	}

	mgSuccess := migrations.RunMigrations(db)
	if !mgSuccess {
		eplog.Error("app", "Migrations failed. Exiting.")
		eplog.Stop()
		eplog.WaitForStop()
		os.Exit(1)
		return
	}

	// ## services
	userService := postgres.NewUserService(db)
	tokenService := postgres.NewTokenService(db)
	authService := auth.NewService(userService, tokenService)

	// ## handlers
	authHandler := auth.NewHandler(authService)
	profileHandler := profile.NewHandler(userService, tokenService)
	r := http.NewServeMux()
	r.Handle("/v1/auth/", withLogging(authHandler))
	r.Handle("/v1/profile/", withLogging(profileHandler))

	server := &http.Server{
		Addr:         requireStringValue(config.Server.Addr, "0.0.0.0:8080", "server.addr cannot be empty"),
		WriteTimeout: time.Second * 15,
		ReadTimeout:  time.Second * 15,
		IdleTimeout:  time.Second * 30,
		Handler:      r,
	}

	shutdownTimeout, err := strconv.Atoi(requireStringValue(config.Server.ShutdownTimeout, "15000", "server.shutdownTimeout cannot be empty."))
	if err != nil {
		log.Fatal("server.shutdownTimeout must be a valid number.")
	}

	go func() {
		log.Println("starting server...")
		if err := server.ListenAndServe(); err != nil {
			log.Println(err)
		}
	}()

	sigChan := make(chan os.Signal, 1)

	// catches the interrupt signal (SIGINT / Ctrl+C)
	signal.Notify(sigChan, os.Interrupt)

	// block until SIGINT is caught
	<-sigChan

	// deadline for shutting down
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(shutdownTimeout)*time.Millisecond)
	defer cancel()

	log.Println("shutting down...")
	log.Println("waiting for connections...")
	server.Shutdown(ctx)
	log.Println("shutting down eplog...")
	eplog.Stop()
	eplog.WaitForStop()
	log.Println("shutdown.")

	os.Exit(0)
}
