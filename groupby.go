package main

import (
	"strings"
)

// Generalized groupBy function
func generalizedGroupBy(funcName string, entries interface{}, getValue func(interface{}) (interface{}, error), addEntry func(map[string][]interface{}, interface{}, interface{})) (map[string][]interface{}, error) {
	entriesVal, err := getArrayValues(funcName, entries)

	if err != nil {
		return nil, err
	}

	groups := make(map[string][]interface{})
	for i := 0; i < entriesVal.Len(); i++ {
		v := entriesVal.Index(i).Interface()
		value, err := getValue(v)
		if err != nil {
			return nil, err
		}
		if value != nil {
			addEntry(groups, value, v)
		}
	}
	return groups, nil
}

func generalizedGroupByKey(funcName string, entries interface{}, key string, addEntry func(map[string][]interface{}, interface{}, interface{})) (map[string][]interface{}, error) {
	getKey := func(v interface{}) (interface{}, error) {
		return deepGet(v, key), nil
	}
	return generalizedGroupBy(funcName, entries, getKey, addEntry)
}

func groupByMulti(entries interface{}, key, sep string) (map[string][]interface{}, error) {
	return generalizedGroupByKey("groupByMulti", entries, key, func(groups map[string][]interface{}, value interface{}, v interface{}) {
		items := strings.Split(value.(string), sep)
		for _, item := range items {
			groups[item] = append(groups[item], v)
		}
	})
}

// groupBy groups a generic array or slice by the path property key
func groupBy(entries interface{}, key string) (map[string][]interface{}, error) {
	return generalizedGroupByKey("groupBy", entries, key, func(groups map[string][]interface{}, value interface{}, v interface{}) {
		groups[value.(string)] = append(groups[value.(string)], v)
	})
}

// groupByWithDefault is the same as groupBy but allows a default value to be set
func groupByWithDefault(entries interface{}, key string, defaultValue string) (map[string][]interface{}, error) {
	getValueWithDefault := func(v interface{}) (interface{}, error) {
		value := deepGet(v, key)
		if value == nil {
			return defaultValue, nil
		}
		return value, nil
	}
	return generalizedGroupBy("groupByWithDefault", entries, getValueWithDefault, func(groups map[string][]interface{}, value interface{}, v interface{}) {
		groups[value.(string)] = append(groups[value.(string)], v)
	})
}

// groupByKeys is the same as groupBy but only returns a list of keys
func groupByKeys(entries interface{}, key string) ([]string, error) {
	keys, err := generalizedGroupByKey("groupByKeys", entries, key, func(groups map[string][]interface{}, value interface{}, v interface{}) {
		groups[value.(string)] = append(groups[value.(string)], v)
	})

	if err != nil {
		return nil, err
	}

	ret := []string{}
	for k := range keys {
		ret = append(ret, k)
	}
	return ret, nil
}
