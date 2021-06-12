package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/neo4j/neo4j-go-driver/neo4j"

	"qbot_webserver/src/datasources"
	helpers "qbot_webserver/src/helpers"
	"qbot_webserver/src/repositories"
)

func HandleSubjects(w http.ResponseWriter, r *http.Request, logger *log.Logger, driver neo4j.Driver, path string) {
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
	case http.MethodGet:
		response, status, err = getSubjects(r, session, path)
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

func getSubjects(r *http.Request, driver neo4j.Session, path string) ([]byte, int, error) {
	token, err := helpers.GetToken(r)
	if err != nil {
		return nil, http.StatusBadRequest, helpers.InvalidTokenError(path, err)
	}
	forUserOnly, err := helpers.GetBoolParam(r, repositories.ForUserOnly, false)
	if err != nil {
		return nil, http.StatusBadRequest, helpers.BadParameterError(path, err)
	}

	subjects, err := datasources.GetSubjects(driver, path, token, forUserOnly)
	if err != nil {
		return nil, http.StatusInternalServerError, helpers.GetError(path, err)
	}

	response, err := json.Marshal(subjects)
	if err != nil {
		return nil, http.StatusInternalServerError, helpers.MarshalError(path, err)
	}

	return response, http.StatusOK, nil
}

//func addSubjectsForUser(r *http.Request, driver neo4j.Driver, logger *log.Logger) (int, error) {
//	student, err := getStudentFromRequestBody(r)
//	if err != nil {
//		return http.StatusBadRequest, errors.New("student information sent on request body does not match required format")
//	}
//
//	_, err = db.InsertAdresa(student)
//	if err != nil {
//		logger.Printf("Internal error: %s", err.Error())
//		return http.StatusInternalServerError, errors.New("could not save student")
//	}
//
//	return http.StatusOK, nil
//}
//
//func getStudentFromRequestBody(r *http.Request) (repositories.Student, error) {
//	var unmarshalledStudent repositories.Student
//
//	body, err := ioutil.ReadAll(r.Body)
//	if err != nil {
//		return repositories.Student{}, err
//	}
//
//	err = json.Unmarshal(body, &unmarshalledStudent)
//	if err != nil {
//		return repositories.Student{}, err
//	}
//
//	return unmarshalledStudent, nil
//}
