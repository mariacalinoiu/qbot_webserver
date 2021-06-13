package tests

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/neo4j/neo4j-go-driver/neo4j"

	"qbot_webserver/src/datasources"
	helpers "qbot_webserver/src/helpers"
	"qbot_webserver/src/repositories"
)

func HandleTestNotifications(w http.ResponseWriter, r *http.Request, logger *log.Logger, driver neo4j.Driver, path string) {
	var response []byte
	var status int
	var err error

	helpers.SetContentType(w)
	session, err := helpers.GetNeo4jSession(driver)
	if err != nil {
		helpers.PrintError(logger, err, status)
		http.Error(w, err.Error(), status)

		return
	}
	defer session.Close()

	switch r.Method {
	case http.MethodOptions:
		helpers.SetAccessControlHeaders(w)
	case http.MethodGet:
		response, status, err = getNotifications(r, session, path)
	default:
		status = http.StatusBadRequest
		err = helpers.WrongMethodError(path)
	}

	if err != nil {
		helpers.PrintError(logger, err, status)
		http.Error(w, err.Error(), status)

		return
	}

	if response == nil {
		response, _ = json.Marshal(repositories.ResponseItem{Message: helpers.Success})
	}

	_, err = w.Write(response)
	if err != nil {
		status = http.StatusInternalServerError
		helpers.PrintError(logger, err, status)
		http.Error(w, err.Error(), status)

		return
	}

	status = http.StatusOK
	helpers.PrintStatus(logger, status)
}

func getNotifications(r *http.Request, session neo4j.Session, path string) ([]byte, int, error) {
	token, err := helpers.GetToken(r)
	if err != nil {
		return []byte{}, http.StatusBadRequest, helpers.InvalidTokenError(path, err)
	}

	notificationTests, err := datasources.GetNotificationTests(session, path, token)
	if err != nil {
		return nil, http.StatusInternalServerError, helpers.GetError(path, err)
	}

	response, err := json.Marshal(notificationTests)
	if err != nil {
		return nil, http.StatusInternalServerError, helpers.MarshalError(path, err)
	}

	return response, http.StatusOK, nil
}
