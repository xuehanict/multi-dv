package main

import (
	"fmt"
	"sync"
	"time"
	"os"
)

const BufferSize = 1000
const SendCycle = 1

type RouterID int
type Distance int

type MultPathRouter struct {
	ID          RouterID
	RouterTable map[RouterID]map[RouterID]Distance
	UpdateTable map[RouterID]*updateEntry
	Neighbours  []RouterID
	LinkBase    map[string]*Link
	RouterBase  map[RouterID]*MultPathRouter
	MessagePool chan *Probe
	timer       *time.Ticker
	wg          sync.WaitGroup
	quit        chan struct{}
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

func (r *MultPathRouter) start() {
	r.wg.Add(1)
	defer r.wg.Done()
	for {
		select {
		case probe := <-r.MessagePool:
			r.handleProbe(probe)
		case r.timer.C:
			for _, neighbor := range r.Neighbours {
				link := r.getLink(neighbor)
				if link == nil {
					fmt.Printf("router %v can't find the "+
						"link between %v", r.ID, neighbor)
					continue
				}
				probe := r.newProbe()
				err := r.sendProbeToRouter(neighbor, probe)
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

func (r *MultPathRouter) Stop ()  {
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

	mapNextHop, ok := r.RouterTable[p.dest]
	if !ok {
		r.RouterTable[p.dest] = make(map[RouterID]Distance)
		r.RouterTable[p.dest][p.upper] = p.distance + 1

		r.UpdateTable[p.dest] = &updateEntry{
			updated: false,
			updateTime: time.Now().Unix(),
			bestHop: p.upper,
			minDis: p.distance + 1,
		}
		for _, neighbor := range r.Neighbours {
			probe := &Probe{
				dest:    p.dest,
				upper:   r.ID,
				distance: p.distance + 1,
			}
			err := r.sendProbeToRouter(neighbor, probe)
			if err != nil {
				os.Exit(1)
				fmt.Printf("router %v handle probe"+
					" %v failed : %v", r.ID, p, err)
			}
		}
	} else {
		mapNextHop[p.upper] = p.distance + 1
		if p.distance + 1 < r.UpdateTable[p.dest].minDis {
			r.UpdateTable[p.dest] = &updateEntry{
				updated: false,
				updateTime: time.Now().Unix(),
				bestHop: p.upper,
				minDis: p.distance + 1,
			}
		}
	}

}

func (r *MultPathRouter) sendProbeToRouter(id RouterID, probe *Probe) error {
	neighbor, ok := r.RouterBase[id]
	if ok != true {
		return fmt.Errorf("can't find the router id : %v", id)
	}
	neighbor.MessagePool <- probe
	return nil
}

func newRouter(id RouterID) *MultPathRouter {
	router := &MultPathRouter{
		ID:          id,
		RouterTable: make(map[RouterID]map[RouterID]Distance),
		Neighbours:  make([]RouterID, 0),
		UpdateTable: make(map[RouterID]*updateEntry),
		MessagePool: make(chan *Probe, BufferSize),
		timer:       time.NewTicker(SendCycle * time.Second),
		wg:          sync.WaitGroup{},
		quit:        make(chan struct{}),
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
