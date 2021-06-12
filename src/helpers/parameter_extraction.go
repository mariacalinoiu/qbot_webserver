package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

func GetToken(r *http.Request) (string, error) {
	token, err := GetStringParameter(r, "token", true)
	if err != nil {
		return "", fmt.Errorf("could not get 'token' parameter: %s", err.Error())
	}

	return token, nil
}

func GetIntParameter(r *http.Request, paramName string, isMandatory bool) (int, error) {
	stringParam, err := GetStringParameter(r, paramName, isMandatory)
	if err != nil {
		return 0, err
	}
	if !isMandatory && len(stringParam) == 0 {
		return 0, nil
	}

	param, err := strconv.Atoi(stringParam)
	if err != nil {
		return 0, fmt.Errorf("could not convert parameter '%s' to integer", paramName)
	}

	return param, nil
}

func GetBoolParam(r *http.Request, paramName string, isMandatory bool) (bool, error) {
	stringParam, err := GetStringParameter(r, paramName, isMandatory)
	if err != nil {
		return false, err
	}
	if !isMandatory && len(stringParam) == 0 {
		return false, nil
	}

	param, err := strconv.ParseBool(stringParam)
	if err != nil {
		return false, fmt.Errorf("could not convert parameter '%s' to bool", paramName)
	}

	return param, nil
}

func GetStringParameter(r *http.Request, paramName string, isMandatory bool) (string, error) {
	var err error = nil
	params, ok := r.URL.Query()[paramName]

	if !ok || len(params[0]) < 1 {
		if isMandatory {
			err = fmt.Errorf("mandatory parameter '%s' not found", paramName)
		}
		return "", err
	}

	param := strings.Replace(params[0], `'`, ``, -1)
	param = strings.Replace(param, `"`, ``, -1)
	param = strings.Replace(param, `”`, ``, -1)

	return param, nil
}
