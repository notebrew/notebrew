package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/notebrew/notebrew"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	app, err := notebrew.NewApp(os.Getenv("DATABASE_URL"), os.Getenv("NOTEBREW_DATA"))
	if err != nil {
		log.Fatal(err)
	}
	server := http.Server{
		Addr:    os.Getenv("NOTEBREW_ADDR"),
		Handler: app.Handler(),
	}
	if server.Addr == "" {
		server.Addr = "localhost:7070"
	}
	fmt.Println("Listening on " + server.Addr)
	go server.ListenAndServe()
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_ = server.Shutdown(ctx)
	_ = app.Cleanup()
}
