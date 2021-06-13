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
	"qbot_webserver/src/handlers/spinneritems"
	"qbot_webserver/src/handlers/tests"
	handlers2 "qbot_webserver/src/helpers"

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
			spinneritems.HandleSubjects(w, r, s.logger, driver, "subjects")
		},
	)
	s.mux.HandleFunc("/faculties",
		func(w http.ResponseWriter, r *http.Request) {
			spinneritems.HandleFaculties(w, r, s.logger, driver, "faculties")
		},
	)
	s.mux.HandleFunc("/specializations",
		func(w http.ResponseWriter, r *http.Request) {
			spinneritems.HandleSpecializations(w, r, s.logger, driver, "specializations")
		},
	)
	s.mux.HandleFunc("/groups",
		func(w http.ResponseWriter, r *http.Request) {
			spinneritems.HandleGroups(w, r, s.logger, driver, "groups")
		},
	)
	s.mux.HandleFunc("/tests/answers",
		func(w http.ResponseWriter, r *http.Request) {
			tests.HandleTestAnswers(w, r, s.logger, driver, "testAnswers")
		},
	)
	s.mux.HandleFunc("/tests/errors",
		func(w http.ResponseWriter, r *http.Request) {
			tests.HandleTestErrors(w, r, s.logger, driver, "testErrors")
		},
	)
	s.mux.HandleFunc("/tests/feedback",
		func(w http.ResponseWriter, r *http.Request) {
			tests.HandleTestFeedback(w, r, s.logger, driver, "testFeedback")
		},
	)
	s.mux.HandleFunc("/tests/grade",
		func(w http.ResponseWriter, r *http.Request) {
			tests.HandleTestGrade(w, r, s.logger, driver, "testGrade")
		},
	)
	s.mux.HandleFunc("/tests/notifications",
		func(w http.ResponseWriter, r *http.Request) {
			tests.HandleTestNotifications(w, r, s.logger, driver, "testNotifications")
		},
	)
	s.mux.HandleFunc("/tests",
		func(w http.ResponseWriter, r *http.Request) {
			tests.HandleTests(w, r, s.logger, driver, "tests")
		},
	)
	s.mux.HandleFunc("/objectives",
		func(w http.ResponseWriter, r *http.Request) {
			handlers.HandleObjectives(w, r, s.logger, driver, "objective")
		},
	)

	return s
}

func main() {
	logger := log.New(os.Stdout, "", 0)
	ip := "bolt://3.125.35.149"

	driver, err := handlers2.ConnectNeo4j(ip, "neo4j", "mariairene")
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
