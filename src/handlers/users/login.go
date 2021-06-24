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

func HandleLogin(w http.ResponseWriter, r *http.Request, logger *log.Logger, driver neo4j.Driver, path string) {
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
		response, status, err = logIn(r, session, path)
	case http.MethodPut:
		status, err = validateToken(r, session, path)
	case http.MethodDelete:
		status, err = logOut(r, session, path)
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

func logIn(r *http.Request, session neo4j.Session, path string) ([]byte, int, error) {
	user, err := extractUser(r)
	if err != nil {
		return nil, http.StatusInternalServerError, helpers.GetError(path, err)
	}

	completeUser, err := datasources.GetUserByEmailAndPassword(session, path, user.Email, user.Password)
	if err != nil {
		return nil, http.StatusInternalServerError, helpers.GetError(path, err)
	}

	response, err := json.Marshal(completeUser)
	if err != nil {
		return nil, http.StatusInternalServerError, helpers.MarshalError(path, err)
	}

	return response, http.StatusOK, nil
}

func logOut(r *http.Request, session neo4j.Session, path string) (int, error) {
	token, err := helpers.GetToken(r)
	if err != nil {
		return http.StatusBadRequest, helpers.InvalidTokenError(path, err)
	}
	err = datasources.DeleteToken(session, path, token)
	if err != nil {
		return http.StatusBadRequest, helpers.InvalidTokenError(path, err)
	}

	return http.StatusOK, nil
}

func validateToken(r *http.Request, session neo4j.Session, path string) (int, error) {
	token, err := helpers.GetToken(r)
	if err != nil {
		return http.StatusBadRequest, helpers.InvalidTokenError(path, err)
	}
	_, err = datasources.GetTokenInfo(session, token)
	if err != nil {
		return http.StatusBadRequest, helpers.InvalidTokenError(path, err)
	}

	return http.StatusOK, nil
}

func extractUser(r *http.Request) (repositories.User, error) {
	var unmarshalledUser repositories.User

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return repositories.User{}, err
	}

	err = json.Unmarshal(body, &unmarshalledUser)
	if err != nil {
		return repositories.User{}, err
	}

	return unmarshalledUser, nil
}
