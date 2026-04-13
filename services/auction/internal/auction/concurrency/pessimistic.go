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
// Supports both single-winner (quantity=1) and multi-winner (quantity>1) auctions.
//
// For quantity=1: current_highest = highest bid, new bids must exceed it.
// For quantity>1: current_highest = floor price (lowest winning bid, or start_bid
// while winner slots remain open). A new bid must exceed the floor to win a slot.
// When all slots are full, the lowest winner is evicted if a higher bid arrives.
//
// KEYS[1] = auction:{auctionID}
// KEYS[2] = auction:{auctionID}:bids  (sorted set for winner tracking)
// ARGV[1] = bid amount (int64)
// ARGV[2] = bidder ID (string)
//
// Returns (as string array):
//
//	{newVersion, '', floor}   on success (no eviction)
//	{newVersion, evictedID, floor}  on success (someone evicted, quantity>1 only)
//	{'-1', '', '0'}           auction not found
//	{'-2', '', '0'}           auction is not open
//	{'-3', '', threshold}     bid too low (threshold = price to beat)
//	{'-4', '', maxPrice}      bid exceeds max price
var placeBidLua = redis.NewScript(`
local key = KEYS[1]
local bidsKey = KEYS[2]
local amount = tonumber(ARGV[1])
local bidder = ARGV[2]

local status = redis.call('HGET', key, 'status')
if status == false then
	return {'-1', '', '0'}
end
if status ~= 'OPEN' then
	return {'-2', '', '0'}
end

local maxPrice = tonumber(redis.call('HGET', key, 'max_price')) or 0
if maxPrice > 0 and amount > maxPrice then
	return {'-4', '', tostring(maxPrice)}
end

local current = tonumber(redis.call('HGET', key, 'current_highest')) or 0
local quantity = tonumber(redis.call('HGET', key, 'quantity')) or 1
local cardCount = redis.call('ZCARD', bidsKey)

if quantity <= 1 then
	-- Single-winner: standard logic
	if amount <= current then
		return {'-3', '', tostring(current)}
	end
else
	-- Multi-winner slot management
	local existingScore = redis.call('ZSCORE', bidsKey, bidder)
	if existingScore ~= false then
		-- Bidder already holds a slot: must improve on their own bid
		if amount <= tonumber(existingScore) then
			return {'-3', '', tostring(existingScore)}
		end
	elseif cardCount >= quantity then
		-- All slots full: must beat the floor (lowest current winner)
		local floor = redis.call('ZRANGE', bidsKey, 0, 0, 'WITHSCORES')
		if #floor >= 2 then
			local floorAmount = tonumber(floor[2])
			if amount <= floorAmount then
				return {'-3', '', tostring(floorAmount)}
			end
		end
	else
		-- Open slots available: must exceed start_bid (= initial current_highest)
		if amount <= current then
			return {'-3', '', tostring(current)}
		end
	end
end

local version = tonumber(redis.call('HGET', key, 'version')) or 0
local newVersion = version + 1

-- Place the bid in the sorted set
redis.call('ZADD', bidsKey, amount, bidder)

-- For multi-winner: trim excess and identify evicted bidder
local evicted = ''
if quantity > 1 then
	local newCard = redis.call('ZCARD', bidsKey)
	if newCard > quantity then
		local lowest = redis.call('ZRANGE', bidsKey, 0, 0)
		if #lowest >= 1 then
			evicted = lowest[1]
		end
		redis.call('ZREMRANGEBYRANK', bidsKey, 0, 0)
	end
end

-- Calculate the new floor price
local newFloor = amount  -- default for quantity=1: highest bid
if quantity > 1 then
	local finalCard = redis.call('ZCARD', bidsKey)
	if finalCard >= quantity then
		-- Floor = lowest winner's bid
		local floorEntry = redis.call('ZRANGE', bidsKey, 0, 0, 'WITHSCORES')
		if #floorEntry >= 2 then
			newFloor = tonumber(floorEntry[2])
		end
	else
		-- Still open slots: floor stays at start_bid (= current_highest)
		newFloor = current
	end
end

redis.call('HSET', key,
	'current_highest', newFloor,
	'highest_bidder',  bidder,
	'version',         newVersion)
redis.call('HINCRBY', key, 'bid_count', 1)

return {tostring(newVersion), evicted, tostring(newFloor)}
`)

// BidPlacement holds the result of a successful bid placement.
type BidPlacement struct {
	NewVersion    int64
	EvictedBidder string // non-empty if a bidder was evicted from winner slots (quantity>1)
	NewFloor      int64  // updated floor price (= current_highest after the bid)
}

// Pessimistic implements atomic bid placement using a Lua script.
type Pessimistic struct {
	rdb *redis.Client
}

// NewPessimistic creates a new Pessimistic controller.
func NewPessimistic(rdb *redis.Client) *Pessimistic {
	return &Pessimistic{rdb: rdb}
}

// TryPlaceBid atomically validates and places a bid via a Lua script.
// Returns a BidPlacement with version, eviction info, and updated floor.
func (p *Pessimistic) TryPlaceBid(ctx context.Context, auctionID string, amount int64, bidderID string) (*BidPlacement, error) {
	key := "auction:" + auctionID
	bidsKey := key + ":bids"

	res, err := placeBidLua.Run(ctx, p.rdb, []string{key, bidsKey},
		amount, bidderID,
	).StringSlice()
	if err != nil {
		return nil, fmt.Errorf("place bid lua: %w", err)
	}

	code, _ := strconv.ParseInt(res[0], 10, 64)
	threshold := res[2]

	switch code {
	case -1:
		return nil, fmt.Errorf("auction not found")
	case -2:
		return nil, fmt.Errorf("auction is not open")
	case -3:
		return nil, fmt.Errorf("bid amount %d must be higher than current highest %s",
			amount, threshold)
	case -4:
		return nil, fmt.Errorf("bid amount %d exceeds max price %s",
			amount, threshold)
	default:
		floor, _ := strconv.ParseInt(threshold, 10, 64)
		return &BidPlacement{
			NewVersion:    code,
			EvictedBidder: res[1],
			NewFloor:      floor,
		}, nil
	}
}
