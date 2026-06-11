package source

import (
	"fmt"
	"sync"

	"github.com/wu/agentsstore/internal/config"
	"github.com/wu/agentsstore/internal/store"
)

type State struct {
	Name        string `json:"name"`
	Enabled     bool   `json:"enabled"`
	LastRefresh string `json:"last_refresh,omitempty"`
	LastError   string `json:"last_error,omitempty"`
}

type Manager struct {
	cfgPath string
	store   *store.Store
	mu      *sync.Mutex
	states  map[string]State
}

func NewManager(cfgPath string, st *store.Store, mu *sync.Mutex) *Manager {
	return &Manager{cfgPath: cfgPath, store: st, mu: mu, states: map[string]State{}}
}

func (m *Manager) Add(s config.Source) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cfg, err := m.loadLocked()
	if err != nil {
		return err
	}
	for _, existing := range cfg.Sources {
		if existing.Name == s.Name {
			return fmt.Errorf("source %q already exists", s.Name)
		}
	}
	if err := s.Validate(); err != nil {
		return err
	}
	cfg.Sources = append(cfg.Sources, s)
	if err := cfg.Save(m.cfgPath); err != nil {
		return err
	}
	m.states[s.Name] = State{Name: s.Name, Enabled: s.Enabled}
	return nil
}

func (m *Manager) Update(name string, s config.Source) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cfg, err := m.loadLocked()
	if err != nil {
		return err
	}
	found := false
	for i, existing := range cfg.Sources {
		if existing.Name == name {
			if s.Name != name {
				return fmt.Errorf("cannot rename source %q (use delete + add)", name)
			}
			cfg.Sources[i] = s
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("source %q not found", name)
	}
	if err := s.Validate(); err != nil {
		return err
	}
	if err := cfg.Save(m.cfgPath); err != nil {
		return err
	}
	m.states[name] = State{Name: name, Enabled: s.Enabled}
	return nil
}

func (m *Manager) Delete(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cfg, err := m.loadLocked()
	if err != nil {
		return err
	}
	out := cfg.Sources[:0]
	found := false
	for _, existing := range cfg.Sources {
		if existing.Name == name {
			found = true
			continue
		}
		out = append(out, existing)
	}
	if !found {
		return fmt.Errorf("source %q not found", name)
	}
	cfg.Sources = out
	if err := cfg.Save(m.cfgPath); err != nil {
		return err
	}
	delete(m.states, name)
	return m.store.RemoveSource(name)
}

func (m *Manager) List() ([]State, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cfg, err := m.loadLocked()
	if err != nil {
		return nil, err
	}
	out := make([]State, 0, len(cfg.Sources))
	for _, s := range cfg.Sources {
		st := m.states[s.Name]
		st.Name = s.Name
		st.Enabled = s.Enabled
		out = append(out, st)
	}
	return out, nil
}

func (m *Manager) Get(name string) (config.Source, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cfg, err := m.loadLocked()
	if err != nil {
		return config.Source{}, err
	}
	for _, s := range cfg.Sources {
		if s.Name == name {
			return s, nil
		}
	}
	return config.Source{}, fmt.Errorf("source %q not found", name)
}

func (m *Manager) Snapshot() (*config.Config, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.loadLocked()
}

func (m *Manager) SetState(name string, mut func(*State)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	st := m.states[name]
	mut(&st)
	m.states[name] = st
}

func (m *Manager) GetState(name string) State {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.states[name]
}

func (m *Manager) loadLocked() (*config.Config, error) {
	return loadIfExists(m.cfgPath)
}
