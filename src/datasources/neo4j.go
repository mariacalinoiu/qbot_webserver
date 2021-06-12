package datasources

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
