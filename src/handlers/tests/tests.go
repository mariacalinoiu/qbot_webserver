package tests

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/neo4j/neo4j-go-driver/neo4j"

	"qbot_webserver/src/datasources"
	helpers "qbot_webserver/src/helpers"
	"qbot_webserver/src/repositories"
)

func HandleTests(w http.ResponseWriter, r *http.Request, logger *log.Logger, driver neo4j.Driver, path string, s3Bucket string, s3Region string, s3Profile string) {
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
		response, status, err = getTests(r, session, path)
	case http.MethodPost, http.MethodPut:
		response, status, err = addTest(r, session, path, s3Bucket, s3Region, s3Profile, logger)
	case http.MethodDelete:
		status, err = deleteTest(r, session, path)
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

func getTests(r *http.Request, session neo4j.Session, path string) ([]byte, int, error) {
	token, err := helpers.GetToken(r)
	if err != nil {
		return nil, http.StatusBadRequest, helpers.InvalidTokenError(path, err)
	}
	testID, err := helpers.GetIntParameter(r, repositories.TestID, false)
	if err != nil {
		return nil, http.StatusBadRequest, helpers.BadParameterError(path, err)
	}
	searchString, err := helpers.GetStringParameter(r, repositories.Search, false)
	if err != nil {
		return nil, http.StatusBadRequest, helpers.BadParameterError(path, err)
	}

	tests, err := datasources.GetTests(session, path, token, testID, searchString, false)
	if err != nil {
		return nil, http.StatusInternalServerError, helpers.GetError(path, err)
	}

	response, err := json.Marshal(tests)
	if err != nil {
		return nil, http.StatusInternalServerError, helpers.MarshalError(path, err)
	}

	return response, http.StatusOK, nil
}

func addTest(r *http.Request, session neo4j.Session, path string, s3Bucket string, s3Region string, s3Profile string, logger *log.Logger) ([]byte, int, error) {
	token, err := helpers.GetToken(r)
	if err != nil {
		return nil, http.StatusBadRequest, helpers.InvalidTokenError(path, err)
	}
	test, err := extractTest(r)
	if err != nil {
		return nil, http.StatusBadRequest, helpers.CouldNotExtractBodyError(path, err)
	}

	testID, err := datasources.AddTest(session, path, token, test, s3Bucket, s3Region, s3Profile, logger)
	if err != nil {
		return nil, http.StatusInternalServerError, helpers.AddError(path, err)
	}
	tests, err := datasources.GetTests(session, path, token, testID, helpers.EmptyStringParameter, true)
	if err != nil {
		return nil, http.StatusInternalServerError, helpers.GetError(path, err)
	}

	response, err := json.Marshal(tests)
	if err != nil {
		return nil, http.StatusInternalServerError, helpers.MarshalError(path, err)
	}

	return response, http.StatusOK, nil
}

func deleteTest(r *http.Request, session neo4j.Session, path string) (int, error) {
	token, err := helpers.GetToken(r)
	if err != nil {
		return http.StatusBadRequest, helpers.InvalidTokenError(path, err)
	}
	testID, err := helpers.GetIntParameter(r, repositories.TestID, true)
	if err != nil {
		return http.StatusBadRequest, helpers.BadParameterError(path, err)
	}

	err = datasources.DeleteTest(session, path, token, testID)
	if err != nil {
		return http.StatusInternalServerError, helpers.GetError(path, err)
	}

	return http.StatusOK, nil
}

func extractTest(r *http.Request) (repositories.Test, error) {
	var unmarshalledTest repositories.Test

	body, err := ioutil.ReadAll(r.Body)
	fmt.Printf("request body: %+v", body)
	if err != nil {
		return repositories.Test{}, err
	}

	err = json.Unmarshal(body, &unmarshalledTest)
	if err != nil {
		return repositories.Test{}, err
	}

	return unmarshalledTest, nil
}
