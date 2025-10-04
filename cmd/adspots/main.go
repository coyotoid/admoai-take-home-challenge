package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	adspots "github.com/coyotoid/admoai-take-home-challenge"
)

func main() {
	db, err := gorm.Open(sqlite.Open("adspots.db"), &gorm.Config{})
	if err != nil {
		log.Fatal(err)
	}
	db.AutoMigrate(&adspots.AdSpot{})

	server := adspots.NewServer(db)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	srv := &http.Server{
		Handler:      server.Mux(),
		Addr:         "localhost:8081",
		BaseContext:  func(net.Listener) context.Context { return ctx },
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			stop()
			log.Fatalln("error while serving:", err)
		}
	}()

	log.Print("listening on localhost:8081")
	<-ctx.Done()
	log.Print("stopping server...")

	timeout, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(timeout); err != nil {
		log.Fatalln("error while shutting down:", err)
	}
}
