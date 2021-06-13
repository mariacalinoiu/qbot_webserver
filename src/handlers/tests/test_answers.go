package tests

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

func HandleTestAnswers(w http.ResponseWriter, r *http.Request, logger *log.Logger, driver neo4j.Driver, path string) {
	var response []byte
	var status int
	var err error

	helpers.SetContentType(w)
	session, err := datasources.GetNeo4jSession(driver)
	if err != nil {
		helpers.PrintError(logger, err, status)
		http.Error(w, err.Error(), status)

		return
	}
	defer session.Close()

	switch r.Method {
	case http.MethodOptions:
		helpers.SetAccessControlHeaders(w)
	case http.MethodPost:
		status, err = addAnswers(r, session, path)
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

func addAnswers(r *http.Request, session neo4j.Session, path string) (int, error) {
	token, err := helpers.GetToken(r)
	if err != nil {
		return http.StatusBadRequest, helpers.InvalidTokenError(path, err)
	}
	answers, err := extractAnswers(r)
	if err != nil {
		return http.StatusBadRequest, helpers.CouldNotExtractBodyError(path, err)
	}
	testID, err := helpers.GetIntParameter(r, repositories.TestID, true)
	if err != nil {
		return http.StatusBadRequest, helpers.BadParameterError(path, err)
	}

	err = datasources.AddTestAnswers(session, token, testID, answers)
	if err != nil {
		return http.StatusInternalServerError, helpers.GetError(path, err)
	}

	return http.StatusOK, nil
}

func extractAnswers(r *http.Request) (map[int][]string, error) {
	var unmarshalledAnswers map[int][]string

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return map[int][]string{}, err
	}

	err = json.Unmarshal(body, &unmarshalledAnswers)
	if err != nil {
		return map[int][]string{}, err
	}

	return unmarshalledAnswers, nil
}
