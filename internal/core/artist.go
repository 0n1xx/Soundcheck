package core

// Artist is a musician a user follows.
type Artist struct {
	Name           string // spelled the way Ticketmaster spells it
	TicketmasterID string // empty for artists added before validation existed
}
