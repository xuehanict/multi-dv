package main

import (
	"fmt"
	"math/rand"
	"os"
	"sync"
	"time"
	//"math"
	"math"
)

const (
	BufferSize   = 1000
	ProbeCycle   = 3
	UpdateWindow = 1
)

type RouterID int
type Distance int

type MultPathRouter struct {
	ID           RouterID
	RouterTable  map[RouterID]map[RouterID]Distance
	UpdateTable  map[RouterID]*updateEntry
	Neighbours   []RouterID
	LinkBase     map[string]*Link
	RouterBase   map[RouterID]*MultPathRouter
	MessagePool  chan *Probe
	RequestPool  chan *Request
	ResponsePool chan *Response
	HoldRequests map[uint64]chan *Response
	timer        *time.Ticker
	wg           sync.WaitGroup
	quit         chan struct{}
}

type updateEntry struct {
	updateTime int64
	updated    bool
	bestHop    RouterID
	minDis     Distance
}

type Link struct {
	R1       RouterID
	R2       RouterID
	capacity int64
}

type Probe struct {
	dest     RouterID
	upper    RouterID
	distance Distance
}

type Request struct {
	RequestID   uint64
	Destination RouterID
	PathNodes   []RouterID
}

type Response struct {
	RequestID uint64
	Route     []RouterID
	Success   bool
	Reason    string
}

func (r *MultPathRouter) start() {
	r.wg.Add(1)
	defer r.wg.Done()
	for {
		select {
		case probe := <-r.MessagePool:
			r.handleProbe(probe)
		case request := <-r.RequestPool:
			r.handleRequest(request)
		case response := <-r.ResponsePool:
			r.handleResponse(response)
		case <-r.timer.C:
			for _, neighbor := range r.Neighbours {
				link := r.getLink(neighbor)
				if link == nil {
					fmt.Printf("router %v can't find the "+
						"link between %v", r.ID, neighbor)
					continue
				}
				probe := r.newProbe()
				err := r.sendMessageToRouter(neighbor, probe)
				if err != nil {
					fmt.Printf("router %v send probe to "+
						"%v failed : %v", r.ID, neighbor, err)
					continue
				}
			}
		case <-r.quit:
			return
		}
	}
}

func (r *MultPathRouter) Stop() {
	close(r.quit)
	r.wg.Wait()
}

func (r *MultPathRouter) newProbe() *Probe {
	probe := &Probe{
		dest:     r.ID,
		upper:    r.ID,
		distance: 0,
	}
	return probe
}

func (r *MultPathRouter) handleProbe(p *Probe) {
	if p.dest == r.ID {
		return
	}
	/*
		if r.ID == 10 {
			fmt.Printf("probe :%v \n", p)
		}
	*/
	mapNextHop, ok := r.RouterTable[p.dest]
	if !ok {
		r.RouterTable[p.dest] = make(map[RouterID]Distance)
		r.RouterTable[p.dest][p.upper] = p.distance + 1

		r.UpdateTable[p.dest] = &updateEntry{
			updated:    false,
			updateTime: time.Now().Unix(),
			bestHop:    p.upper,
			minDis:     p.distance + 1,
		}
		for _, neighbor := range r.Neighbours {
			probe := &Probe{
				dest:     p.dest,
				upper:    r.ID,
				distance: p.distance + 1,
			}
			err := r.sendMessageToRouter(neighbor, probe)
			if err != nil {
				os.Exit(1)
				fmt.Printf("router %v handle probe"+
					" %v failed : %v", r.ID, p, err)
			}
		}
	} else {
		mapNextHop[p.upper] = p.distance + 1
		// 最优的hop发来的probe，更新
		if p.upper == r.UpdateTable[p.dest].bestHop {
			// 比最小的还小，那么肯定还是最优解
			if p.distance+1 < r.UpdateTable[p.dest].minDis {
				r.UpdateTable[p.dest].updated = true
				r.UpdateTable[p.dest].bestHop = p.upper
				r.UpdateTable[p.dest].minDis = p.distance + 1

				// 否则遍历，找到最小的, 并且修改updateTable
			} else {

				newMinDistance := r.UpdateTable[p.dest].minDis
				newBestHop := r.UpdateTable[p.dest].bestHop
				for upper, distance := range mapNextHop {
					if distance < newMinDistance {
						newMinDistance = distance
						newBestHop = upper
					}
				}
				r.UpdateTable[p.dest].updated = true
				r.UpdateTable[p.dest].bestHop = newBestHop
				r.UpdateTable[p.dest].minDis = newMinDistance
			}
			// 非最优的hop发来的probe
		} else {
			newMinDistance := r.UpdateTable[p.dest].minDis
			newBestHop := r.UpdateTable[p.dest].bestHop
			bestChanged := false
			for upper, distance := range mapNextHop {
				if distance < newMinDistance {
					newMinDistance = distance
					newBestHop = upper
					bestChanged = true
				}
			}
			if bestChanged {
				r.UpdateTable[p.dest].updated = true
				r.UpdateTable[p.dest].bestHop = newBestHop
				r.UpdateTable[p.dest].minDis = newMinDistance
			}
		}

		timeDiff := time.Now().Unix() - r.UpdateTable[p.dest].updateTime
		if timeDiff >= UpdateWindow && r.UpdateTable[p.dest].updated {
			for _, neighbor := range r.Neighbours {
				probe := &Probe{
					dest:     p.dest,
					upper:    r.ID,
					distance: r.UpdateTable[p.dest].minDis,
				}
				/*
					if r.ID == 10 {
						fmt.Printf("send update probe")
					}
				*/
				err := r.sendMessageToRouter(neighbor, probe)
				r.UpdateTable[p.dest].updated = false
				r.UpdateTable[p.dest].updateTime = time.Now().Unix()
				if err != nil {
					os.Exit(1)
					fmt.Printf("router %v handle probe"+
						" %v failed : %v", r.ID, p, err)
				}
			}
		}
	}
}

