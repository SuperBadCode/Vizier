package vizier

import (
	log "github.com/sirupsen/logrus"
)

type Manager struct {
	_      struct{}
	name   string
	states map[string]IState
	edges  map[string](chan interface{})
	Pool   *Pool
}

func (m *Manager) CreateState(name string, state IState) vizierErr {
	if _, ok := m.states[name]; ok {
		return NewVizierError(ErrSourceManager, ErrMsgStateAlreadyExists, name)
	}
	log.WithFields(log.Fields{
		"source": "manager",
		"name":   name,
	}).Info("created state")
	m.states[name] = state
	return nil
}

func (m *Manager) DeleteState(name string) vizierErr {
	if _, ok := m.states[name]; ok {
		log.WithFields(log.Fields{
			"source": "manager",
			"name":   name,
		}).Info("deleted state")
		delete(m.states, name)
		return nil
	}
	return NewVizierError(ErrSourceManager, ErrMsgStateDoesNotExist, name)
}

func (m *Manager) GetState(name string) (IState, vizierErr) {
	if s, ok := m.states[name]; ok {
		log.WithFields(log.Fields{
			"source": "manager",
			"name":   name,
		}).Info("get state")
		return s, nil
	}
	return nil, NewVizierError(ErrSourceManager, ErrMsgStateDoesNotExist, name)
}

func (m *Manager) CreateEdge(name string) vizierErr {
	if _, ok := m.edges[name]; ok {
		return NewVizierError(ErrSourceManager, ErrMsgEdgeAlreadyExists, name)
	}
	log.WithFields(log.Fields{
		"source": "manager",
		"name":   name,
	}).Info("created edge")
	m.edges[name] = make(chan interface{})
	return nil
}

func (m *Manager) DeleteEdge(name string) vizierErr {
	if _, ok := m.edges[name]; ok {
		log.WithFields(log.Fields{
			"source": "manager",
			"name":   name,
		}).Info("delete edge")
		delete(m.edges, name)
		return nil
	}
	return NewVizierError(ErrSourceManager, ErrMsgEdgeDoesNotExist, name)
}

func (m *Manager) GetEdge(name string) (chan interface{}, vizierErr) {
	if e, ok := m.edges[name]; ok {
		log.WithFields(log.Fields{
			"source": "manager",
			"name":   name,
		}).Info("get edge")
		return e, nil
	}
	return nil, NewVizierError(ErrSourceManager, ErrMsgEdgeDoesNotExist, name)
}

func NewManager(name string, poolSize int) (*Manager, error) {
	states := make(map[string]IState)
	pool, err := NewPool(name, poolSize, states)
	if err != nil {
		return nil, err
	}
	log.WithFields(log.Fields{
		"source": "manager",
		"name":   name,
	}).Info("created manager")
	return &Manager{
		name:   name,
		states: states,
		edges:  make(map[string](chan interface{})),
		Pool:   pool,
	}, nil
}