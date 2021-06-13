package datasources

import (
	"fmt"

	"github.com/neo4j/neo4j-go-driver/neo4j"

	helpers "qbot_webserver/src/helpers"
	"qbot_webserver/src/repositories"
)

const (
	studentLabel = "Student"
	teacherLabel = "Teacher"
)

// Tests

func AddTestAnswers(session neo4j.Session, token string, testID int, answers map[int][]string) error {
	// TODO

	return nil
}

func AddFeedbackForTest(session neo4j.Session, token string, testID int, feedback string) error {
	// TODO

	return nil
}

func OverwriteGradeForTest(session neo4j.Session, token string, testID int, studentID int, newGrade int) error {
	// TODO

	return nil
}

func SignalErrorForTest(session neo4j.Session, token string, testID int) error {
	// TODO

	return nil
}

func GetNotificationTests(session neo4j.Session, token string) ([]repositories.CompletedTest, error) {
	// TODO

	return []repositories.CompletedTest{}, nil
}

func GradeTest(session neo4j.Session, token string, test repositories.CompletedTest) error {
	// TODO

	return nil
}

func GetTests(session neo4j.Session, token string, onlyGraded bool, testID int, searchString string, subject string) ([]repositories.CompletedTest, error) {
	// TODO

	return []repositories.CompletedTest{}, nil
}

func AddTest(session neo4j.Session, token string, test repositories.Test) error {
	// TODO

	return nil
}

func DeleteTest(session neo4j.Session, token string, testID int) error {
	// TODO

	return nil
}

// Objectives

func GetObjectives(session neo4j.Session, token string, subject string) ([]repositories.Objective, error) {
	//query := fmt.Sprintf(`
	//	MATCH (s:Student)-[so:SET_OBJECTIVE]->(subj:Subject)
	//	WHERE s.token = $token AND subj.name = '%s'
	//	RETURN TODO
	//`, subject)
	//param := map[string]interface{}{
	//	"token": token,
	//}

	return []repositories.Objective{}, nil
}

func AddObjective(session neo4j.Session, token string, subject string, objective repositories.Objective) error {
	//query := fmt.Sprintf(`
	//	MATCH (s:Student)-[so:SET_OBJECTIVE]->(subj:Subject)
	//	WHERE s.token = $token AND subj.name = '%s'
	//	SET TODO
	//`, subject)
	//param := map[string]interface{}{
	//	"token": token,
	//}

	return nil
}

// General

func GetFaculties(session neo4j.Session) ([]repositories.SpinnerItem, error) {
	query := `
		MATCH (f:Faculty) 
		RETURN f.name AS name
	`

	return getSpinnerItems(session, query, map[string]interface{}{}, "name")
}

func GetSpecializations(session neo4j.Session, faculty string) ([]repositories.SpinnerItem, error) {
	query := fmt.Sprintf(`
		MATCH (s:Specialization)-[r:IN_FACULTY]->(f:Faculty) 
		WHERE f.name = '%s' 
		RETURN s.name AS name
	`, faculty)

	return getSpinnerItems(session, query, map[string]interface{}{}, "name")
}

func GetGroups(session neo4j.Session, faculty string, specialization string) ([]repositories.SpinnerItem, error) {
	query := fmt.Sprintf(`
		MATCH (g:Group)-[gs:HAS_SPECIALIZATION]->(s:Specialization)-[sf:IN_FACULTY]->(f:Faculty) 
		WHERE f.name = '%s' AND s.name = '%s'
		RETURN apoc.convert.toString(g.gID) AS group
	`, faculty, specialization)

	return getSpinnerItems(session, query, map[string]interface{}{}, "group")
}

func GetSubjects(session neo4j.Session, path string, token string, forUserOnly bool) ([]repositories.SpinnerItem, error) {
	tokenInfo, err := GetTokenInfo(session, token)
	if err != nil {
		return []repositories.SpinnerItem{}, helpers.InvalidTokenError(path, err)
	}

	query := `
		MATCH (s:Subject) 
		RETURN s.name AS name
	`
	params := map[string]interface{}{}

	if forUserOnly {
		if tokenInfo.Label == studentLabel {
			query = `
				MATCH (stud:Student)-[r:ENROLLED_IN]->(s:Subject) 
				WHERE stud.ID = $id 
				RETURN s.name AS name
			`
		} else {
			query = `
				MATCH (prof:Teacher)-[r:TEACHES]->(s:Subject) 
				WHERE prof.ID = $id 
				RETURN s.name AS name
			`
		}

		params = map[string]interface{}{
			"id": tokenInfo.ID,
		}
	}

	return getSpinnerItems(session, query, params, "name")
}

func GetTokenInfo(session neo4j.Session, token string) (repositories.TokenInfo, error) {
	query := `
		MATCH (n) 
		WHERE (n:Student OR n:Teacher) AND n.token = $token 
		RETURN n.ID AS userID, labels(n) AS type
	`
	tokenQueryResults, err := session.ReadTransaction(func(tx neo4j.Transaction) (interface{}, error) {
		records, err := tx.Run(query, map[string]interface{}{"token": token})
		if err != nil {
			return repositories.TokenInfo{}, err
		}

		for records.Next() {
			record := records.Record()
			resultID, ok := record.Get("userID")
			if !ok {
				return repositories.TokenInfo{}, fmt.Errorf("'userID' not found in query result")
			}
			resultType, ok := record.Get("type")
			if !ok {
				return repositories.TokenInfo{}, fmt.Errorf("'type' not found in query result")
			}

			return repositories.TokenInfo{
				ID:    int(resultID.(int64)),
				Label: helpers.GetStringSliceFromInterfaceSlice(resultType.([]interface{}))[0],
			}, nil
		}

		return repositories.TokenInfo{}, fmt.Errorf("'userID' not found")
	})

	if err != nil {
		return repositories.TokenInfo{}, err
	}

	return tokenQueryResults.(repositories.TokenInfo), nil
}

func getSpinnerItems(session neo4j.Session, query string, params map[string]interface{}, recordName string) ([]repositories.SpinnerItem, error) {
	subjects, err := session.ReadTransaction(func(tx neo4j.Transaction) (interface{}, error) {
		records, err := tx.Run(query, params)
		if err != nil {
			return []repositories.SpinnerItem{}, err
		}
		var results []repositories.SpinnerItem
		for records.Next() {
			name, ok := records.Record().Get(recordName)
			if !ok {
				return []repositories.SpinnerItem{}, fmt.Errorf("'%s' not found in query result", recordName)
			}
			results = append(results, repositories.SpinnerItem{Name: name.(string)})
		}

		return results, nil
	})
	if err != nil {
		return []repositories.SpinnerItem{}, err
	}

	return subjects.([]repositories.SpinnerItem), nil
}
