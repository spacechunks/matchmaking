package matchmaking

import "time"

type Ticket struct {
	ID          string
	PlayerCount int
	Active      bool
	FlavorID    string
	Assignment  *Assignment
	MatchID     *string
}

func (t Ticket) GetID() string {
	return t.ID
}

type Assignment struct {
	InstanceID string
}

type Match struct {
	ID        string
	Tickets   TicketList
	Full      bool
	FlavorID  string
	CreatedAt time.Time
}

func (m Match) GetID() string {
	return m.ID
}

func (m Match) PlayerCount() int {
	return m.Tickets.PlayerCount()
}

type TicketList []Ticket

func (l TicketList) PlayerCount() int {
	sum := 0
	for _, ticket := range l {
		sum += ticket.PlayerCount
	}
	return sum
}
