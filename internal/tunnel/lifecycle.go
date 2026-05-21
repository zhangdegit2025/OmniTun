package tunnel

import "fmt"

type TunnelStatus string

const (
	StatusStopped  TunnelStatus = "stopped"
	StatusStarting TunnelStatus = "starting"
	StatusActive   TunnelStatus = "active"
	StatusError    TunnelStatus = "error"
)

var ValidTransitions = map[TunnelStatus][]TunnelStatus{
	StatusStopped:  {StatusStarting, StatusError},
	StatusStarting: {StatusActive, StatusError},
	StatusActive:   {StatusStopped, StatusError},
	StatusError:    {StatusStopped, StatusStarting},
}

func (s TunnelStatus) String() string {
	return string(s)
}

func ValidateTransition(from, to TunnelStatus) error {
	validTargets, ok := ValidTransitions[from]
	if !ok {
		return fmt.Errorf("unknown tunnel status: %s", from)
	}
	for _, t := range validTargets {
		if t == to {
			return nil
		}
	}
	return fmt.Errorf("invalid status transition: %s -> %s", from, to)
}
