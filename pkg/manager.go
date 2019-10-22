package internal

import (
	"fmt"
	"math"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

type Manager struct {
	_          struct{}
	name       string
	states     map[string]IState
	size       int
	run        bool
	stopWorker chan bool
}

func (m *Manager) Node(name string, f func(interface{}) map[string]interface{}) *Manager {
	if _, ok := m.states[name]; ok {
		detail := fmt.Sprintf("failed to create state %s.", name)
		panic(newVizierError(ErrSourceManager, ErrMsgStateAlreadyExists, detail))
	}

	m.log(log.Fields{"name": name}).Info("created node")
	m.states[name] = newState(name, f)

	return m
}

func (m *Manager) Output(from, name string) chan Packet {
	if _, ok := m.states[from]; !ok {
		detail := fmt.Sprintf("failed to create output edge %s. source state %s", name, from)
		panic(newVizierError(ErrSourceManager, ErrMsgStateDoesNotExist, detail))
	}

	if m.states[from].HasEdge(name) {
		detail := fmt.Sprintf("failed to create output edge %s. source state %s", name, from)
		panic(newVizierError(ErrSourceManager, ErrMsgEdgeAlreadyExists, detail))
	}

	m.log(log.Fields{
		"node": from,
		"edge": name,
	}).Info("created output edge")

	output := make(chan Packet)
	err := m.states[from].AttachEdge(name, output, true)
	if err != nil {
		panic(err)
	}

	return output
}

func (m *Manager) Edge(from, to, name string) *Manager {
	edgeName := fmt.Sprintf("%s_to_%s_%s", from, to, name)

	if _, ok := m.states[to]; !ok {
		detail := fmt.Sprintf("failed to create edge %s. destination state %s", name, to)
		panic(newVizierError(ErrSourceManager, ErrMsgStateDoesNotExist, detail))
	}

	if _, ok := m.states[from]; !ok {
		detail := fmt.Sprintf("failed to create edge %s. source state %s", name, from)
		panic(newVizierError(ErrSourceManager, ErrMsgStateDoesNotExist, detail))
	}

	if m.states[from].HasEdge(edgeName) {
		detail := fmt.Sprintf("failed to create edge %s. source state %s", name, from)
		panic(newVizierError(ErrSourceManager, ErrMsgEdgeAlreadyExists, detail))
	}

	m.log(log.Fields{
		"from": from,
		"to":   to,
		"edge": name,
	}).Info("created edge")

	err := m.states[from].AttachEdge(edgeName, m.states[to].GetPipe(), false)
	if err != nil {
		panic(err)
	}

	return m
}

func (m *Manager) BatchInvoke(name string, batch []interface{}) (*sync.WaitGroup, vizierErr) {
	var wg sync.WaitGroup

	if _, ok := m.states[name]; !ok {
		detail := fmt.Sprintf("failed to invoke state %s.", name)
		return nil, newVizierError(ErrSourceManager, ErrMsgStateDoesNotExist, detail)
	}

	m.log(log.Fields{
		"node": name,
		"size": len(batch),
	}).Info("batch invoke")

	for _, payload := range batch {
		m.states[name].Invoke(payload, &wg)
	}

	return &wg, nil
}

func (m *Manager) Invoke(name string, payload interface{}) (*sync.WaitGroup, vizierErr) {
	var wg sync.WaitGroup

	if _, ok := m.states[name]; !ok {
		detail := fmt.Sprintf("failed to invoke state %s.", name)
		return nil, newVizierError(ErrSourceManager, ErrMsgStateDoesNotExist, detail)
	}

	m.log(log.Fields{"node": name}).Info("invoke")
	m.states[name].Invoke(payload, &wg)

	return &wg, nil
}

func (m *Manager) Start() vizierErr {
	if len(m.states) < 1 {
		return newVizierError(ErrSourceManager, ErrMsgPoolEmptyStates, m.name)
	}

	if m.run {
		return newVizierError(ErrSourceManager, ErrMsgPoolIsRunning, m.name)
	}

	m.log(log.Fields{}).Info("started")

	m.run = true
	for i := 0; i < m.size; i++ {
		m.spawnWorker()
	}

	return nil
}

func (m *Manager) Stop() vizierErr {
	if !m.run {
		return newVizierError(ErrSourceManager, ErrMsgPoolNotRunning, "")
	}
	m.log(log.Fields{}).Info("stopped")
	m.run = false
	return nil
}

func (m *Manager) GetSize() int {
	return m.size
}

func (m *Manager) SetSize(size int) vizierErr {
	if !m.run {
		return newVizierError(ErrSourceManager, ErrMsgPoolNotRunning, "")
	}

	if size <= 0 {
		detail := fmt.Sprintf("%s. invalid size %d", m.name, size)
		return newVizierError(ErrSourceManager, ErrMsgPoolSizeInvalid, detail)
	}

	m.log(log.Fields{
		"old_size": m.size,
		"new_size": size,
	}).Info("resize")

	delta := int(math.Abs(float64(m.size - size)))
	spawn := (size > m.size)
	for i := 0; i < delta; i++ {
		if spawn {
			m.spawnWorker()
			continue
		}
		m.stopWorker <- true
	}
	m.size = size

	return nil
}

func (m *Manager) GetResults(wg *sync.WaitGroup, size int, pipe chan Packet) []interface{} {
	results := make([]interface{}, size)
	isWaiting := true
	go func() {
		index := 0
		for isWaiting || index < size {
			packet := <-pipe
			results[index] = packet.Payload
			index++
		}
	}()
	wg.Wait()
	isWaiting = false
	return results
}

func (m *Manager) spawnWorker() {
	m.log(log.Fields{}).Info("worker spawned")

	go func() {
		defer func() {
			if err := recover(); err != nil {
				m.log(log.Fields{
					"err": err,
				}).Warn("worker panic")
				m.spawnWorker()
			}
		}()
		for m.run {
			select {
			case <-m.stopWorker:
				return
			default:
				for _, state := range m.states {
					state.Poll()
				}
			}
		}
	}()
}

func (m *Manager) log(fields log.Fields) *log.Entry {
	fields["source"] = "manager"
	fields["name"] = m.name
	fields["time"] = time.Now().UTC().String()
	return log.WithFields(fields)
}

func NewManager(name string, size int) (*Manager, error) {
	states := make(map[string]IState)

	manager := Manager{
		name:       name,
		states:     states,
		size:       size,
		stopWorker: make(chan bool, size),
	}

	manager.log(log.Fields{
		"size": size,
	}).Info("created manager")

	return &manager, nil
}
