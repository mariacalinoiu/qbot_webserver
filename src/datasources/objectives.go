package datasources

import (
	"github.com/neo4j/neo4j-go-driver/neo4j"

	"qbot_webserver/src/repositories"
)

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
