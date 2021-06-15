package handlers

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/neo4j/neo4j-go-driver/neo4j"

	"qbot_webserver/src/datasources"
	helpers "qbot_webserver/src/helpers"
	"qbot_webserver/src/repositories"
)

func HandleObjectives(w http.ResponseWriter, r *http.Request, logger *log.Logger, driver neo4j.Driver, path string) {
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
		response, status, err = getObjectives(r, session, path)
	case http.MethodPost, http.MethodPut:
		status, err = setObjective(r, session, path)
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

func getObjectives(r *http.Request, session neo4j.Session, path string) ([]byte, int, error) {
	token, err := helpers.GetToken(r)
	if err != nil {
		return nil, http.StatusBadRequest, helpers.InvalidTokenError(path, err)
	}
	subject, err := helpers.GetStringParameter(r, repositories.Subject, false)
	if err != nil {
		return nil, http.StatusBadRequest, helpers.BadParameterError(path, err)
	}

	objective, err := datasources.GetObjectives(session, path, token, subject)
	if err != nil {
		return nil, http.StatusInternalServerError, helpers.GetError(path, err)
	}

	response, err := json.Marshal(objective)
	if err != nil {
		return nil, http.StatusInternalServerError, helpers.MarshalError(path, err)
	}

	return response, http.StatusOK, nil
}

func setObjective(r *http.Request, session neo4j.Session, path string) (int, error) {
	token, err := helpers.GetToken(r)
	if err != nil {
		return http.StatusBadRequest, helpers.InvalidTokenError(path, err)
	}
	objective, err := extractObjective(r)
	if err != nil {
		return http.StatusBadRequest, helpers.CouldNotExtractBodyError(path, err)
	}
	subject, err := helpers.GetStringParameter(r, repositories.Subject, false)
	if err != nil {
		return http.StatusBadRequest, helpers.BadParameterError(path, err)
	}

	err = datasources.AddObjective(session, path, token, subject, objective)
	if err != nil {
		return http.StatusInternalServerError, helpers.GetError(path, err)
	}

	return http.StatusOK, nil
}

func extractObjective(r *http.Request) (repositories.Objective, error) {
	var unmarshalledObjective repositories.Objective

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return repositories.Objective{}, err
	}

	err = json.Unmarshal(body, &unmarshalledObjective)
	if err != nil {
		return repositories.Objective{}, err
	}

	return unmarshalledObjective, nil
}
