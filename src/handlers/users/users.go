package users

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

func HandleUsers(w http.ResponseWriter, r *http.Request, logger *log.Logger, driver neo4j.Driver, path string) {
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
		response, status, err = getUser(r, session, path)
	case http.MethodPost, http.MethodPut:
		response, status, err = signUp(r, session, path)
	case http.MethodDelete:
		status, err = deleteUser(r, session, path)
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

func getUser(r *http.Request, session neo4j.Session, path string) ([]byte, int, error) {
	token, err := helpers.GetToken(r)
	if err != nil {
		return nil, http.StatusBadRequest, helpers.InvalidTokenError(path, err)
	}

	user, err := datasources.GetUser(session, path, token)
	if err != nil {
		return nil, http.StatusInternalServerError, helpers.GetError(path, err)
	}

	response, err := json.Marshal(user)
	if err != nil {
		return nil, http.StatusInternalServerError, helpers.MarshalError(path, err)
	}

	return response, http.StatusOK, nil
}

func deleteUser(r *http.Request, session neo4j.Session, path string) (int, error) {
	token, err := helpers.GetToken(r)
	if err != nil {
		return http.StatusBadRequest, helpers.InvalidTokenError(path, err)
	}

	err = datasources.DeleteUser(session, path, token)
	if err != nil {
		return http.StatusInternalServerError, helpers.GetError(path, err)
	}

	return http.StatusOK, nil
}

func signUp(r *http.Request, session neo4j.Session, path string) ([]byte, int, error) {
	userType, err := helpers.GetStringParameter(r, repositories.UserType, true)
	if err != nil {
		return nil, http.StatusBadRequest, helpers.BadParameterError(path, err)
	}
	var user interface{}
	if userType == datasources.StudentLabel {
		user, err = extractStudent(r)
	} else {
		user, err = extractTeacher(r)
	}
	if err != nil {
		return nil, http.StatusBadRequest, helpers.BadParameterError(path, err)
	}

	tokenItem, err := datasources.AddUser(session, userType, user)
	if err != nil {
		return nil, http.StatusInternalServerError, helpers.GetError(path, err)
	}

	response, err := json.Marshal(tokenItem)
	if err != nil {
		return nil, http.StatusInternalServerError, helpers.MarshalError(path, err)
	}

	return response, http.StatusOK, nil
}

func extractStudent(r *http.Request) (repositories.Student, error) {
	var unmarshalledStudent repositories.Student

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return repositories.Student{}, err
	}

	err = json.Unmarshal(body, &unmarshalledStudent)
	if err != nil {
		return repositories.Student{}, err
	}

	return unmarshalledStudent, nil
}

func extractTeacher(r *http.Request) (repositories.Professor, error) {
	var unmarshalledTeacher repositories.Professor

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return repositories.Professor{}, err
	}

	err = json.Unmarshal(body, &unmarshalledTeacher)
	if err != nil {
		return repositories.Professor{}, err
	}

	return unmarshalledTeacher, nil
}
