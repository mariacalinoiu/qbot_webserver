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

	subjects, err := session.ReadTransaction(func(tx neo4j.Transaction) (interface{}, error) {
		records, err := tx.Run(query, params)
		if err != nil {
			return []repositories.SpinnerItem{}, err
		}
		var results []repositories.SpinnerItem
		for records.Next() {
			name, ok := records.Record().Get("name")
			if !ok {
				return []repositories.SpinnerItem{}, fmt.Errorf("'name' not found in query result")
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
