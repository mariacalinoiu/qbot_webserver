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

func HandleTestGrade(w http.ResponseWriter, r *http.Request, logger *log.Logger, driver neo4j.Driver, path string) {
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
	case http.MethodPost:
		status, err = gradeTest(r, logger, session, path)
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

func gradeTest(r *http.Request, logger *log.Logger, session neo4j.Session, path string) (int, error) {
	token, err := helpers.GetToken(r)
	if err != nil {
		return http.StatusBadRequest, helpers.InvalidTokenError(path, err)
	}
	test, err := extractCompletedTest(r)
	if err != nil {
		return http.StatusBadRequest, helpers.CouldNotExtractBodyError(path, err)
	}

	err = datasources.GradeTest(logger, session, path, token, test)
	if err != nil {
		return http.StatusInternalServerError, helpers.GetError(path, err)
	}

	return http.StatusOK, nil
}

func extractCompletedTest(r *http.Request) (repositories.CompletedTest, error) {
	var unmarshalledTest repositories.CompletedTest

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return repositories.CompletedTest{}, err
	}

	err = json.Unmarshal(body, &unmarshalledTest)
	if err != nil {
		return repositories.CompletedTest{}, err
	}

	return unmarshalledTest, nil
}
