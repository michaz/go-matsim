package main

import (
  "fmt"
  "time"
  "encoding/xml"
  "net/http"
  "container/heap"
  "os"
  "compress/gzip"
)
func time2Dur(dur_time time.Time) time.Duration {
  zero, _ := time.Parse("15:04", "00:00")
  dur := dur_time.Sub(zero)
  return dur
}
type ChanWithHead struct {
  head Event
  tail chan Event
}
func (cwh ChanWithHead) priority() float64 {
  return cwh.head.Time
}
type PriorityQueue []*ChanWithHead
func (pq PriorityQueue) Len() int { return len(pq) }
func (pq PriorityQueue) Less(i, j int) bool {
   return pq[i].priority() < pq[j].priority()
}
func (pq PriorityQueue) Swap(i, j int) {
  pq[i], pq[j] = pq[j], pq[i]
}
func (pq *PriorityQueue) Push(x interface{}) {
  item := x.(*ChanWithHead)
  *pq = append(*pq, item)
}
func (pq *PriorityQueue) Pop() interface{} {
  old := *pq
  n := len(old)
  item := old[n-1]
  *pq = old[0 : n-1]
  return item
}

type Link struct {
  From string `xml:"from,attr"`
  To string `xml:"to,attr"`
}
type Network struct {
  Links []Link `xml:"links>link"`
}
type Activity struct {
  Link string `xml:"link,attr"`
  Type string `xml:"type,attr"`
  EndTime string `xml:"end_time,attr"`
  Duration string `xml:"dur,attr"`
}
type Plan struct {
  Activities []Activity `xml:"act"`
}
type Event struct {
  XMLName xml.Name `xml:"event"`
  Time float64 `xml:"time,attr"`
  Type string `xml:"type,attr"`
  Person string `xml:"person,attr"`
  Link string `xml:"link,attr"`
  ActType string `xml:"actType,attr"`
}
func (p *Plan) start() (time.Time, string, string, string, Plan) {
  a := p.Activities[0]
  end_time, _ := time.Parse("15:04", a.EndTime)
  next_destination := p.Activities[1].Link
  return end_time, a.Link, next_destination, a.Type, Plan{p.Activities[1:]}
}
func (plan *Plan) simulate(person *Person, c chan Event) {
  end_time, linkId, next_destination, actType, rest := plan.start()
  for len(rest.Activities) > 0 {
    c <- Event{Time: time2Dur(end_time).Seconds(), Type: "actEnd", Person: person.Id, Link: linkId, ActType: actType}
    linkId = next_destination
    c <- Event{Time: time2Dur(end_time).Seconds(), Type: "actStart", Person: person.Id, Link: linkId, ActType: actType}
    end_time, linkId, actType, rest = rest.arrive(end_time)
  }
  close(c)  
}
func (p *Plan) arrive(arrival_time time.Time) (time.Time, string, string, Plan) {
  if (len(p.Activities) > 1) {
    a := p.Activities[0]
    dur_time, _ := time.Parse("15:04", a.Duration)
    dur := time2Dur(dur_time)
    end_time := arrival_time.Add(dur)
    next_destination := p.Activities[1].Link
    return end_time, next_destination, a.Type, Plan{p.Activities[1:]}  
  } else {
    return time.Now(), "", "", Plan{}
  }
}
type Person struct {
  Id string `xml:"id,attr"`
  Plans []Plan `xml:"plan"`
}
type Population struct {
  Persons []Person `xml:"person"`
}

func network() Network {
  resp, _ := http.Get("http://ci.matsim.org:8080/job/MATSim_M2/ws/trunk/src/test/resources/test/scenarios/berlin/network.xml.gz")
  r, _ := gzip.NewReader(resp.Body)
  d := xml.NewDecoder(r)
  v := Network{}
  d.Decode(&v)
  r.Close()
  return v  
}

func population() Population {
  resp, _ := http.Get("http://ci.matsim.org:8080/job/MATSim_M2/ws/trunk/src/test/resources/test/scenarios/berlin/plans_hwh_1pct.xml.gz")
  r, _ := gzip.NewReader(resp.Body)
  d := xml.NewDecoder(r)
  v := Population{}
  d.Decode(&v)
  r.Close()
  return v  
}

func main() {
  n := network()
  fmt.Printf("hello, world\n")
  fmt.Printf("%d\n", len(n.Links))
  p := population()
  done := make(chan int)
  cc := make(chan (chan Event))
  go func() {
    merged := make(chan Event)
    pq := make(PriorityQueue, 0)
    go func() {
      for e := range merged {
        out, _ := xml.MarshalIndent(e, "\n", " ")
        os.Stdout.Write(out)      
      }
      done <- 0
    }()
    heap.Init(&pq)
    for c := range cc {
      e, ok := <- c
      if ok {
        heap.Push(&pq, &ChanWithHead{e, c})
      }
    }
    for pq.Len() > 0 {
      cwh := heap.Pop(&pq).(*ChanWithHead)
      merged <- cwh.head
      e, ok := <- cwh.tail
      if ok {
        heap.Push(&pq, &ChanWithHead{e, cwh.tail})
      }
    }
    close(merged)
  }()
  
  for _, person := range p.Persons {
    for _, plan := range person.Plans {
      c := make(chan Event)
      cc <- c
      go plan.simulate(&person, c) 
    }
  }
  close(cc)
  <- done 
}
