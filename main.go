package main

import (
	"io/ioutil"
	"fmt"
	"os"
	"encoding/json"
	"time"
	//"bufio"
	"bufio"
)

const (
	twoNodesGraphFile = "testdata/two_nodes.json"

	basicGraphFile = "testdata/basic_graph.json"

	tenNodesGraphFile = "testdata/ten_nodes.json"
)

func main() {

	var (
		nodes= make(map[RouterID]*MultPathRouter, 0)
		edges= make(map[string]*Link, 0)
	)
	graphJson, err := ioutil.ReadFile(tenNodesGraphFile)
	if err != nil {
		fmt.Printf("can't open the json file: %v", err)
		os.Exit(1)
	}

	var g testGraph
	if err := json.Unmarshal(graphJson, &g); err != nil {
		fmt.Printf("%v", err)
		os.Exit(1)
	}

	for _, node := range g.Nodes {
		router := newRouter(node.Id)
		router.RouterBase = nodes
		router.LinkBase = edges
		nodes[node.Id] = router
	}

	for _, edge := range g.Edges {
		err := addLink(edge.Node1, edge.Node2, edge.Capacity, nodes, edges)
		if err != nil {
			fmt.Printf("failed: %v", err)
			os.Exit(1)
		}
	}

	for _, router := range nodes {
		go router.start()
		time.Sleep(1 * time.Second)
	}
	/*
	go func() {
		modify := edges["2-3"]
		time.Sleep(20 * time.Second)
		modify.capacity = 3333
	}()

	for {
		for _, router := range nodes {
			//router.PrintBestTable()
			router.PrintTable()
		}
		time.Sleep(3 * time.Second)
	}
*/
	time.Sleep(10 * time.Second)
	f := bufio.NewReader(os.Stdin)
	var origion, end RouterID
	for{
		fmt.Printf("-----find way-------\n")
		fmt.Print("Please input the origin node ID: ")
		input, _ := f.ReadString('\n')
		fmt.Sscanf(input, "%d", &origion)
		fmt.Printf("Please input the end node ID:")
		input, _ = f.ReadString('\n')
		fmt.Sscanf(input,"%d", &end)

		route,_ := nodes[origion].FindPath(end)
		fmt.Printf("route is : %v\n", route)
	}

}
