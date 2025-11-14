package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/SwissOpenEM/globus"
	"github.com/SwissOpenEM/scicat-globus-proxy/internal/api"
	"github.com/SwissOpenEM/scicat-globus-proxy/internal/config"
	"github.com/SwissOpenEM/scicat-globus-proxy/internal/serviceuser"
	"github.com/SwissOpenEM/scicat-globus-proxy/internal/tasks"
)

func setupLogging(logLevel string) {
	level := slog.LevelDebug
	switch logLevel {
	case "Info":
		level = slog.LevelInfo
	case "Debug":
		level = slog.LevelDebug
	case "Error":
		level = slog.LevelError
	case "Warning":
		level = slog.LevelWarn
	}

	opts := &slog.HandlerOptions{Level: level}
	h := slog.NewTextHandler(os.Stdout, opts)
	slog.SetDefault(slog.New(h))
}

func main() {
	setupLogging("Debug")

	globusClientId := os.Getenv("GLOBUS_CLIENT_ID")
	globusClientSecret := os.Getenv("GLOBUS_CLIENT_SECRET")
	scicatServiceUserUsername := os.Getenv("SCICAT_SERVICE_USER_USERNAME")
	scicatServiceUserPassword := os.Getenv("SCICAT_SERVICE_USER_PASSWORD")

	conf, err := config.ReadConfig()
	if err != nil {
		slog.Error("couldn't read config", "error", err)
		os.Exit(1)
	}

	serviceUser, err := serviceuser.CreateServiceUser(conf.ScicatUrl, scicatServiceUserUsername, scicatServiceUserPassword)
	if err != nil {
		slog.Error("couldn't create service user", "error", err)
		os.Exit(1)
	}

	globusClient, err := globus.AuthCreateServiceClient(context.Background(), globusClientId, globusClientSecret, conf.GlobusScopes)
	if err != nil {
		slog.Error("couldn't create globus client", "error", err)
		os.Exit(1)
	}

	taskPool := tasks.CreateTaskPool(conf.ScicatUrl, globusClient, serviceUser, conf.Task.MaxConcurrency, conf.Task.QueueSize, conf.Task.PollInterval)

	err = tasks.RestoreGlobusTransferJobsFromScicat(conf.ScicatUrl, serviceUser, taskPool)
	if err != nil {
		slog.Error("couldn't resume unfinished jobs", "error", err)
		os.Exit(1)
	}

	serverHandler, err := api.NewServerHandler(globusClient, conf.GlobusScopes, conf.ScicatUrl, serviceUser, conf.FacilityCollectionIDs, conf.FacilitySrcGroupTemplate, conf.FacilityDstGroupTemplate, conf.DstPathTemplate, taskPool)
	if err != nil {
		slog.Error("couldn't create server handler", "error", err)
		os.Exit(1)
	}

	server, err := api.NewServer(&serverHandler, conf.Port, conf.ScicatUrl)
	if err != nil {
		slog.Error("couldn't create server", "error", err)
		os.Exit(1)
	}

	slog.Info("Starting server", "port", conf.Port)
	err = server.ListenAndServe()
	if err != nil {
		slog.Error("server encountered an error", "error", err)
		os.Exit(1)
	}
}
