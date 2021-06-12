package handlers

func GetStringSliceFromInterfaceSlice(slice []interface{}) []string {
	var stringSlice []string
	for _, param := range slice {
		stringSlice = append(stringSlice, param.(string))
	}

	return stringSlice
}
