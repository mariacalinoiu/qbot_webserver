package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/neo4j/neo4j-go-driver/neo4j"

	"qbot_webserver/src/datasources"
	"qbot_webserver/src/handlers"
)

type server struct {
	mux    *http.ServeMux
	logger *log.Logger
}

type option func(*server)

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.log("Method: %s, Path: %s", r.Method, r.URL.Path)
	s.mux.ServeHTTP(w, r)
}

func (s *server) log(format string, v ...interface{}) {
	s.logger.Printf(format+"\n", v...)
}

func logWith(logger *log.Logger) option {
	return func(s *server) {
		s.logger = logger
	}
}

func setup(logger *log.Logger, driver neo4j.Driver) *http.Server {
	server := newServer(driver, logWith(logger))
	return &http.Server{
		Addr:         ":8081",
		Handler:      server,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  600 * time.Second,
	}
}

func newServer(driver neo4j.Driver, options ...option) *server {
	s := &server{logger: log.New(ioutil.Discard, "", 0)}

	for _, o := range options {
		o(s)
	}

	s.mux = http.NewServeMux()

	s.mux.HandleFunc("/subjects",
		func(w http.ResponseWriter, r *http.Request) {
			handlers.HandleSubjects(w, r, s.logger, driver, "/subjects")
		},
	)

	return s
}

func main() {
	logger := log.New(os.Stdout, "", 0)
	ip := "neo4j://3.125.35.149"
	//database := "neo4j"
	//ip := "localhost"

	driver, err := datasources.ConnectNeo4j(ip, "neo4j", "mariairene")
	if err != nil {
		logger.Println(fmt.Sprintf("error connecting to Neo4j: %s", err))
	} else {
		logger.Println("connected to Neo4j")
	}

	hs := setup(logger, driver)

	logger.Printf("Listening on http://localhost%s\n", hs.Addr)
	go func() {
		if err := hs.ListenAndServe(); err != nil {
			logger.Println(err)
		}
	}()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	<-signals

	logger.Println("Shutting down webserver.")
	os.Exit(0)
}
