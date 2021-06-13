package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"qbot_webserver/src/repositories"
)

const (
	EmptyIntParameter    = -1
	EmptyStringParameter = ""
	EmptyBoolParameter   = false
)

func GetToken(r *http.Request) (string, error) {
	token, err := GetStringParameter(r, repositories.Token, true)
	if err != nil {
		return EmptyStringParameter, fmt.Errorf("could not get '%s' parameter: %s", repositories.Token, err.Error())
	}

	return token, nil
}

func GetIntParameter(r *http.Request, paramName string, isMandatory bool) (int, error) {
	stringParam, err := GetStringParameter(r, paramName, isMandatory)
	if err != nil {
		return EmptyIntParameter, err
	}
	if !isMandatory && len(stringParam) == 0 {
		return EmptyIntParameter, nil
	}

	param, err := strconv.Atoi(stringParam)
	if err != nil {
		return EmptyIntParameter, fmt.Errorf("could not convert parameter '%s' to integer", paramName)
	}

	return param, nil
}

func GetBoolParameter(r *http.Request, paramName string, isMandatory bool) (bool, error) {
	stringParam, err := GetStringParameter(r, paramName, isMandatory)
	if err != nil {
		return EmptyBoolParameter, err
	}
	if !isMandatory && len(stringParam) == 0 {
		return EmptyBoolParameter, nil
	}

	param, err := strconv.ParseBool(stringParam)
	if err != nil {
		return EmptyBoolParameter, fmt.Errorf("could not convert parameter '%s' to bool", paramName)
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
		return EmptyStringParameter, err
	}

	param := strings.Replace(params[0], `'`, ``, -1)
	param = strings.Replace(param, `"`, ``, -1)
	param = strings.Replace(param, `”`, ``, -1)

	return param, nil
}
