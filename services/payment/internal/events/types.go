package events

import shared "rtb/shared/events"

// Type aliases — canonical definitions live in rtb/shared/events.
// All services should import from there directly; these aliases exist
// so payment-internal code keeps working without double imports.
type (
	AuctionClosedEvent    = shared.AuctionClosedEvent
	PaymentProcessedEvent = shared.PaymentProcessedEvent
	PaymentFailedEvent    = shared.PaymentFailedEvent
	RefundProcessedEvent  = shared.RefundProcessedEvent
)
