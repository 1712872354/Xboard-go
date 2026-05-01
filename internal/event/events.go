package event

// ============================================================================
// Event type constants
// ============================================================================

const (
	// Order events
	EventOrderCreated   = "order.created"
	EventOrderPaid      = "order.paid"
	EventOrderOpened    = "order.opened"
	EventOrderCancelled = "order.cancelled"

	// User events
	EventUserRegistered = "user.registered"

	// Traffic events
	EventTrafficReset = "traffic.reset"

	// Ticket events
	EventTicketCreated = "ticket.created"
	EventTicketReplied = "ticket.replied"
	EventTicketClosed  = "ticket.closed"
)

// ============================================================================
// Event payloads
// ============================================================================

// OrderCreatedEvent is fired when a new order is created
type OrderCreatedEvent struct {
	OrderID   uint
	TradeNo   string
	UserID    uint
	PlanID    uint
	PlanName  string
	Amount    float64
	OrderType int
}

// OrderPaidEvent is fired when an order payment succeeds
type OrderPaidEvent struct {
	OrderID   uint
	TradeNo   string
	UserID    uint
	Amount    float64
	PaidAt    int64 // unix timestamp
}

// OrderOpenedEvent is fired when an order is opened (subscription activated)
type OrderOpenedEvent struct {
	OrderID   uint
	TradeNo   string
	UserID    uint
	PlanID    uint
	ExpiredAt int64 // unix timestamp
}

// UserRegisteredEvent is fired when a new user registers
type UserRegisteredEvent struct {
	UserID       uint
	Email        string
	InviteUserID uint
}

// TrafficResetEvent is fired when a user's traffic is reset
type TrafficResetEvent struct {
	UserID       uint
	TriggerSource string // "cron", "manual", "admin"
}

// TicketCreatedEvent is fired when a support ticket is created
type TicketCreatedEvent struct {
	TicketID uint
	UserID   uint
	Subject  string
	Level    int
}