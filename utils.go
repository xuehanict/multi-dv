package main

import "strconv"

func newLinkKey(r1, r2 RouterID) string {
	return strconv.Itoa(int(r1)) + "-" + strconv.Itoa(int(r2))
}

func copyMap(m map[RouterID]Distance)  map[RouterID]Distance{
	resultMap := make(map[RouterID]Distance)
	for key, value := range m {
		resultMap[key] = value
	}
	return resultMap
}


type testGraph struct {
	Info  []string   `json:"info"`
	Nodes []testNode `json:"nodes"`
	Edges []testEdge `json:"edges"`
}

type testNode struct {
	Id RouterID `json:"id"`
}

type testEdge struct {
	Node1    RouterID `json:"node_1"`
	Node2    RouterID `json:"node_2"`
	Capacity int64    `json:"capacity"`
}