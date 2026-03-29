package notification

// Polling baseline for Experiment 3 — no code required in this service.
//
// The polling endpoint GET /auctions/{auction_id} is owned by the Auction Service
// (Person 1). It returns the current auction state including the highest bid.
// Clients may poll this endpoint at a fixed interval (e.g. every 2 seconds) to
// observe bid updates without maintaining a persistent connection.
//
// Person 4 uses this pull-model endpoint as the baseline in Experiment 3 and
// compares its resource usage and notification latency against the WebSocket
// push model served by this notification service.
//
// No persistent connection is needed from this service for polling — Person 1's
// endpoint handles it end-to-end.
