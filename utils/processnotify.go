package utils

import (
	"strings"
	"time"
)

const (
	PROCESS_CREATE = iota
	PROCESS_DELETE
	PROCESS_MODIFY
	PROCESS_ERROR
)

type ProcessEvent struct {
	Type        int
	Error       error
	TimeStamp   uint64
	ProcessId   uint32
	Name        string
	CommandLine string
}

// ProcessNotify polls the Toolhelp process snapshot every second and reports
// set differences for the watched exe name. New pids → PROCESS_CREATE,
// vanished pids → PROCESS_DELETE, snapshot failures → PROCESS_ERROR
// (PROCESS_MODIFY is kept for API compatibility but never produced).
type ProcessNotify struct {
	name string
	ch   chan<- *ProcessEvent
	stop chan struct{}
}

func NewProcessNotify(name string, ch chan<- *ProcessEvent) (*ProcessNotify, error) {
	return &ProcessNotify{
		name: name,
		ch:   ch,
		stop: make(chan struct{}),
	}, nil
}

func (s *ProcessNotify) Start() {
	go s.watch()
}

func (s *ProcessNotify) Stop() {
	select {
	case <-s.stop:
	default:
		close(s.stop)
	}
}

func (s *ProcessNotify) watch() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	prev, err := snapshotProcesses()
	if err != nil {
		s.ch <- &ProcessEvent{Type: PROCESS_ERROR, Error: err}
		prev = make(map[uint32]string)
	}
	for {
		select {
		case <-s.stop:
			return
		case <-ticker.C:
			cur, err := snapshotProcesses()
			if err != nil {
				s.ch <- &ProcessEvent{Type: PROCESS_ERROR, Error: err}
				continue
			}
			now := uint64(time.Now().UnixNano())
			for pid, name := range cur {
				if _, ok := prev[pid]; ok {
					continue
				}
				if !strings.EqualFold(name, s.name) {
					continue
				}
				s.ch <- &ProcessEvent{
					Type:        PROCESS_CREATE,
					TimeStamp:   now,
					ProcessId:   pid,
					Name:        name,
					CommandLine: processCommandLine(pid),
				}
			}
			for pid, name := range prev {
				if _, ok := cur[pid]; ok {
					continue
				}
				if !strings.EqualFold(name, s.name) {
					continue
				}
				s.ch <- &ProcessEvent{
					Type:      PROCESS_DELETE,
					TimeStamp: now,
					ProcessId: pid,
					Name:      name,
				}
			}
			prev = cur
		}
	}
}
