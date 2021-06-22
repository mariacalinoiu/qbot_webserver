package datasources

import (
	"fmt"

	"github.com/neo4j/neo4j-go-driver/neo4j"

	helpers "qbot_webserver/src/helpers"
	"qbot_webserver/src/repositories"
)

func GetObjectives(session neo4j.Session, path string, token string, subject string, search string) ([]repositories.Objective, error) {
	tokenInfo, err := GetTokenInfo(session, token)
	if err != nil || tokenInfo.Label != studentLabel {
		return []repositories.Objective{}, helpers.InvalidTokenError(path, err)
	}

	objectives, err := getObjectivesWithoutCompletedTestsForStudent(session, tokenInfo.ID, subject, search)
	if err != nil {
		return []repositories.Objective{}, err
	}

	for index, objective := range objectives {
		completedTests, err := getAllCompletedTestsForStudent(
			session,
			tokenInfo.ID,
			helpers.EmptyStringParameter,
			objective.Tests[0].Subject,
		)
		if err != nil {
			return []repositories.Objective{}, err
		}

		objectives[index].Tests = completedTests
	}

	return objectives, nil
}

func AddObjective(session neo4j.Session, path string, token string, subject string, objective repositories.Objective) error {
	tokenInfo, err := GetTokenInfo(session, token)
	if err != nil || tokenInfo.Label != studentLabel {
		return helpers.InvalidTokenError(path, err)
	}

	nextID := objective.ID
	if objective.ID == 0 {
		nextID, err = getNextRelationshipID(session, "SET_OBJECTIVE", "ID")
		if err != nil {
			return err
		}
	}

	query := fmt.Sprintf(`
		MATCH (s:Student {ID:$studentID}), (subj:Subject {name:'%s'}) 
		MERGE (s)-[ssubj:SET_OBJECTIVE]->(subj) 
		SET ssubj.ID=$nextID, ssubj.timestampStart=$ts, ssubj.timestampEnd=$te, ssubj.target=$target 
	`, subject)

	params := map[string]interface{}{
		"studentID": tokenInfo.ID,
		"ts":        objective.StartTimestamp,
		"te":        objective.EndTimestamp,
		"target":    objective.TargetGrade,
		"nextID":    nextID,
	}

	return helpers.WriteTX(session, query, params)
}

func getObjectivesWithoutCompletedTestsForStudent(session neo4j.Session, studentID int, subject string, searchString string) ([]repositories.Objective, error) {
	extraCondition := ""
	extraConditionSearch := ""
	if subject != helpers.EmptyStringParameter {
		extraCondition = fmt.Sprintf(" AND subj.name = '%s' ", subject)
	} else if searchString != helpers.EmptyStringParameter {
		extraConditionSearch = fmt.Sprintf(`
			CALL db.index.fulltext.queryNodes('subjects', '%s~')
			YIELD node
			WITH node.name as name
		`, searchString)
		extraCondition = " AND subj.name = name"
	}

	query := fmt.Sprintf(` %s 
		MATCH (s:Student)-[ssubj:SET_OBJECTIVE]->(subj:Subject)
		WHERE s.ID = $studentID %s 
		RETURN subj.name, ssubj.timestampStart, ssubj.timestampEnd, ssubj.target, ssubj.ID 
	`, extraConditionSearch, extraCondition)

	testResults, err := session.ReadTransaction(func(tx neo4j.Transaction) (interface{}, error) {
		var results []repositories.Objective

		fmt.Printf("query: %s\n", query)

		records, err := tx.Run(query, map[string]interface{}{"studentID": studentID})
		if err != nil {
			return []repositories.Objective{}, err
		}

		for records.Next() {
			record := records.Record()
			objective, err := getObjectiveFromQuery(record)
			if err != nil {
				return []repositories.Objective{}, err
			}

			results = append(results, objective)
		}

		return results, nil
	})

	if err != nil {
		return []repositories.Objective{}, err
	}

	return testResults.([]repositories.Objective), nil
}

func getObjectiveFromQuery(record neo4j.Record) (repositories.Objective, error) {
	ID, err := helpers.GetIntParameterFromQuery(record, "ssubj.ID", true, true)
	timestampStart, err := helpers.GetIntParameterFromQuery(record, "ssubj.timestampStart", true, true)
	if err != nil {
		return repositories.Objective{}, err
	}
	if err != nil {
		return repositories.Objective{}, err
	}
	timestampEnd, err := helpers.GetIntParameterFromQuery(record, "ssubj.timestampEnd", true, true)
	if err != nil {
		return repositories.Objective{}, err
	}
	targetGrade, err := helpers.GetIntParameterFromQuery(record, "ssubj.target", true, true)
	if err != nil {
		return repositories.Objective{}, err
	}
	subjectName, err := helpers.GetStringParameterFromQuery(record, "subj.name", true, true)
	if err != nil {
		return repositories.Objective{}, err
	}

	return repositories.Objective{
		ID:             ID,
		Subject:        subjectName,
		TargetGrade:    targetGrade,
		StartTimestamp: timestampStart,
		EndTimestamp:   timestampEnd,
		Tests: []repositories.CompletedTest{
			{
				Test: repositories.Test{
					Subject: subjectName,
				},
			},
		},
	}, nil
}

func getNextRelationshipID(session neo4j.Session, relationshipType string, IDProperty string) (int, error) {
	query := fmt.Sprintf("MATCH ()-[n:%s]->() RETURN max(n.%s) + 1 as next", relationshipType, IDProperty)
	params := map[string]interface{}{}

	nextID, err := session.ReadTransaction(func(tx neo4j.Transaction) (interface{}, error) {

		fmt.Printf("query: %s\n", query)

		records, err := tx.Run(query, params)
		if err != nil {
			return 0, err
		}
		for records.Next() {
			nextID, err := helpers.GetIntParameterFromQuery(records.Record(), "next", true, true)
			if err != nil {
				return 0, err
			}

			return nextID, nil
		}

		return 0, nil
	})

	if err != nil {
		return 0, err
	}

	return nextID.(int), nil
}
