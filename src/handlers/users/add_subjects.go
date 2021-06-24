package users

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/neo4j/neo4j-go-driver/neo4j"

	"qbot_webserver/src/datasources"
	helpers "qbot_webserver/src/helpers"
	"qbot_webserver/src/repositories"
)

func HandleAddSubjects(w http.ResponseWriter, r *http.Request, logger *log.Logger, driver neo4j.Driver, path string) {
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
	case http.MethodPut, http.MethodPost:
		status, err = setSubjects(r, session, path)
	default:
		status = http.StatusBadRequest
		err = helpers.WrongMethodError(path)
	}

	if err != nil {
		helpers.PrintError(logger, err, status)
		http.Error(w, err.Error(), status)

		return
	}

	response, _ = json.Marshal(repositories.ResponseItem{Message: helpers.Success})
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

func setSubjects(r *http.Request, session neo4j.Session, path string) (int, error) {
	token, err := helpers.GetToken(r)
	if err != nil {
		return http.StatusBadRequest, helpers.InvalidTokenError(path, err)
	}
	user, err := extractUser(r)
	if err != nil {
		return http.StatusInternalServerError, helpers.GetError(path, err)
	}

	err = datasources.SetSubjectsForUser(session, path, token, user.Subjects)
	if err != nil {
		return http.StatusInternalServerError, helpers.GetError(path, err)
	}

	return http.StatusOK, nil
}
