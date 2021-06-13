package handlers

import (
	"github.com/neo4j/neo4j-go-driver/neo4j"
)

func ConnectNeo4j(uri string, username string, password string) (neo4j.Driver, error) {
	driver, err := neo4j.NewDriver(uri, neo4j.BasicAuth(username, password, ""))
	if err != nil {
		return nil, err
	}

	return driver, nil
}

func GetNeo4jSession(driver neo4j.Driver) (neo4j.Session, error) {
	return driver.NewSession(neo4j.SessionConfig{
		AccessMode: neo4j.AccessModeWrite,
	})
}

func WriteTX(session neo4j.Session, query string, params map[string]interface{}) error {
	_, err := session.WriteTransaction(func(tx neo4j.Transaction) (interface{}, error) {
		result, err := tx.Run(query, params)
		if err != nil {
			return nil, err
		}

		_, err = result.Consume()
		if err != nil {
			return nil, err
		}

		return nil, nil
	})

	return err
}
