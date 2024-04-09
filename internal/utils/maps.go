package utils

func ConvertMap(in map[string]interface{}) map[string]string {
	res := make(map[string]string)
	for k, v := range in {
		value, ok := v.(string)
		if ok {
			res[k] = value
		}
	}
	return res
}
