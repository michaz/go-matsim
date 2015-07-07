package main

import (
  "fmt"
  "time"
  "encoding/xml"
  "net/http"
  "container/heap"
)
func time2Dur(dur_time time.Time) time.Duration {
  zero, _ := time.Parse("15:04", "00:00")
  dur := dur_time.Sub(zero)
  return dur
}
type ChanWithHead struct {
  head ActivityEnd
  tail chan ActivityEnd
}
func (cwh ChanWithHead) priority() float64 {
  return cwh.head.time
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
type ActivityEnd struct {
  XMLName xml.Name `xml:"event"`
  time float64 `xml:"time,attr"`
  Type string `xml:"type,attr"`
  Person string `xml:"person,attr"`
  Link string `xml:"link,attr"`
  ActType string `xml:"actType,attr"`
}
func (p *Plan) start() (time.Time, string, Plan) {
  a := p.Activities[0]
  end_time, _ := time.Parse("15:04", a.EndTime)
  return end_time, p.Activities[1].Link, Plan{p.Activities[1:]}
}
func (plan *Plan) simulate(person *Person, c chan ActivityEnd) {
  end_time, _, rest := plan.start()
  for len(rest.Activities) > 0 {
    c <- ActivityEnd{time: time2Dur(end_time).Seconds(), Type: "actEnd", Person: person.Id, Link: "start", ActType: "actType"}
    end_time, _, rest = rest.arrive(end_time)
  }
  close(c)  
}
func (p *Plan) arrive(arrival_time time.Time) (time.Time, string, Plan) {
  if (len(p.Activities) > 1) {
    a := p.Activities[0]
    dur_time, _ := time.Parse("15:04", a.Duration)
    dur := time2Dur(dur_time)
    end_time := arrival_time.Add(dur)
    next_destination := p.Activities[1].Link
    return end_time, next_destination, Plan{p.Activities[1:]}  
  } else {
    return time.Now(), "", Plan{}
  }
}
type Person struct {
  Id string `xml:"id"`
  Plans []Plan `xml:"plan"`
}
type Population struct {
  Persons []Person `xml:"person"`
}

func network() Network {
  resp, _ := http.Get("http://ci.matsim.org:8080/job/MATSim_M2/ws/trunk/examples/equil/network.xml")
  defer resp.Body.Close()
  d := xml.NewDecoder(resp.Body)
  v := Network{}
  d.Decode(&v)
  return v  
}

func population() Population {
  resp, _ := http.Get("http://ci.matsim.org:8080/job/MATSim_M2/ws/trunk/examples/equil/plans100.xml")
  defer resp.Body.Close()
  d := xml.NewDecoder(resp.Body)
  v := Population{}
  d.Decode(&v)
  return v  
}

func main() {
  n := network()
  fmt.Printf("hello, world\n")
  fmt.Printf("%d\n", len(n.Links))
  for _, link := range n.Links {
    fmt.Printf("%v\n", link)
  }
  p := population()
  done := make(chan int)
  cc := make(chan (chan ActivityEnd))
  go func() {
    merged := make(chan ActivityEnd)
    pq := make(PriorityQueue, 0)
    go func() {
      for e := range merged {
        fmt.Printf("%v\n", e)
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
      c := make(chan ActivityEnd)
      cc <- c
      go plan.simulate(&person, c) 
    }
  }
  close(cc)
  <- done 
}
