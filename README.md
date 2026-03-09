A platform where local stores post surplus or end-of-day items as rapid 5-minute auctions instead of throwing them away. Imagine a bakery posts 3 mystery pastry boxes at 5pm, and users have 5 minutes to outbid each other before the auction closes.
The core challenge is real-time bidding under extreme concurrency. When an auction is about to close, dozens of users submit bids in the final seconds (the "sniping" problem). Every bid needs to:
Validate that the auction is still open
Check that the new bid is higher than the current highest
Update the highest bid atomically
Notify other bidders they've been outbid
All of this has to happen consistently under concurrent load — if two users bid at the exact same millisecond, only one should win, and nobody should see stale data. This is a concurrency and consistency problem at its core.
On top of that, auctions are bursty by nature. A popular store posts 10 auctions at 5pm, and suddenly thousands of users flood the system simultaneously. Then at 5:05pm when auctions close, there's a second spike as the system processes winners, sends notifications, and handles payments. The system needs to scale horizontally to absorb these spikes and scale back down when idle.
Tech stack
Go for the auction service (goroutines + channels are a natural fit for concurrent bid processing)
ECS Fargate with ALB and auto-scaling (same infrastructure from our assignments)
Redis or DynamoDB for auction state (need fast reads/writes with consistency)
WebSockets or SSE for real-time bid updates to connected clients
Locust for load testing
Three experiments to evaluate scalability
Experiment 1: Bid contention under load Simulate 500 concurrent users bidding on the same auction in the final 10 seconds. Compare different concurrency strategies — optimistic locking with retries vs. pessimistic locking vs. a serialized bid queue. Measure: successful bid rate, rejected bid rate, average bid latency, and whether any consistency violations occur (e.g., a lower bid winning over a higher one).
Experiment 2: Horizontal scaling under auction spikes Simulate a "rush hour" scenario: 50 auctions go live simultaneously, each attracting 100 bidders. Start with 2 ECS tasks and auto-scaling configured. Measure: how quickly auto-scaling responds to the spike, latency during the scale-up window, throughput before vs. after new tasks join, and whether any bids are lost during the transition.
Experiment 3: Real-time notification fan-out When a new highest bid is placed, all other bidders watching that auction need to be notified. Simulate 1000 connected clients watching a single popular auction with rapid bidding. Measure: notification delivery latency (time from bid accepted to all clients notified), system resource usage as connected clients scale, and compare push (WebSocket) vs. pull (polling) approaches.