func (r *MultPathRouter) sendMessageToRouter(id RouterID, i interface{}) error {
	neighbor, ok := r.RouterBase[id]
	if ok != true {
		return fmt.Errorf("can't find the router id : %v", id)
	}
	switch i.(type) {
	case *Probe:
		neighbor.MessagePool <- i.(*Probe)
	case *Request:
		neighbor.RequestPool <- i.(*Request)
	case *Response:
		neighbor.ResponsePool <- i.(*Response)
	}
	return nil
}

func newRouter(id RouterID) *MultPathRouter {
	router := &MultPathRouter{
		ID:           id,
		RouterTable:  make(map[RouterID]map[RouterID]Distance),
		Neighbours:   make([]RouterID, 0),
		UpdateTable:  make(map[RouterID]*updateEntry),
		MessagePool:  make(chan *Probe, BufferSize),
		RequestPool:  make(chan *Request, BufferSize),
		ResponsePool: make(chan *Response, BufferSize),
		HoldRequests: make(map[uint64]chan *Response),
		timer:        time.NewTicker(ProbeCycle * time.Second),
		wg:           sync.WaitGroup{},
		quit:         make(chan struct{}),
	}
	return router
}

func (r *MultPathRouter) getLink(neighbor RouterID) *Link {
	var link *Link
	link, ok := r.LinkBase[newLinkKey(r.ID, neighbor)]
	if ok == true {
		return link
	}
	link, ok = r.LinkBase[newLinkKey(neighbor, r.ID)]
	if ok == true {
		return link
	}
	return nil
}

// addLink adds a link between two nodes
func addLink(r1, r2 RouterID, capacity int64, nodeBase map[RouterID]*MultPathRouter,
	linkBase map[string]*Link) error {
	linkKey1 := newLinkKey(r1, r2)
	linkKey2 := newLinkKey(r2, r1)
	link := &Link{
		R1:       r1,
		R2:       r2,
		capacity: capacity,
	}
	_, ok1 := linkBase[linkKey1]
	_, ok2 := linkBase[linkKey2]
	ok := ok1 || ok2
	if ok {
		return fmt.Errorf("link: %v <-----> %v exsist", r1, r2)
	}
	linkBase[linkKey1] = link

	nodeBase[r1].Neighbours = append(nodeBase[r1].Neighbours, r2)
	nodeBase[r2].Neighbours = append(nodeBase[r2].Neighbours, r1)
	return nil
}

// 目前实现的是单路径的
func (r *MultPathRouter) sendRequest(dest RouterID) (*Request, error) {
	entry, ok := r.UpdateTable[dest]
	if ok {
		req := &Request{
			RequestID:   rand.Uint64(),
			Destination: dest,
			PathNodes:   append(make([]RouterID, 0), r.ID),
		}
		r.sendMessageToRouter(entry.bestHop, req)
		return req, nil
	} else {
		return nil, fmt.Errorf("cann't find the path")
	}
}

