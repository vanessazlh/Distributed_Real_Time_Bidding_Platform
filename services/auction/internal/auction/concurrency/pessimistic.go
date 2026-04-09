package concurrency

import (
	"context"
	"fmt"
	"strconv"

	"github.com/redis/go-redis/v9"
)

// placeBidLua atomically reads, validates, and updates an auction in a single
// Redis call. Because Lua scripts execute without interleaving, no external
// lock is needed.
//
// KEYS[1] = auction:{auctionID}
// KEYS[2] = auction:{auctionID}:bids  (sorted set for winner tracking)
// ARGV[1] = bid amount (int64)
// ARGV[2] = bidder ID (string)
//
// Returns:
//
//	newVersion (int)  on success
//	-1                auction not found
//	-2                auction is not open
//	-3                bid too low (current highest returned as second value)
//	-4                bid exceeds max price (max_price returned as second value)
var placeBidLua = redis.NewScript(`
local key = KEYS[1]
local amount = tonumber(ARGV[1])
local bidder = ARGV[2]

-- Read only the fields we need
local status = redis.call('HGET', key, 'status')
if status == false then
	return {-1, 0}
end
if status ~= 'OPEN' then
	return {-2, 0}
end

local current = tonumber(redis.call('HGET', key, 'current_highest')) or 0
if amount <= current then
	return {-3, current}
end

local maxPrice = tonumber(redis.call('HGET', key, 'max_price')) or 0
if maxPrice > 0 and amount > maxPrice then
	return {-4, maxPrice}
end

local version = tonumber(redis.call('HGET', key, 'version')) or 0
local newVersion = version + 1

redis.call('HSET', key,
	'current_highest', amount,
	'highest_bidder',  bidder,
	'version',         newVersion)
redis.call('HINCRBY', key, 'bid_count', 1)
redis.call('ZADD', KEYS[2], amount, bidder)

return {newVersion, 0}
`)

// Pessimistic implements atomic bid placement using a Lua script.
type Pessimistic struct {
	rdb *redis.Client
}

// NewPessimistic creates a new Pessimistic controller.
func NewPessimistic(rdb *redis.Client) *Pessimistic {
	return &Pessimistic{rdb: rdb}
}

// TryPlaceBid atomically validates and places a bid via a Lua script.
func (p *Pessimistic) TryPlaceBid(ctx context.Context, auctionID string, amount int64, bidderID string) (int64, error) {
	key := "auction:" + auctionID
	bidsKey := key + ":bids"

	res, err := placeBidLua.Run(ctx, p.rdb, []string{key, bidsKey},
		amount, bidderID,
	).Int64Slice()
	if err != nil {
		return 0, fmt.Errorf("place bid lua: %w", err)
	}

	code := res[0]
	switch code {
	case -1:
		return 0, fmt.Errorf("auction not found")
	case -2:
		return 0, fmt.Errorf("auction is not open")
	case -3:
		return 0, fmt.Errorf("bid amount %d must be higher than current highest %s",
			amount, strconv.FormatInt(res[1], 10))
	case -4:
		return 0, fmt.Errorf("bid amount %d exceeds max price %s",
			amount, strconv.FormatInt(res[1], 10))
	default:
		return code, nil // code == newVersion
	}
}
