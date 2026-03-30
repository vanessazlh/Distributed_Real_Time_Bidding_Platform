export type AuctionStatus = 'OPEN' | 'CLOSED'

/** A physical item listed in a shop (from the shop service) */
export interface Item {
  item_id:      string
  shop_id:      string
  title:        string
  description:  string
  retail_value: number
  image_url?:   string
}

/** A shop registered by a user */
export interface Shop {
  shop_id:  string
  name:     string
  location: string
  owner_id: string
  logo_url?: string
}
export type BidStatus = 'WINNING' | 'OUTBID' | 'WON' | 'LOST'

export interface AuctionItem {
  title: string
  shop_name: string
  shop_id: string
}

export interface Auction {
  auction_id: string
  item: AuctionItem
  current_highest_bid: number  // cents
  retail_price: number         // cents
  end_time: number             // Unix ms
  status: AuctionStatus
  bid_count: number
  image_url: string
  shop_logo_url: string
  description: string
}

export interface User {
  user_id: string
  username: string
  email: string
  role: 'buyer' | 'seller'
}

/** A bid placed by the current user, shown on My Bids page */
export interface UserBid {
  bid_id: string
  auction_id: string
  item_title: string
  shop_name: string
  amount: number    // cents
  timestamp: number // Unix ms
  status: BidStatus
}

/** A single entry in the live bid history feed on the Auction Detail page */
export interface BidHistoryEntry {
  id: number
  user: string
  amount: number  // cents
  time: number    // Unix ms
}

export type PaymentStatus = 'pending' | 'processing' | 'completed' | 'failed' | 'refunded'

export interface Payment {
  payment_id:  string
  auction_id:  string
  user_id:     string
  item_id:     string
  shop_id:     string
  amount:      number  // cents
  status:      PaymentStatus
  fail_reason?: string
  created_at:  string
  updated_at:  string
}

/** WebSocket message received from the notification service */
export interface BidPlacedEvent {
  type: 'bid_placed'
  auction_id: string
  user_id: string
  amount: number
  previous_bidder: string
  item_title: string
  message: string
  bid_accepted_at: string  // ISO timestamp — for latency measurement
  delivered_at: string
  timestamp: string
}
