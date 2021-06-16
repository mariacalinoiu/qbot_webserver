package handlers

import (
	"encoding/json"
	"fmt"

	"github.com/neo4j/neo4j-go-driver/neo4j"
)

func GetAnswerMapFromQuery(record neo4j.Record, key string, shouldCheck bool, mandatory bool) (map[int][]string, error) {
	stringAnswers, err := GetStringParameterFromQuery(record, key, shouldCheck, mandatory)
	if err != nil {
		return map[int][]string{}, err
	}
	if stringAnswers == "" {
		return map[int][]string{}, nil
	}

	var answers map[int][]string
	err = json.Unmarshal([]byte(stringAnswers), &answers)
	if err != nil {
		return map[int][]string{}, err
	}

	return answers, nil
}

func GetStringFromAnswerMap(answers map[int][]string) (string, error) {
	stringAnswers, err := json.Marshal(answers)
	if err != nil {
		return "", err
	}

	return string(stringAnswers), nil
}

func GetStringSliceFromInterfaceSlice(slice []interface{}) []string {
	var stringSlice []string
	for _, param := range slice {
		stringSlice = append(stringSlice, param.(string))
	}

	return stringSlice
}

func GetStringParameterFromQuery(record neo4j.Record, key string, shouldCheck bool, mandatory bool) (string, error) {
	interfaceParam, ok := record.Get(key)
	if shouldCheck && !ok {
		return "", fmt.Errorf("'%s' not found in query result", key)
	}

	param, ok := interfaceParam.(string)
	if mandatory && !ok {
		return "", fmt.Errorf("wrong type for '%s' parameter", key)
	}

	return param, nil
}

func GetIntParameterFromQuery(record neo4j.Record, key string, shouldCheck bool, mandatory bool) (int, error) {
	interfaceParam, ok := record.Get(key)
	if shouldCheck && !ok {
		return 0, fmt.Errorf("'%s' not found in query result", key)
	}

	param, ok := interfaceParam.(int64)
	if mandatory && !ok {
		return 0, fmt.Errorf("wrong type for '%s' parameter", key)
	}

	return int(param), nil
}

func GetBoolParameterFromQuery(record neo4j.Record, key string, shouldCheck bool, mandatory bool) (bool, error) {
	interfaceParam, ok := record.Get(key)
	if shouldCheck && !ok {
		return false, fmt.Errorf("'%s' not found in query result", key)
	}

	param, ok := interfaceParam.(bool)
	if mandatory && !ok {
		return false, fmt.Errorf("wrong type for '%s' parameter", key)
	}

	return param, nil
}
