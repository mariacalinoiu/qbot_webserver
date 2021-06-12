package handlers

import (
	"fmt"
	"log"
	"net/http"
)

const (
	Success = "ok"
)

func SetContentType(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
}

func SetAccessControlHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
	w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, Authorization, X-CSRF-Token")
	w.Header().Set("Access-Control-Expose-Headers", "Authorization")
}

func ConnectionError(database string) error {
	return fmt.Errorf("could not connect to Neo4j database '%s'", database)
}

func WrongMethodError(path string) error {
	return fmt.Errorf("wrong method type for %s route", path)
}

func PrintError(logger *log.Logger, err error, status int) {
	logger.Printf("Error: %s; Status: %d %s", err.Error(), status, http.StatusText(status))
}

func PrintStatus(logger *log.Logger, status int) {
	logger.Printf("Status: %d %s", status, http.StatusText(status))
}

func GetError(path string, err error) error {
	return fmt.Errorf("could not get %s: %s", path, err.Error())
}

func BadParameterError(path string, err error) error {
	return fmt.Errorf("bad parameters for request %s: %s", path, err.Error())
}

func InvalidTokenError(path string, err error) error {
	return fmt.Errorf("token not found for %s: %s", path, err.Error())
}

func MarshalError(path string, err error) error {
	return fmt.Errorf("could not marshal response json for %s: %s", path, err.Error())
}
