package app

import (
	"github.com/sori883/ai-dd/src/internal/assignment"
	"github.com/sori883/ai-dd/src/internal/cli"
	"github.com/sori883/ai-dd/src/internal/flow"
	"strconv"
)

func (s Service) executeAssignment(r cli.CommandRequest) ([]byte, error) {
	registry := assignment.Store{Root: s.Root}
	switch r.Action {
	case "init":
		var req assignment.InitRequest
		if err := s.decodeDraft(r.File, &req); err != nil {
			return nil, err
		}
		v, err := registry.Init(req)
		if err != nil {
			return nil, err
		}
		return encode(v)
	case "reset":
		var req assignment.ResetRequest
		if err := s.decodeDraft(r.File, &req); err != nil {
			return nil, err
		}
		v, err := registry.Reset(req)
		if err != nil {
			return nil, err
		}
		return encode(v)
	case "list", "show", "check":
		reg, err := registry.Read()
		if err != nil {
			return nil, err
		}
		if r.Action == "list" {
			return encode(reg.Reservations)
		}
		for _, v := range reg.Reservations {
			if v.ID == r.Target {
				if r.Action == "check" {
					if err := (flow.Store{Root: s.Root, Space: v.Space}).CheckAssignment(v); err != nil {
						return nil, err
					}
				}
				return encode(v)
			}
		}
		return nil, invalid("assignment not found")
	case "reserve", "release":
		expect, err := strconv.ParseUint(r.Expect, 10, 64)
		if err != nil || expect == 0 {
			return nil, invalid("positive expected revision required")
		}
		if r.Action == "reserve" {
			var req flow.AssignmentRequest
			if err := s.decodeDraft(r.File, &req); err != nil {
				return nil, err
			}
			v, err := (flow.Store{Root: s.Root, Space: r.Space}).ReserveAssignment(r.Target, expect, r.Session, req)
			if err != nil {
				return nil, err
			}
			return encode(v)
		}
		var req assignment.ReleaseRequest
		if err := s.decodeDraft(r.File, &req); err != nil {
			return nil, err
		}
		v, err := registry.Release(r.Target, r.Session, expect, req)
		if err != nil {
			return nil, err
		}
		return encode(v)
	}
	return nil, invalid("unknown assignment action")
}
