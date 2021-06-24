package datasources

import (
	"fmt"

	"github.com/neo4j/neo4j-go-driver/neo4j"

	helpers "qbot_webserver/src/helpers"
	"qbot_webserver/src/repositories"
)

func GetFaculties(session neo4j.Session) ([]repositories.Item, error) {
	query := `
		MATCH (f:Faculty) 
		RETURN f.name AS name
	`

	return getSpinnerItems(session, query, map[string]interface{}{}, "name")
}

func GetSpecializations(session neo4j.Session, faculty string) ([]repositories.Item, error) {
	query := fmt.Sprintf(`
		MATCH (s:Specialization)-[r:IN_FACULTY]->(f:Faculty) 
		WHERE f.name = '%s' 
		RETURN s.name AS name
	`, faculty)

	return getSpinnerItems(session, query, map[string]interface{}{}, "name")
}

func GetGroups(session neo4j.Session, faculty string, specialization string) ([]repositories.Item, error) {
	query := fmt.Sprintf(`
		MATCH (g:Group)-[gs:HAS_SPECIALIZATION]->(s:Specialization)-[sf:IN_FACULTY]->(f:Faculty) 
		WHERE f.name = '%s' AND s.name = '%s'
		RETURN apoc.convert.toString(g.gID) AS group
	`, faculty, specialization)

	return getSpinnerItems(session, query, map[string]interface{}{}, "group")
}

func GetSubjects(session neo4j.Session, path string, token string, forUserOnly bool) ([]repositories.Item, error) {
	var err error
	tokenInfo := repositories.TokenInfo{}

	if token != helpers.EmptyStringParameter {
		tokenInfo, err = GetTokenInfo(session, token)
		if err != nil && forUserOnly {
			return []repositories.Item{}, helpers.InvalidTokenError(path, err)
		}
	}

	query := `
		MATCH (s:Subject) 
		RETURN s.name AS name
	`
	params := map[string]interface{}{}

	if forUserOnly {
		if tokenInfo.Label == StudentLabel {
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

func getSpinnerItems(session neo4j.Session, query string, params map[string]interface{}, recordName string) ([]repositories.Item, error) {
	subjects, err := session.ReadTransaction(func(tx neo4j.Transaction) (interface{}, error) {

		fmt.Printf("query: %s\n", query)

		records, err := tx.Run(query, params)
		if err != nil {
			return []repositories.Item{}, err
		}
		var results []repositories.Item
		for records.Next() {
			name, err := helpers.GetStringParameterFromQuery(records.Record(), recordName, true, true)
			if err != nil {
				return []repositories.Item{}, err
			}

			results = append(results, repositories.Item{Name: name})
		}

		return results, nil
	})
	if err != nil {
		return []repositories.Item{}, err
	}

	return subjects.([]repositories.Item), nil
}
