package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

func getIntParameter(r *http.Request, paramName string, isMandatory bool) (int, error) {
	stringParam, err := getStringParameter(r, paramName, isMandatory)
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

func getStringParameter(r *http.Request, paramName string, isMandatory bool) (string, error) {
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
