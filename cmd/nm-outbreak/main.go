package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"networkmemories/nm-outbreak-go/internal/capcom"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"networkmemories/nm-outbreak-go/internal/config"
	"networkmemories/nm-outbreak-go/internal/core"
	"networkmemories/nm-outbreak-go/internal/httpapi"
	"networkmemories/nm-outbreak-go/internal/identity"
	"networkmemories/nm-outbreak-go/internal/ingest"
	"networkmemories/nm-outbreak-go/internal/kddi"
	"networkmemories/nm-outbreak-go/internal/modules/file1"
	"networkmemories/nm-outbreak-go/internal/modules/file2"
	"networkmemories/nm-outbreak-go/internal/nativeweb"
	"networkmemories/nm-outbreak-go/internal/reports"
	"networkmemories/nm-outbreak-go/internal/results"
	"networkmemories/nm-outbreak-go/internal/snap"
	"networkmemories/nm-outbreak-go/internal/stun"
	"networkmemories/nm-outbreak-go/internal/world"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	// Offline operator utility for the standalone authority. Passwords are never CLI arguments.
	if len(os.Args) > 1 {
		if len(os.Args) != 3 || (os.Args[1] != "account-create" && os.Args[1] != "account-enable-snap") {
			log.Fatal("usage: nm-outbreak account-create|account-enable-snap USERNAME < password-file")
		}
		if cfg.IdentityMode != "standalone" {
			log.Fatal("local account creation requires standalone mode")
		}
		store, err := identity.OpenLocal(cfg.AccountFile)
		if err != nil {
			log.Fatal(err)
		}
		raw, err := io.ReadAll(io.LimitReader(os.Stdin, 131))
		if err != nil {
			log.Fatal("password input unavailable")
		}
		password := strings.TrimSuffix(strings.TrimSuffix(string(raw), "\n"), "\r")
		if len(raw) > 130 {
			log.Fatal("password input too long")
		}
		var account *identity.Account
		if os.Args[1] == "account-create" {
			account, err = store.Register(os.Args[2], password)
		} else {
			account, err = store.PrepareSNAP(os.Args[2], password)
		}
		if err != nil {
			log.Fatal(err)
		}
		if err := json.NewEncoder(os.Stdout).Encode(account); err != nil {
			log.Fatal("account output unavailable")
		}
		return
	}
	if cfg.IdentityMode != "standalone" && cfg.IdentityMode != "nm" {
		log.Fatal("identity mode must be standalone or nm")
	}
	authority, err := identity.Open(identity.Options{Mode: cfg.IdentityMode, AccountFile: cfg.AccountFile, URL: cfg.IdentityURL, KeyFile: cfg.IdentityKeyFile})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("[nm-outbreak-go] configured identity mode: %s (native authentication integration pending)\n", cfg.IdentityMode)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// 1. Mandatory immutable NM STUN link (startup + periodic healthcheck + relay fallback).
	stunLink := stun.NewLink(cfg.StunAddr, cfg.StunCheckInterval, cfg.StunTimeout)
	stunLink.Start(ctx)

	// 2. Unified in-memory WorldRegistry (shared directly by SNAP, KDDI, and HTTP state API).
	registry := world.NewRegistry()
	file1Module := file1.Definition()
	if path := os.Getenv("NM_OUTBREAK_FILE1_JP_CATALOGUE"); path != "" {
		f, err := os.Open(path)
		if err != nil {
			log.Fatal(err)
		}
		raw, err := io.ReadAll(io.LimitReader(f, file1.JPCatalogueSize+1))
		f.Close()
		if err != nil {
			log.Fatal(err)
		}
		if err := file1.ConfigureJPCatalogue(&file1Module, raw); err != nil {
			log.Fatal(err)
		}
	}
	file2Module := file2.Definition()
	if path := os.Getenv("NM_OUTBREAK_FILE2_JP_CATALOGUE"); path != "" {
		f, err := os.Open(path)
		if err != nil {
			log.Fatal(err)
		}
		raw, err := io.ReadAll(io.LimitReader(f, file2.JPCatalogueSize+1))
		f.Close()
		if err != nil {
			log.Fatal(err)
		}
		if err := file2.ConfigureJPCatalogue(&file2Module, raw); err != nil {
			log.Fatal(err)
		}
	}
	engine := core.New(registry, authority, file1Module, file2Module)
	ledger, err := results.Open(filepath.Join(filepath.Dir(cfg.AccountFile), "results.json"))
	if err != nil {
		log.Fatal("canonical result store unavailable")
	}
	if err = engine.ConfigureResults(ledger); err != nil {
		log.Fatal(err)
	}
	if err := engine.ConfigureSNAP(core.SNAPConfig{AdvertiseHost: cfg.AdvertiseHost, BootstrapPort: uint16(cfg.SnapAdvertiseBootstrapPort), GamePorts: map[world.Game]uint16{world.GameFile1: uint16(cfg.SnapFile1Port), world.GameFile2: uint16(cfg.SnapFile2Port)}}); err != nil {
		log.Printf("SNAP bootstrap unavailable: configure NM_OUTBREAK_ADVERTISE_HOST as the server IPv4 address")
	}

	nativeSources := map[string]func() stun.NativeStatus{}
	// 3. SNAP UDP dorsal (Bootstrap 9090, File #1 9091, File #2 9092).
	for _, srv := range []*snap.Server{
		snap.NewTransport("bootstrap", "", cfg.BindHost, cfg.SnapBootstrapPort, engine),
		snap.NewTransport("file1", world.GameFile1, cfg.BindHost, cfg.SnapFile1Port, engine),
		snap.NewTransport("file2", world.GameFile2, cfg.BindHost, cfg.SnapFile2Port, engine),
	} {
		srv.WithNativeControl(cfg.StunControl).WithNativeBackupControl(cfg.StunBackupControl)
		nativeSources[srv.Role()] = srv.NativeStatus
		if err := srv.Start(ctx); err != nil {
			log.Fatalf("failed to start SNAP server: %v", err)
		}
	}

	reportArchive := &ingest.Sink{Engine: engine, ArchiveSink: &reports.FileSink{Path: filepath.Join(filepath.Dir(cfg.AccountFile), "native-reports.jsonl")}}
	// 4. KDDI TCP dorsal (File #1 8300/8690, File #2 8200/8590).
	for _, srv := range []*kddi.Server{
		kddi.NewTransport("file1-lobby", world.GameFile1, cfg.BindHost, cfg.KddiFile1LobbyPort, engine).WithGameAddress(cfg.AdvertiseHost, cfg.KddiFile1GamePort),
		kddi.NewTransport("file1-game", world.GameFile1, cfg.BindHost, cfg.KddiFile1GamePort, engine),
		kddi.NewTransport("file2-lobby", world.GameFile2, cfg.BindHost, cfg.KddiFile2LobbyPort, engine).WithGameAddress(cfg.AdvertiseHost, cfg.KddiFile2GamePort),
		kddi.NewTransport("file2-game", world.GameFile2, cfg.BindHost, cfg.KddiFile2GamePort, engine),
	} {
		srv.WithReports(reportArchive)
		if err := srv.Start(ctx); err != nil {
			log.Fatalf("failed to start KDDI server: %v", err)
		}
	}

	// Capcom APP is another native transport in this process, using module data.
	app, err := capcom.NewServer(net.JoinHostPort(cfg.BindHost, strconv.Itoa(cfg.CapcomAppPort)), cfg.AdvertiseHost, reportArchive, func(game world.Game, key string) uint16 {
		if key != "area" {
			return 0
		}
		return engine.AreaPopulation(game)
	}, file1.AppDefinition(), file2.AppDefinition())
	if err != nil {
		log.Printf("Capcom APP unavailable: configure an advertised IPv4")
	} else if err = app.Start(ctx); err != nil {
		log.Fatalf("failed to start Capcom APP: %v", err)
	}

	// 5. Unified HTTP presence/state & health API.
	api := httpapi.NewServer(cfg.StateHTTPAddr, registry, stunLink)
	api.SetNativeTransports(nativeSources)
	api.EnablePresence(engine, cfg.IdentityMode == "nm")
	if err := api.Start(); err != nil {
		log.Fatalf("failed to start state API: %v", err)
	}

	web := nativeweb.NewServer(cfg.NativeWebAddr, engine, authority).WithLobbyPorts(cfg.KddiFile1LobbyPort, cfg.KddiFile2LobbyPort)
	web.WithNMRegistration(os.Getenv("NM_OUTBREAK_KDDI_REGISTRATION") == "1")
	if err := web.Start(); err != nil {
		log.Fatalf("failed to start native web: %v", err)
	}
	go engine.RunReaper(ctx.Done(), 30*time.Second, func(ids []string) {
		log.Printf("[nm-outbreak-go] released %d silent session(s)", len(ids))
	})
	log.Printf("[nm-outbreak-go] unified File #1 + File #2 dorsal started")
	<-ctx.Done()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	_ = api.Shutdown(shutdownCtx)
	_ = web.Shutdown(shutdownCtx)
	log.Printf("[nm-outbreak-go] clean shutdown complete")
}
