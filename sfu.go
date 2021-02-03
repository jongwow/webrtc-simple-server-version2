package main

import (
	"fmt"
	"sync"
)

type SFU struct {
	rooms map[string]*Room
	sync.RWMutex
}

func NewSFU() *SFU {
	fmt.Println("SFU Created")
	sfu := &SFU{
		rooms: make(map[string]*Room),
	}
	return sfu
}

func (s *SFU) GetRoom(name string) *Room {
	room := s.getRoom(name)
	if room == nil {
		room = s.newRoom(name)
		fmt.Printf("Room[%s] creating\n", name)
	} else {
		fmt.Printf("Room[%s] exists\n", name)
	}
	return room
}

func (s *SFU) newRoom(name string) *Room {
	room := NewRoom(name)

	defer func() {
		if room.isEmpty() {
			s.Lock()
			delete(s.rooms, name)
			s.Unlock()
		}
	}()

	s.Lock()
	s.rooms[name] = room
	s.Unlock()

	return room
}

func (s *SFU) getRoom(name string) *Room {
	s.RLock()
	defer s.RUnlock()
	return s.rooms[name]
}
