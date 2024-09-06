package main

import "encoding/json"

func mustInt(x json.Number) int {
	xi64, err := x.Int64()
	if err != nil {
		panic(err)
	}
	return int(xi64)
}