func (r *MultPathRouter) handleRequest(request *Request) {
	// 先判断是否有回路，如果有回路，直接丢弃这个request
	for _, node := range request.PathNodes {
		if node == r.ID {
			fmt.Printf("router %v 丢弃 从%v 发来的req %v \n", r.ID,
				request.PathNodes[len(request.PathNodes)-1], request)
			return
		}
	}

	fmt.Printf("router %v 收到request:%v \n", r.ID, request)
	// 如果我们就是目的地，则直接创建response，返回给上一跳节点
	if request.Destination == r.ID {
		res := &Response{
			RequestID: request.RequestID,
			Success:   true,
			Route:     append(request.PathNodes, r.ID),
		}
		r.sendMessageToRouter(request.PathNodes[len(request.PathNodes)-1], res)
	} else {

		// 不是目的节点，则分两种情况：1. 在路由表中找到，将request传给下一跳；
		// 2. 未在路由表中找到，返回一个失败的response给上一跳
		entry, ok := r.UpdateTable[request.Destination]
		if ok {
			req := &Request{
				RequestID:   request.RequestID,
				PathNodes:   append(request.PathNodes, r.ID),
				Destination: request.Destination,
			}

			if len(r.RouterTable[request.Destination]) >= 2 {

				leftMap := copyMap(r.RouterTable[req.Destination])
				delete(leftMap, entry.bestHop)

				if r.ID == 3 {
					fmt.Printf("%v\n", leftMap)
				}
				minDis := Distance(math.MaxInt8)
				minNeigh := RouterID(0)
				for neigh, distance := range leftMap {
					if distance < minDis {
						minNeigh = neigh
						minDis = distance
					}
				}
				fmt.Printf("router %v send request to %v\n",r.ID, minNeigh)
				fmt.Printf("router %v send request to %v\n",r.ID, entry.bestHop)

				r.sendMessageToRouter(minNeigh, req)
				r.sendMessageToRouter(entry.bestHop, req)

			} else {
				r.sendMessageToRouter(entry.bestHop, req)
			}
		} else {
			res := &Response{
				RequestID: request.RequestID,
				Success:   false,
				Route:     append(request.PathNodes, r.ID),
				Reason:    string(r.ID) + "cann't find the destination in routing table",
			}
			r.sendMessageToRouter(request.PathNodes[len(request.PathNodes)-1], res)
		}
	}
}

func (r *MultPathRouter) handleResponse(response *Response) error {
	if response.Route[0] == r.ID {
		if response.Success {
			r.HoldRequests[response.RequestID] <- response
			return nil
		} else {
			return fmt.Errorf("can not find the path")
		}
	} else {
		for i, node := range response.Route {
			if node == r.ID {
				r.sendMessageToRouter(response.Route[i-1], response)
				return nil
			}
		}
	}
	return nil
}

func (r *MultPathRouter) FindPath(dest RouterID) ([][]RouterID, error) {

	entry, ok := r.UpdateTable[dest]
	if ok {
		req := &Request{
			RequestID:   rand.Uint64(),
			Destination: dest,
			PathNodes:   append(make([]RouterID, 0), r.ID),
		}
		r.HoldRequests[req.RequestID] = make(chan *Response, BufferSize)
		defer delete(r.HoldRequests, req.RequestID)

		if len(r.RouterTable[req.Destination]) >= 2 {
			leftMap := copyMap(r.RouterTable[req.Destination])
			delete(leftMap, entry.bestHop)

			if r.ID == 3 {
				fmt.Printf("%v\n", leftMap)
			}
			minDis := Distance(math.MaxInt8)
			minNeigh := RouterID(0)
			for neigh, distance := range leftMap {
				if distance < minDis {
					minNeigh = neigh
					minDis = distance
				}
			}

				fmt.Printf("router %v send request to %v\n",r.ID, minNeigh)
				fmt.Printf("router %v send request to %v\n",r.ID, entry.bestHop)

			r.sendMessageToRouter(minNeigh, req)
			r.sendMessageToRouter(entry.bestHop, req)

		} else {
			r.sendMessageToRouter(entry.bestHop, req)
		}
		//r.sendMessageToRouter(entry.bestHop, req)
		routes := make([][]RouterID, 0)
		for {
			select {
			case response := <-r.HoldRequests[req.RequestID]:
				routes = append(routes, response.Route)
			case <-time.After(2 * time.Second):
				return routes, nil
			}
		}

	} else {
		return nil, fmt.Errorf("cann't find the dest in the routing table")
	}
}

func (r *MultPathRouter) PrintBestTable() {
	fmt.Printf("-----router: %v 's best table---------\n", r.ID)
	for key, entry := range r.UpdateTable {
		fmt.Printf("dest: %v  nextHop :%v distance: %v \n",
			key, entry.bestHop, entry.minDis)
	}
}

func (r *MultPathRouter) PrintTable() {
	fmt.Printf("-----router: %v 's full table---------\n", r.ID)
	for dest, sonTable := range r.RouterTable {
		fmt.Printf("for dest -%v \n", dest)
		for neighbour, entry := range sonTable {
			fmt.Printf("    neighbour :%v  distance %v \n", neighbour, entry)
		}
	}
}
