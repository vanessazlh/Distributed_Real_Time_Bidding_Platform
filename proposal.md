# Real-Time Surplus Auction Platform

A platform where local stores post surplus or end-of-day items as rapid **5-minute auctions** instead of throwing them away.

For example, a bakery might post **3 mystery pastry boxes at 5pm**, and users have **5 minutes to bid** before the auction closes.

---

# Core Challenge: Real-Time Bidding Under Extreme Concurrency

When an auction is about to close, dozens of users may submit bids within the final seconds (the **"sniping" problem**).

Each bid must:

- Validate that the auction is still open
- Check that the new bid is higher than the current highest bid
- Update the highest bid **atomically**
- Notify other bidders that they have been outbid

All of these operations must happen **consistently under concurrent load**.

If two users submit bids at the exact same millisecond:

- Only **one bid should win**
- No user should observe **stale data**

At its core, this is a **concurrency and consistency problem**.

---

# Burst Traffic Characteristics

Auctions are **bursty by nature**.

Example scenario:

- A popular store posts **10 auctions at 5pm**
- Thousands of users flood the system simultaneously

Then at **5:05pm**, another spike occurs when:

- Auctions close
- Winners are processed
- Notifications are sent
- Payments are handled

The system must:

- **Scale horizontally** to absorb sudden spikes
- **Scale down** when traffic drops

---

# Tech Stack

- **Go**  
  Used for the auction service. Goroutines and channels naturally support concurrent bid processing.

- **ECS Fargate** with **Application Load Balancer (ALB)** and **auto-scaling**  
  Infrastructure based on the same setup used in course assignments.

- **Redis or DynamoDB**  
  Used to store auction state with fast reads/writes and strong consistency guarantees.

- **WebSockets or Server-Sent Events (SSE)**  
  For real-time bid updates to connected clients.

- **Locust**  
  For load testing and performance evaluation.

---

# Scalability Experiments

## Experiment 1: Bid Contention Under Load

Simulate **500 concurrent users** bidding on the same auction during the **final 10 seconds**.

Compare different concurrency control strategies:

- Optimistic locking with retries
- Pessimistic locking
- Serialized bid queue

### Metrics

- Successful bid rate
- Rejected bid rate
- Average bid latency
- Consistency violations  
  (e.g., a lower bid winning over a higher bid)

---

## Experiment 2: Horizontal Scaling During Auction Spikes

Simulate a **rush-hour scenario**:

- **50 auctions** go live simultaneously
- Each attracts **100 bidders**

System starts with **2 ECS tasks** with auto-scaling enabled.

### Metrics

- Auto-scaling response time to the spike
- Latency during the scale-up window
- Throughput **before vs. after** new tasks join
- Whether any bids are lost during scaling transitions

---

## Experiment 3: Real-Time Notification Fan-Out

Whenever a **new highest bid** is placed, all other bidders watching the auction must be notified.

Simulate:

- **1000 connected clients**
- Watching a **single popular auction**
- With rapid bid updates

### Metrics

- Notification delivery latency  
  (time from bid acceptance to all clients being notified)

- System resource usage as the number of connected clients scales

- Performance comparison:
  - **Push model** (WebSockets)
  - **Pull model** (polling)
