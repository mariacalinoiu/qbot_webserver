package datasources

import (
	"github.com/neo4j/neo4j-go-driver/v4/neo4j"
)

func ConnectNeo4j(uri string, username string, password string) (neo4j.Driver, error) {
	driver, err := neo4j.NewDriver(uri, neo4j.BasicAuth(username, password, ""))
	if err != nil {
		return nil, err
	}

	return driver, nil
}
